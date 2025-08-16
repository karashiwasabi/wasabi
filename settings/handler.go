// C:\Dev\WASABI\settings\handler.go

package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings" // stringsパッケージをインポート
	"wasabi/config"
	"wasabi/db"    // dbパッケージをインポート
	"wasabi/model" // modelパッケージをインポート
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

		if err := config.SaveConfig(payload); err != nil {
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
