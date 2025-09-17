// C:\Dev\WASABI\settings\handler.go

package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
)

// GetSettingsHandler returns the current settings.
func GetSettingsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "Failed to load settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

// SaveSettingsHandler saves the settings.
func SaveSettingsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload config.Config
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// 1. まず現在の設定を全てサーバーから読み込む
		currentSettings, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "Failed to load current settings", http.StatusInternalServerError)
			return
		}

		// 2. 読み込んだ現在の設定に、画面の入力値をマージ（上書き）する
		currentSettings.EmednetUserID = payload.EmednetUserID
		currentSettings.EmednetPassword = payload.EmednetPassword
		currentSettings.EdeUserID = payload.EdeUserID
		currentSettings.EdePassword = payload.EdePassword
		currentSettings.UsageFolderPath = payload.UsageFolderPath
		// ▼▼▼【ここから修正】▼▼▼
		// 新しい期間日数の値をペイロードから読み取って上書きする
		currentSettings.CalculationPeriodDays = payload.CalculationPeriodDays
		// ▲▲▲【修正ここまで】▲▲▲

		// 3. 完成した設定オブジェクトをサーバーに送信して保存
		if err := config.SaveConfig(currentSettings); err != nil {
			http.Error(w, "Failed to save settings: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "設定を保存しました。"})
	}
}

// WholesalersHandler は卸業者に関するリクエストを処理します。
func WholesalersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			wholesalers, err := db.GetAllWholesalers(conn)
			if err != nil {
				http.Error(w, "Failed to get wholesalers", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wholesalers)

		case http.MethodPost:
			var payload model.Wholesaler
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if payload.Code == "" || payload.Name == "" {
				http.Error(w, "Code and Name are required", http.StatusBadRequest)
				return
			}
			if err := db.CreateWholesaler(conn, payload.Code, payload.Name); err != nil {
				http.Error(w, "Failed to create wholesaler", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"message": "卸業者を追加しました。"})

		case http.MethodDelete:
			code := strings.TrimPrefix(r.URL.Path, "/api/settings/wholesalers/")
			if code == "" {
				http.Error(w, "Wholesaler code is required", http.StatusBadRequest)
				return
			}
			if err := db.DeleteWholesaler(conn, code); err != nil {
				http.Error(w, "Failed to delete wholesaler", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"message": "卸業者を削除しました。"})

		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ClearTransactionsHandler は全ての取引データを削除するリクエストを処理します。
func ClearTransactionsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := db.ClearAllTransactions(conn); err != nil {
			http.Error(w, "Failed to clear all transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "全ての取引データを削除しました。"})
	}
}

// ClearMastersHandler は全ての製品マスターデータを削除するリクエストを処理します。
func ClearMastersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := db.ClearAllProductMasters(conn); err != nil {
			http.Error(w, "Failed to clear all product masters: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "全ての製品マスターを削除しました。"})
	}
}

// GetUsagePathHandlerは設定からusageFolderPathの値のみを返します。
func GetUsagePathHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path": cfg.UsageFolderPath,
		})
	}
}
