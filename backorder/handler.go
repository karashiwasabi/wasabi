package backorder

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
)

// GetBackordersHandler は発注残リストを返します。
func GetBackordersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ▼▼▼ [修正点] 呼び出す関数名を変更 ▼▼▼
		backorders, err := db.GetAllBackordersList(conn)
		// ▲▲▲ 修正ここまで ▲▲▲
		if err != nil {
			http.Error(w, "Failed to get backorder list", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(backorders)
	}
}
