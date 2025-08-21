// C:\Dev\WASABI\reprocess\handler.go

package reprocess

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
)

func ReProcessTransactionsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		provisionalRecords, err := db.GetProvisionalTransactions(tx)
		if err != nil {
			http.Error(w, "Failed to fetch provisional records: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(provisionalRecords) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "更新対象の仮登録データはありませんでした。"})
			return
		}

		keyList := make([]string, 0, len(provisionalRecords))
		keySet := map[string]struct{}{}
		for _, rec := range provisionalRecords {
			key := rec.JanCode
			if key == "" || key == "0000000000000" {
				key = fmt.Sprintf("9999999999999%s", rec.ProductName)
			}
			if _, seen := keySet[key]; !seen {
				keySet[key] = struct{}{}
				keyList = append(keyList, key)
			}
		}
		mastersMap, err := db.GetProductMastersByCodesMap(tx, keyList)
		if err != nil {
			http.Error(w, "Failed to load product masters: "+err.Error(), http.StatusInternalServerError)
			return
		}

		updatedCount := 0
		for _, rec := range provisionalRecords {
			key := rec.JanCode
			if key == "" || key == "0000000000000" {
				key = fmt.Sprintf("9999999999999%s", rec.ProductName)
			}
			master, ok := mastersMap[key]

			if !ok || master.Origin != "JCSHMS" {
				continue
			}

			mappers.MapProductMasterToTransaction(&rec, master)
			// ▼▼▼ [修正点] ProcessingStatusの設定を削除 ▼▼▼
			rec.ProcessFlagMA = "COMPLETE"
			// ▲▲▲ 修正ここまで ▲▲▲

			if err := db.UpdateFullTransactionInTx(tx, &rec); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update record ID %d: %v", rec.ID, err), http.StatusInternalServerError)
				return
			}
			updatedCount++
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d 件の仮登録データを再計算しました。", updatedCount),
		})
	}
}