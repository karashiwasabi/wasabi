// C:\Dev\WASABI\medrec\handler.go

package medrec

import (
	"bytes"
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
)

// DownloadHandler handles the entire process of logging into e-mednet and downloading the DAT file.
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

		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}

		loginURL := "https://www.e-mednet.jp/NASApp/ASP/MenuMain"
		loginData := url.Values{}
		// ▼▼▼ [修正点] HTMLのフォーム名と完全に一致させる ▼▼▼
		loginData.Set("userid", cfg.EmednetUserID)
		loginData.Set("userpsw", cfg.EmednetPassword)
		// ▲▲▲ 修正ここまで ▲▲▲
		loginData.Set("loginkbn", "login")

		loginReq, _ := http.NewRequest("POST", loginURL, strings.NewReader(loginData.Encode()))
		loginReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		loginResp, err := client.Do(loginReq)
		if err != nil {
			http.Error(w, "e-mednetへのログインリクエストに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer loginResp.Body.Close()

		bodyBytes, err := io.ReadAll(loginResp.Body)
		if err != nil {
			http.Error(w, "e-mednetからのレスポンス読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		loginResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if loginResp.StatusCode != http.StatusOK || strings.Contains(string(bodyBytes), "ログインＩＤまたはパスワードに誤りがあります") {
			log.Printf("e-mednet login failed. Server response:\n---\n%s\n---", string(bodyBytes))
			http.Error(w, fmt.Sprintf("e-mednetへのログインに失敗しました。ID/パスワードを確認するか、サイトの仕様が変更された可能性があります。"), http.StatusUnauthorized)
			return
		}
		log.Println("e-mednet login successful.")

		downloadURL := "https://www.e-mednet.jp/NASApp/ASP/SrDeliveryJanDownload/downloadAll"
		downloadData := url.Values{}
		downloadData.Set("busi_id", "11")
		downloadData.Set("func_id", "02")
		downloadData.Set("sys", "310")
		downloadData.Set("ver", "1.00")
		downloadData.Set("type", "1")

		downloadReq, _ := http.NewRequest("POST", downloadURL, strings.NewReader(downloadData.Encode()))
		downloadReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		downloadResp, err := client.Do(downloadReq)
		if err != nil {
			http.Error(w, "ダウンロードリクエストに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer downloadResp.Body.Close()
		log.Println("e-mednet download request successful.")

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

		fileBytes, err := io.ReadAll(downloadResp.Body)
		if err != nil {
			http.Error(w, "ダウンロードデータの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(fullPath, fileBytes, 0644); err != nil {
			http.Error(w, "ファイル保存に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("File saved to %s", fullPath)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("ファイルを %s に保存しました。", fullPath),
		})
	}
}
