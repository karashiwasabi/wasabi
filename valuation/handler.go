package valuation

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
	"wasabi/model" // modelパッケージをインポート
)

// GetValuationHandler は在庫評価レポートのデータを返します
func GetValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// ▼▼▼ [修正点] フィルターを構造体で受け取るように変更 ▼▼▼
		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		}

		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		}

		results, err := db.GetInventoryValuation(conn, filters)
		// ▲▲▲ 修正ここまで ▲▲▲

		if err != nil {
			http.Error(w, "Failed to get inventory valuation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
