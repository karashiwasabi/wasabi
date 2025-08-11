package aggregation

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"
)

// GetAggregationHandler handles the request for the stock ledger report.
func GetAggregationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5 // Default value
		}

		// 新しいフィルター条件を構造体に含める
		filters := model.AggregationFilters{
			StartDate:   q.Get("startDate"),
			EndDate:     q.Get("endDate"),
			KanaName:    q.Get("kanaName"),
			DrugTypes:   strings.Split(q.Get("drugTypes"), ","),
			DosageForm:  q.Get("dosageForm"), // 剤型を取得
			Coefficient: coefficient,
		}

		results, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get aggregated data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
