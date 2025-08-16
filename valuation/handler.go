package valuation

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
)

// GetValuationHandler は在庫評価レポートのデータを返します
func GetValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		}

		results, err := db.GetInventoryValuation(conn, date)
		if err != nil {
			http.Error(w, "Failed to get inventory valuation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
