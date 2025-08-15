// C:\Dev\WASABI\settings\handler.go

package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/config"
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
