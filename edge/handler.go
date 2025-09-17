package edge

import (
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

	"context" // contextパッケージをインポート

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

func writeJsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func DownloadHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) 設定読み込み
		cfg, err := config.LoadConfig()
		if err != nil {
			writeJsonError(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg.EdeUserID == "" || cfg.EdePassword == "" {
			writeJsonError(w, "IDまたはパスワードが設定されていません。", http.StatusBadRequest)
			return
		}

		// 2) 一時プロファイルディレクトリ作成
		tempDir, err := os.MkdirTemp("", "chromedp-edge-")
		if err != nil {
			writeJsonError(w, "一時プロファイルディレクトリの作成に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tempDir)

		// ▼▼▼【ここから修正】▼▼▼
		// ご指摘の箇所を、元の動作するコードに戻しました。
		// 3) Edge実行ファイルのパス (元のコードのまま)
		edgePath := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`

		// 4) ExecAllocator を作成 (元のコードのまま)
		allocCtx, allocCancel := chromedp.NewExecAllocator(
			r.Context(),
			append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.ExecPath(edgePath),
				chromedp.Flag("headless", false),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.UserDataDir(tempDir),
			)...,
		)
		defer allocCancel()
		// ▲▲▲【修正ここまで】▲▲▲

		// 5) Context とロギング設定
		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
		defer cancel()

		// 6) ダウンロードフォルダ準備
		downloadDir, err := filepath.Abs(filepath.Join(".", "download", "DAT"))
		if err != nil {
			writeJsonError(w, "ダウンロードディレクトリの絶対パス取得に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			writeJsonError(w, "ダウンロードディレクトリの作成に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}

		filesBefore, err := os.ReadDir(downloadDir)
		if err != nil {
			writeJsonError(w, "ダウンロードディレクトリの読み取りに失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}
		filesBeforeMap := make(map[string]bool)
		for _, f := range filesBefore {
			filesBeforeMap[f.Name()] = true
		}

		// 7) ログインしてボタンをクリック
		var initialTS string
		err = chromedp.Run(ctx,
			browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(downloadDir),
			chromedp.Navigate(`https://www.e-mednet.jp/`),
			chromedp.WaitVisible(`input[name="userid"]`),
			chromedp.SendKeys(`input[name="userid"]`, cfg.EdeUserID),
			chromedp.SendKeys(`input[name="userpsw"]`, cfg.EdePassword),
			chromedp.Click(`input[type="submit"][value="ログイン"]`),
			chromedp.WaitVisible(`//a[contains(@href, "busi_id=11")]`),
			chromedp.Click(`//a[contains(@href, "busi_id=11")]`),
			chromedp.WaitVisible(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.Click(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.WaitReady(`input[name="unreceive_button"]`),
			chromedp.Text(`table.result-list-table tbody tr:first-child td.col-transceiving-date`, &initialTS, chromedp.AtLeast(0)),
			chromedp.Click(`input[name="unreceive_button"]`),
		)
		if err != nil {
			writeJsonError(w, "自動操作に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 8) 結果待ちのロジック (medrecの方式)
		var newFilePath string
		var downloadSuccess, noDataFound bool
		timeout := time.After(30 * time.Second)
	CheckLoop:
		for {
			select {
			case <-timeout:
				writeJsonError(w, "30秒以内にサイトの反応が確認できませんでした。", http.StatusRequestTimeout)
				return
			case <-time.After(1 * time.Second):
				var newLatestTimestamp, resultText string

				checkCtx, cancelCheck := context.WithTimeout(ctx, 4*time.Second)
				_ = chromedp.Run(checkCtx,
					chromedp.Text(`table.result-list-table tbody tr:first-child td.col-transceiving-date`, &newLatestTimestamp, chromedp.AtLeast(0)),
				)
				cancelCheck()

				if strings.TrimSpace(newLatestTimestamp) != strings.TrimSpace(initialTS) {
					checkCtx, cancelCheck = context.WithTimeout(ctx, 4*time.Second)
					_ = chromedp.Run(checkCtx,
						chromedp.Text(`table.result-list-table tbody tr:first-child td.col-result`, &resultText, chromedp.AtLeast(0)),
					)
					cancelCheck()

					if strings.TrimSpace(resultText) == "正常完了" {
						downloadSuccess = true
						break CheckLoop
					}
					if strings.TrimSpace(resultText) == "受信データなし" {
						noDataFound = true
						break CheckLoop
					}
				}
			}
		}

		// 9) 「受信データなし」の場合のハンドリング
		if noDataFound {
			writeJsonError(w, "未受信の納品データはありませんでした。", http.StatusOK)
			return
		}

		// 10) ファイルダウンロード待機処理
		if downloadSuccess {
			timeoutFile := time.After(10 * time.Second)
		FileLoop:
			for {
				select {
				case <-timeoutFile:
					writeJsonError(w, "サイトの反応はありましたが、10秒以内にファイルが見つかりませんでした。", http.StatusInternalServerError)
					return
				case <-time.After(500 * time.Millisecond):
					filesAfter, _ := os.ReadDir(downloadDir)
					for _, f := range filesAfter {
						if !filesBeforeMap[f.Name()] && !strings.HasSuffix(f.Name(), ".crdownload") {
							newFilePath = filepath.Join(downloadDir, f.Name())
							break FileLoop
						}
					}
				}
			}
		} else {
			writeJsonError(w, "ダウンロードされたファイルの検知に失敗しました。", http.StatusInternalServerError)
			return
		}

		// 11) DAT ファイル処理
		processedRecords, err := dat.ProcessDatFile(conn, newFilePath)
		if err != nil {
			writeJsonError(w, "ダウンロードしたDATファイルの処理に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 12) レスポンス
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の納品データをダウンロードし登録しました。", len(processedRecords)),
		})
	}
}
