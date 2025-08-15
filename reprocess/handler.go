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

// ▼▼▼ [修正点] ロジック全体を書き換え、デッドロックを解消 ▼▼▼
func ReProcessTransactionsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 最初にトランザクションを開始
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// 2. トランザクション内で、更新対象のリストを取得
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

		// 3. トランザクション内で、関連マスターを一括取得
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

		// 4. 一件ずつマッピング＆更新
		updatedCount := 0
		for _, rec := range provisionalRecords {
			key := rec.JanCode
			if key == "" || key == "0000000000000" {
				key = fmt.Sprintf("9999999999999%s", rec.ProductName)
			}
			master, ok := mastersMap[key]

			if !ok || master.Origin == "PROVISIONAL" {
				continue
			}

			mappers.MapProductMasterToTransaction(&rec, master)
			// 再計算なので、ステータスをCOMPLETEに変更
			rec.ProcessFlagMA = "COMPLETE"
			rec.ProcessingStatus = sql.NullString{String: "completed", Valid: true}

			if err := db.UpdateFullTransactionInTx(tx, &rec); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update record ID %d: %v", rec.ID, err), http.StatusInternalServerError)
				return
			}
			updatedCount++
		}

		// 5. コミットして完了
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

// ▲▲▲ 修正ここまで ▲▲▲
