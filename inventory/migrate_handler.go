package inventory

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

// MigrationResultRow は移行処理の各行の結果を格納します
type MigrationResultRow struct {
	OriginalRow   []string                 `json:"originalRow"`
	ParsedRecord  model.UnifiedInputRecord `json:"parsedRecord"`
	MasterCreated string                   `json:"masterCreated"` // "JCSHMS", "PROVISIONAL", "EXISTED"
	ResultRecord  *model.TransactionRecord `json:"resultRecord"`
	Error         string                   `json:"error"`
	IsZeroFill    bool                     `json:"isZeroFill,omitempty"`
}

// MigrateInventoryHandler は在庫移行用のCSVアップロードを処理します
func MigrateInventoryHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "ファイルのアップロードエラー: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		br := bufio.NewReader(file)
		bom, err := br.Peek(3)
		if err == nil && bom[0] == 0xef && bom[1] == 0xbb && bom[2] == 0xbf {
			br.Discard(3)
		}

		csvReader := csv.NewReader(br)
		csvReader.LazyQuotes = true
		allRows, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "CSVファイルの解析に失敗: "+err.Error(), http.StatusBadRequest)
			return
		}

		if len(allRows) < 2 {
			http.Error(w, "CSVにヘッダー行またはデータ行がありません。", http.StatusBadRequest)
			return
		}

		headerMap := make(map[string]int)
		for i, header := range allRows[0] {
			headerMap[header] = i
		}

		dateIdx, okDate := headerMap["inventory_date"]
		codeIdx, okCode := headerMap["product_code"]
		qtyIdx, okQty := headerMap["quantity"]

		if !okDate || !okCode || !okQty {
			http.Error(w, "CSVヘッダーに 'inventory_date', 'product_code', 'quantity' が見つかりません。", http.StatusBadRequest)
			return
		}

		recordsByDate := make(map[string][]model.UnifiedInputRecord)
		originalRowsByDate := make(map[string][][]string)

		for i, row := range allRows {
			if i == 0 {
				continue
			}
			date := row[dateIdx]
			code := strings.Trim(strings.TrimSpace(row[codeIdx]), `="`)
			qty, _ := strconv.ParseFloat(row[qtyIdx], 64)

			if date != "" && code != "" {
				recordsByDate[date] = append(recordsByDate[date], model.UnifiedInputRecord{
					Date:       date,
					JanCode:    code,
					YjQuantity: qty,
				})
				originalRowsByDate[date] = append(originalRowsByDate[date], row)
			}
		}

		var finalResults []MigrationResultRow
		var totalImported int

		for date, recs := range recordsByDate {
			var dateResults []MigrationResultRow
			for i := range recs {
				dateResults = append(dateResults, MigrationResultRow{
					OriginalRow:  originalRowsByDate[date][i],
					ParsedRecord: recs[i],
				})
			}

			tx, err := conn.Begin()
			if err != nil {
				for i := range dateResults {
					dateResults[i].Error = "トランザクション開始エラー: " + err.Error()
				}
				finalResults = append(finalResults, dateResults...)
				continue
			}

			var productCodes []string
			csvProductCodesMap := make(map[string]struct{})
			for _, rec := range recs {
				if _, exists := csvProductCodesMap[rec.JanCode]; !exists {
					productCodes = append(productCodes, rec.JanCode)
					csvProductCodesMap[rec.JanCode] = struct{}{}
				}
			}

			mastersMap, err := db.GetProductMastersByCodesMap(tx, productCodes)
			if err != nil {
				tx.Rollback()
				for i := range dateResults {
					dateResults[i].Error = "既存マスターの検索中にエラーが発生: " + err.Error()
				}
				finalResults = append(finalResults, dateResults...)
				continue
			}

			for i, rec := range recs {
				if _, exists := mastersMap[rec.JanCode]; !exists {
					masterStatus := ""
					jcshms, errJcshms := db.GetJcshmsRecordByJan(tx, rec.JanCode)
					if errJcshms == nil && jcshms != nil {
						newMasterInput := mappers.JcshmsToProductMasterInput(jcshms, rec.JanCode)
						if errUpsert := db.UpsertProductMasterInTx(tx, newMasterInput); errUpsert == nil {
							masterStatus = "JCSHMS"
						}
					} else {
						newYjCode, errSeq := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
						if errSeq == nil {
							provisionalMaster := model.ProductMasterInput{
								ProductCode: rec.JanCode, YjCode: newYjCode,
								ProductName: fmt.Sprintf("（JCSHMS未登録 JAN: %s）", rec.JanCode), Origin: "PROVISIONAL",
							}
							if errUpsert := db.UpsertProductMasterInTx(tx, provisionalMaster); errUpsert == nil {
								masterStatus = "PROVISIONAL"
							}
						}
					}
					dateResults[i].MasterCreated = masterStatus
				} else {
					dateResults[i].MasterCreated = "EXISTED"
				}
			}

			mastersMap, err = db.GetProductMastersByCodesMap(tx, productCodes)
			if err != nil {
				tx.Rollback()
				for i := range dateResults {
					dateResults[i].Error = "マスターの再検索中にエラーが発生: " + err.Error()
				}
				finalResults = append(finalResults, dateResults...)
				continue
			}

			if err := db.DeleteTransactionsByFlagAndDate(tx, 0, date); err != nil {
				tx.Rollback()
				for i := range dateResults {
					dateResults[i].Error = "古い棚卸データの削除に失敗: " + err.Error()
				}
				finalResults = append(finalResults, dateResults...)
				continue
			}

			for i, rec := range recs {
				master, ok := mastersMap[rec.JanCode]
				if !ok {
					dateResults[i].Error = "マスターデータの解決に失敗しました。"
					continue
				}

				tr := model.TransactionRecord{
					TransactionDate: rec.Date, Flag: 0, JanCode: rec.JanCode, YjQuantity: rec.YjQuantity,
					ReceiptNumber: fmt.Sprintf("MIGRATE-%s", date), LineNumber: strconv.Itoa(i + 1),
				}
				if master.JanPackInnerQty > 0 {
					tr.JanQuantity = tr.YjQuantity / master.JanPackInnerQty
				}
				mappers.MapProductMasterToTransaction(&tr, master)
				tr.ProcessFlagMA = "COMPLETE"

				// ▼▼▼【修正】Subtotalを計算する処理を追加 ▼▼▼
				tr.Subtotal = tr.YjQuantity * tr.UnitPrice
				// ▲▲▲【修正ここまで】▲▲▲

				if err := db.PersistTransactionRecordsInTx(tx, []model.TransactionRecord{tr}); err != nil {
					dateResults[i].Error = "レコード登録に失敗: " + err.Error()
					continue
				}
				dateResults[i].ResultRecord = &tr
				totalImported++
			}

			allMasters, err := db.GetAllProductMasters(tx)
			if err != nil {
				tx.Rollback()
				errorMsg := "ゼロフィル対象の全マスター取得に失敗: " + err.Error()
				for j := range dateResults {
					if dateResults[j].Error == "" {
						dateResults[j].Error = errorMsg
					}
				}
				finalResults = append(finalResults, dateResults...)
				continue
			}

			var zeroFillRecords []model.TransactionRecord
			var zeroFillResults []MigrationResultRow
			receiptNumber := fmt.Sprintf("MIGRATE-%s", date)
			zeroFillCounter := 0

			for _, master := range allMasters {
				if _, existsInCsv := csvProductCodesMap[master.ProductCode]; !existsInCsv {
					zeroFillCounter++
					tr := model.TransactionRecord{
						TransactionDate: date,
						Flag:            0,
						JanCode:         master.ProductCode,
						YjQuantity:      0,
						JanQuantity:     0,
						ReceiptNumber:   receiptNumber,
						LineNumber:      fmt.Sprintf("Z%d", zeroFillCounter),
						ProcessFlagMA:   "COMPLETE",
						UnitPrice:       0, // 金額も0なので単価も0
						Subtotal:        0,
					}
					mappers.MapProductMasterToTransaction(&tr, master)
					tr.UnitPrice = master.NhiPrice // ただし単価は記録しておく
					zeroFillRecords = append(zeroFillRecords, tr)

					zeroFillResults = append(zeroFillResults, MigrationResultRow{
						OriginalRow:   []string{"- (ゼロフィル対象) -"},
						ParsedRecord:  model.UnifiedInputRecord{JanCode: master.ProductCode, YjQuantity: 0},
						MasterCreated: "EXISTED",
						ResultRecord:  &tr,
						IsZeroFill:    true,
					})
				}
			}

			if len(zeroFillRecords) > 0 {
				if err := db.PersistTransactionRecordsInTx(tx, zeroFillRecords); err != nil {
					tx.Rollback()
					errorMsg := "ゼロフィルレコードのDB保存に失敗: " + err.Error()
					for j := range dateResults {
						if dateResults[j].Error == "" {
							dateResults[j].Error = errorMsg
						}
					}
					finalResults = append(finalResults, dateResults...)
					continue
				}
				totalImported += len(zeroFillRecords)
			}

			finalResults = append(finalResults, dateResults...)
			finalResults = append(finalResults, zeroFillResults...)

			if err := tx.Commit(); err != nil {
				log.Printf("Failed to commit transaction for date %s: %v", date, err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("計%d件の在庫データを処理しました。", totalImported),
			"details": finalResults,
		})
	}
}
