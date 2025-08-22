// C:\Users\wasab\OneDrive\デスクトップ\WASABI\deadstock\handler.go

package deadstock

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

func GetDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5
		}

		// ▼▼▼ [修正点] 新しいフィルター条件を構造体に含める ▼▼▼
		filters := model.DeadStockFilters{
			StartDate:        q.Get("startDate"),
			EndDate:          q.Get("endDate"),
			ExcludeZeroStock: q.Get("excludeZeroStock") == "true",
			Coefficient:      coefficient,
			KanaName:         q.Get("kanaName"),
			DosageForm:       q.Get("dosageForm"),
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		results, err := db.GetDeadStockList(tx, filters)
		if err != nil {
			http.Error(w, "Failed to get dead stock list: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// (SaveDeadStockHandler is unchanged)
func SaveDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []model.DeadStockRecord
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.UpsertDeadStockRecordsInTx(tx, payload); err != nil {
			http.Error(w, "Failed to save dead stock records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "保存しました。"})
	}
}
