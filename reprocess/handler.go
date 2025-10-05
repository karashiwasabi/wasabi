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
			mastersMap[m.ProductCode] = m
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
			rec, err := db.ScanTransactionRecord(rows)
			if err != nil {
				http.Error(w, "Failed to scan transaction record: "+err.Error(), http.StatusInternalServerError)
				return
			}
			allRecords = append(allRecords, *rec)
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
				master, ok := mastersMap[rec.JanCode]
				if !ok {
					continue
				}

				// 1. 最新のマスター情報をレコードにマッピング
				mappers.MapProductMasterToTransaction(&rec, master)

				// 2. 数量を再計算
				if rec.JanQuantity > 0 && master.JanPackInnerQty > 0 {
					rec.YjQuantity = rec.JanQuantity * master.JanPackInnerQty
				} else if rec.YjQuantity > 0 && rec.JanQuantity == 0 && master.JanPackInnerQty > 0 {
					rec.JanQuantity = rec.YjQuantity / master.JanPackInnerQty
				}

				// ▼▼▼【ここから金額再計算ロジックを修正】▼▼▼
				// 3. 金額を再計算 (取引種別に応じてロジックを分岐)
				switch rec.Flag {
				case 0, 3, 4, 5: // 棚卸系(0,4,5)と処方(3)の場合
					// 常にマスターの薬価(YJ単価)を正として単価と金額を再計算する
					rec.UnitPrice = master.NhiPrice
					rec.Subtotal = rec.YjQuantity * rec.UnitPrice
				default: // その他の取引(納品、返品、入出庫など)
					var unitPriceToUse float64
					if rec.UnitPrice > 0 {
						// 取引時に記録された単価を尊重する
						unitPriceToUse = rec.UnitPrice
					} else {
						// 単価が記録されていない場合のみマスターの薬価を使用する
						unitPriceToUse = master.NhiPrice
						rec.UnitPrice = unitPriceToUse // レコードの単価も更新
					}
					// 金額は数量の変更を反映するために再計算する
					rec.Subtotal = rec.YjQuantity * unitPriceToUse
				}
				// ▲▲▲【修正ここまで】▲▲▲

				// 4. 処理ステータスを更新
				if rec.ProcessFlagMA == "PROVISIONAL" && master.Origin == "JCSHMS" {
					rec.ProcessFlagMA = "COMPLETE"
				}

				// 5. データベースを更新
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
