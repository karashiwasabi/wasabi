// C:\Dev\WASABI\reprocess\handler.go

package reprocess

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

// ▼▼▼ [修正点] ファイル全体を、新しい ProcessTransactionsHandler のみに修正 ▼▼▼

// ProcessTransactionsHandler は全ての取引データを最新のマスター情報で更新します。
func ProcessTransactionsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 全ての製品マスターをメモリにロード（高速化のため）
		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to fetch all product masters: "+err.Error(), http.StatusInternalServerError)
			return
		}
		mastersMap := make(map[string]*model.ProductMaster)
		for _, m := range allMasters {
			// 仮マスター用の合成キーもマップに追加
			key := m.ProductCode
			if m.Origin == "PROVISIONAL" {
				key = fmt.Sprintf("9999999999999%s", m.ProductName)
			}
			mastersMap[key] = m
		}

		// 全ての取引レコードを取得
		rows, err := conn.Query("SELECT " + db.TransactionColumns + " FROM transaction_records")
		if err != nil {
			http.Error(w, "Failed to fetch all transaction records: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var allRecords []model.TransactionRecord
		for rows.Next() {
			r, err := db.ScanTransactionRecord(rows)
			if err != nil {
				http.Error(w, "Failed to scan transaction record: "+err.Error(), http.StatusInternalServerError)
				return
			}
			allRecords = append(allRecords, *r)
		}

		if len(allRecords) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "再計算対象の取引データはありませんでした。"})
			return
		}

		// バッチ処理で更新
		const batchSize = 500
		updatedCount := 0
		for i := 0; i < len(allRecords); i += batchSize {
			end := i + batchSize
			if end > len(allRecords) {
				end = len(allRecords)
			}
			batch := allRecords[i:end]

			tx, err := conn.Begin()
			if err != nil {
				http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
				return
			}

			for _, rec := range batch {
				key := rec.JanCode
				if key == "" || key == "0000000000000" {
					key = fmt.Sprintf("9999999999999%s", rec.ProductName)
				}
				master, ok := mastersMap[key]
				if !ok {
					continue
				}

				mappers.MapProductMasterToTransaction(&rec, master)
				// JCSHMSマスターが見つかったPROVISIONALレコードはCOMPLETEに更新
				if rec.ProcessFlagMA == "PROVISIONAL" && master.Origin == "JCSHMS" {
					rec.ProcessFlagMA = "COMPLETE"
				}

				if err := db.UpdateFullTransactionInTx(tx, &rec); err != nil {
					tx.Rollback()
					http.Error(w, fmt.Sprintf("Failed to update record ID %d: %v", rec.ID, err), http.StatusInternalServerError)
					return
				}
				updatedCount++
			}

			if err := tx.Commit(); err != nil {
				tx.Rollback()
				http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
				return
			}
			log.Printf("Processed %d/%d records...", updatedCount, len(allRecords))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("全 %d 件の取引データを最新のマスター情報で更新しました。", updatedCount),
		})
	}
}

// ▲▲▲ 修正ここまで ▲▲▲
