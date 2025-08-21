// C:\Dev\WASABI\medrec\handler.go

package medrec

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wasabi/config"

	"golang.org/x/net/publicsuffix" // Cookie Jarのために追加
)

// DownloadHandler handles the entire process of logging into e-mednet and downloading the DAT file.
func DownloadHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. WASABIの設定ファイルからID/パスワードを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg.EmednetUserID == "" || cfg.EmednetPassword == "" {
			http.Error(w, "e-mednetのIDまたはパスワードが設定されていません。設定画面を確認してください。", http.StatusBadRequest)
			return
		}

		// 2. Cookieを保持するHTTPクライアントを生成
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			http.Error(w, "Cookie Jarの作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		client := &http.Client{Jar: jar}

		// 3. ログインリクエストを送信
		loginURL := "https://www.e-mednet.jp/NASApp/ASP/MenuMain"
		loginData := url.Values{}
		loginData.Set("userid", cfg.EmednetUserID)    // 確実なキー名を使用 [cite: 193]
		loginData.Set("userpsw", cfg.EmednetPassword) // 確実なキー名を使用 [cite: 193]
		loginData.Set("loginkbn", "login")

		loginResp, err := client.PostForm(loginURL, loginData)
		if err != nil {
			http.Error(w, "e-mednetへのログインリクエストに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer loginResp.Body.Close()

		// レスポンスボディを読んでログイン成否を判定
		bodyBytes, err := io.ReadAll(loginResp.Body)
		if err != nil {
			http.Error(w, "e-mednetからのレスポンス読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if loginResp.StatusCode != http.StatusOK || strings.Contains(string(bodyBytes), "ログインＩＤまたはパスワードに誤りがあります") {
			log.Printf("e-mednet login failed. Server response:\n---\n%s\n---", string(bodyBytes))
			http.Error(w, "e-mednetへのログインに失敗しました。ID/パスワードを確認してください。", http.StatusUnauthorized)
			return
		}
		log.Println("e-mednet login successful.")

		// 4. ダウンロードリクエストを送信
		downloadURL := "https://www.e-mednet.jp/NASApp/ASP/SrDeliveryJanDownload/downloadAll"
		downloadData := url.Values{}
		downloadData.Set("busi_id", "11")
		downloadData.Set("func_id", "02")
		downloadData.Set("sys", "310")
		downloadData.Set("ver", "1.00")
		downloadData.Set("type", "1")

		downloadResp, err := client.PostForm(downloadURL, downloadData)
		if err != nil {
			http.Error(w, "ダウンロードリクエストに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer downloadResp.Body.Close()
		log.Println("e-mednet download request successful.")

		if downloadResp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("ダウンロードに失敗しました (HTTP Status: %d)", downloadResp.StatusCode), http.StatusInternalServerError)
			return
		}

		// 5. WASABIのルールに従ってファイルを保存
		baseDir := filepath.Join("download", "DAT")
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			http.Error(w, "ベースディレクトリの作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		timestamp := time.Now().Format("20060102150405")
		saveDir := filepath.Join(baseDir, timestamp)
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			http.Error(w, "タイムスタンプディレクトリの作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// ヘッダーからファイル名を取得、なければデフォルト値を使用
		filename := "納品データ.DAT"
		contentDisposition := downloadResp.Header.Get("Content-Disposition")
		if contentDisposition != "" {
			_, params, err := mime.ParseMediaType(contentDisposition)
			if err == nil {
				if f, ok := params["filename"]; ok {
					filename = f
				}
			}
		}

		fullPath := filepath.Join(saveDir, filename)
		file, err := os.Create(fullPath)
		if err != nil {
			http.Error(w, "ファイル作成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// レスポンスボディをファイルに書き込む
		_, err = io.Copy(file, downloadResp.Body)
		if err != nil {
			http.Error(w, "ファイル保存に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("File saved to %s", fullPath)

		// 6. 成功メッセージを返す
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("ファイルを %s に保存しました。", fullPath),
		})
	}
}