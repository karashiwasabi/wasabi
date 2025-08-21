package stock

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
)

// GetCurrentStockHandler はJANコード指定で現在の理論在庫を返します。
func GetCurrentStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		janCode := r.URL.Query().Get("jan_code")
		if janCode == "" {
			http.Error(w, "jan_code parameter is required", http.StatusBadRequest)
			return
		}

		stock, err := db.CalculateCurrentStockForProduct(conn, janCode)
		if err != nil {
			http.Error(w, "Failed to calculate stock: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]float64{"stock": stock})
	}
}