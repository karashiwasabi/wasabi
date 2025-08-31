// C:\Dev\WASABI\medrec\handler.go

package medrec

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/dat"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

func DownloadHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg.EmednetUserID == "" || cfg.EmednetPassword == "" {
			http.Error(w, "e-mednetのIDまたはパスワードが設定されていません。設定画面を確認してください。", http.StatusBadRequest)
			return
		}

		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36`),
		)

		if edgePath := findEdgePath(); edgePath != "" {
			opts = append(opts, chromedp.ExecPath(edgePath))
		}

		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
		defer cancel()

		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
		defer cancel()

		downloadDir, err := filepath.Abs("./download/DAT")
		if err != nil {
			http.Error(w, "ダウンロードディレクトリのパス取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			http.Error(w, "ダウンロードディレクトリの作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var downloadedFilePath string
		done := make(chan string, 1)
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			if e, ok := ev.(*browser.EventDownloadProgress); ok {
				if e.State == browser.DownloadProgressStateCompleted {
					done <- e.GUID
				}
			}
		})

		var initialLatestTimestamp string
		err = chromedp.Run(ctx,
			browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
				WithDownloadPath(downloadDir).
				WithEventsEnabled(true),
			chromedp.Navigate(`https://www.e-mednet.jp/`),
			chromedp.WaitVisible(`input[name="userid"]`),
			chromedp.SendKeys(`input[name="userid"]`, cfg.EmednetUserID),
			chromedp.SendKeys(`input[name="userpsw"]`, cfg.EmednetPassword),
			chromedp.Click(`input[type="submit"][value="ログイン"]`),
			chromedp.WaitVisible(`//a[contains(@href, "busi_id=11")]`),
			chromedp.Click(`//a[contains(@href, "busi_id=11")]`),
			chromedp.WaitVisible(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.Click(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.WaitVisible(`input[value="未受信データ全件受信"]`),
			// クリック前に、現在の最新履歴のタイムスタンプを取得しておく
			chromedp.Text(`table.result-list-table tbody tr:first-child td.col-transceiving-date`, &initialLatestTimestamp),
			// ボタンをクリックしてデータ受信を実行
			chromedp.Click(`input[value="未受信データ全件受信"]`),
			// クリック後に意図的に待機
			chromedp.Sleep(3*time.Second),
		)

		if err != nil {
			log.Printf("ERROR: Chromedp task failed during navigation/clicks: %v", err)
			http.Error(w, "e-mednetの自動操作（画面遷移）に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var noDataFound bool
		timeout := time.After(90 * time.Second) // タイムアウトを90秒に延長
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

	Loop:
		for {
			select {
			case guid := <-done:
				downloadedFilePath = filepath.Join(downloadDir, guid)
				log.Printf("Download completed: %s", guid)
				break Loop
			case <-timeout:
				err = fmt.Errorf("operation timed out after 90 seconds")
				break Loop
			case <-ticker.C:
				checkCtx, cancelCheck := context.WithTimeout(ctx, 2*time.Second)
				var newLatestTimestamp, resultText string

				// 履歴テーブルの最新タイムスタンプを再取得
				if errCheck := chromedp.Run(checkCtx,
					chromedp.Text(`table.result-list-table tbody tr:first-child td.col-transceiving-date`, &newLatestTimestamp),
				); errCheck == nil {
					// タイムスタンプがクリック前と異なっていれば、ページが更新されたと判断
					if strings.TrimSpace(newLatestTimestamp) != strings.TrimSpace(initialLatestTimestamp) {
						// 更新された行の「受信結果」テキストを取得
						if errResult := chromedp.Run(checkCtx,
							chromedp.Text(`table.result-list-table tbody tr:first-child td.col-result`, &resultText),
						); errResult == nil && strings.TrimSpace(resultText) == "受信データなし" {
							noDataFound = true
							log.Println("No new delivery data message found in history table.")
							cancelCheck()
							break Loop
						}
					}
				}
				cancelCheck()
			}
		}

		if err != nil {
			log.Printf("ERROR: Chromedp task failed during wait: %v", err)
			errMsg := strings.ReplaceAll(err.Error(), "\n", " ")
			http.Error(w, "e-mednetの自動操作（待機処理）に失敗しました: "+errMsg, http.StatusInternalServerError)
			return
		}

		if noDataFound {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"message": "未受信の納品データはありませんでした。",
			})
			return
		}

		// ファイルハンドルが解放されるのを少し待つ (ファイル名変更失敗対策)
		time.Sleep(500 * time.Millisecond)

		if downloadedFilePath == "" {
			http.Error(w, "e-mednetからのダウンロードに失敗したか、予期せぬ状態になりました。", http.StatusInternalServerError)
			return
		}

		log.Printf("File temporarily downloaded to %s", downloadedFilePath)

		// ファイル名変換ロジック
		tempFile, err := os.Open(downloadedFilePath)
		if err != nil {
			http.Error(w, "ダウンロードしたファイルを開けませんでした: "+err.Error(), http.StatusInternalServerError)
			return
		}

		scanner := bufio.NewScanner(tempFile)
		var destDir string
		var newBaseName string
		if scanner.Scan() {
			firstLine := scanner.Text()
			if strings.HasPrefix(firstLine, "S") && len(firstLine) >= 39 {
				timestampStr := firstLine[27:39]
				yy, mm, dd, h, m, s := timestampStr[0:2], timestampStr[2:4], timestampStr[4:6], timestampStr[6:8], timestampStr[8:10], timestampStr[10:12]
				newBaseName = fmt.Sprintf("20%s%s%s_%s%s%s", yy, mm, dd, h, m, s)
			}
		}
		tempFile.Close() // スキャンが終わったらファイルを閉じる

		if newBaseName != "" {
			destDir = filepath.Join("download", "DAT")
		} else {
			destDir = filepath.Join("download", "DAT", "unorganized")
			newBaseName = time.Now().Format("20060102150405")
			log.Printf("Warning: Could not parse timestamp from downloaded file. Saving to %s", destDir)
		}

		if err := os.MkdirAll(destDir, 0755); err != nil {
			log.Printf("Failed to create destination directory %s: %v", destDir, err)
			http.Error(w, "ディレクトリの作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		finalPath := filepath.Join(destDir, newBaseName+".DAT")
		// 名前が衝突した場合の連番処理
		for i := 1; ; i++ {
			if _, err := os.Stat(finalPath); os.IsNotExist(err) {
				break
			}
			finalPath = filepath.Join(destDir, fmt.Sprintf("%s(%d)%s", newBaseName, i, ".DAT"))
		}

		// ファイルハンドルが解放されるのを少し待つ
		time.Sleep(500 * time.Millisecond)

		if err := os.Rename(downloadedFilePath, finalPath); err != nil {
			log.Printf("WARN: Failed to rename downloaded file: %v. Processing with original name.", err)
			// リネームに失敗しても、元のファイルパスで処理を試みる
			finalPath = downloadedFilePath
		} else {
			log.Printf("File successfully renamed to %s", finalPath)
		}

		processedRecords, err := dat.ProcessDatFile(conn, finalPath)
		if err != nil {
			log.Printf("ERROR: Failed to process downloaded DAT file %s: %v", finalPath, err)
			http.Error(w, "ダウンロードしたDATファイルの処理に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("e-mednetから%d件の納品データをダウンロードし、システムに登録しました。", len(processedRecords)),
		})
	}
}

// findEdgePath は一般的なインストール場所からMicrosoft Edgeの実行ファイルを探します。
func findEdgePath() string {
	// Program FilesとProgram Files (x86)の両方を検索
	lookIn := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
	}
	for _, dir := range lookIn {
		path := filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe")
		if _, err := os.Stat(path); err == nil {
			return path // 見つかったらすぐにパスを返す
		}
	}
	return "" // 見つからなければ空文字を返す
}
