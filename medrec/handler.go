package medrec

import (
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

func writeJsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func findChromePath() string {
	paths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func DownloadHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.LoadConfig()
		if err != nil {
			writeJsonError(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if cfg.EmednetUserID == "" || cfg.EmednetPassword == "" {
			writeJsonError(w, "IDまたはパスワードが設定されていません。", http.StatusBadRequest)
			return
		}

		tempDir, err := os.MkdirTemp("", "chromedp-medrec-")
		if err != nil {
			writeJsonError(w, "一時プロファイルディレクトリの作成に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tempDir)

		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", true),
			chromedp.UserDataDir(tempDir),
		)

		if execPath := findChromePath(); execPath != "" {
			opts = append(opts, chromedp.ExecPath(execPath))
		}

		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
		defer cancel()

		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
		defer cancel()

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

		var initialLatestTimestamp string
		err = chromedp.Run(ctx,
			// ▼▼▼【ここを修正】 page. -> browser. に変更 ▼▼▼
			browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(downloadDir),
			// ▲▲▲【修正ここまで】▲▲▲
			chromedp.Navigate(`https://www.e-mednet.jp/`),
			chromedp.WaitVisible(`input[name="userid"]`),
			chromedp.SendKeys(`input[name="userid"]`, cfg.EmednetUserID),
			chromedp.SendKeys(`input[name="userpsw"]`, cfg.EmednetPassword),
			chromedp.Click(`input[type="submit"][value="ログイン"]`),
			chromedp.WaitVisible(`//a[contains(@href, "busi_id=11")]`),
			chromedp.Click(`//a[contains(@href, "busi_id=11")]`),
			chromedp.WaitVisible(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.Click(`//a[contains(text(), "納品受信(JAN)")]`),
			chromedp.WaitReady(`input[name="unreceive_button"]`),
			chromedp.Text(`table.result-list-table tbody tr:first-child td.col-transceiving-date`, &initialLatestTimestamp, chromedp.AtLeast(0)),
			chromedp.Click(`input[name="unreceive_button"]`),
		)
		if err != nil {
			writeJsonError(w, "自動操作に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

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

				if strings.TrimSpace(newLatestTimestamp) != strings.TrimSpace(initialLatestTimestamp) {
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

		if noDataFound {
			writeJsonError(w, "未受信の納品データはありませんでした。", http.StatusOK)
			return
		}

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

		processedRecords, err := dat.ProcessDatFile(conn, newFilePath)
		if err != nil {
			writeJsonError(w, "ダウンロードしたDATファイルの処理に失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の納品データをダウンロードし登録しました。", len(processedRecords)),
		})
	}
}
