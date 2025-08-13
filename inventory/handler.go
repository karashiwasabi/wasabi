// C:\Dev\WASABI\inventory\handler.go

package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
	"wasabi/model"
	"wasabi/parsers"
)

// insertTransactionQuery defines the SQL statement for inserting transaction records.
const insertTransactionQuery = `
INSERT OR REPLACE INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
    subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma, processing_status
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// UploadInventoryHandler handles the inventory file upload process.
func UploadInventoryHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var originalJournalMode string
		conn.QueryRow("PRAGMA journal_mode").Scan(&originalJournalMode)

		conn.Exec("PRAGMA journal_mode = MEMORY;")
		conn.Exec("PRAGMA synchronous = OFF;")

		defer func() {
			conn.Exec("PRAGMA synchronous = FULL;")
			conn.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", originalJournalMode))
			log.Println("Database settings restored for Inventory handler.")
		}()

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "File upload error", http.StatusBadRequest)
			return
		}
		defer file.Close()

		parsedData, err := parsers.ParseInventoryFile(file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse file: %v", err), http.StatusBadRequest)
			return
		}
		date := parsedData.Date
		if date == "" {
			http.Error(w, "Inventory date not found in file's H record", http.StatusBadRequest)
			return
		}

		recordsToProcess := parsedData.Records
		if len(recordsToProcess) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "No records to process.",
				"details": []model.TransactionRecord{},
			})
			return
		}

		for i := range recordsToProcess {
			recordsToProcess[i].YjQuantity = recordsToProcess[i].JanQuantity * recordsToProcess[i].JanPackInnerQty
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}

		if err := db.DeleteTransactionsByFlagAndDate(tx, 0, date); err != nil { // Flag 0 for inventory
			tx.Rollback()
			http.Error(w, "Failed to delete existing inventory data for date "+date, http.StatusInternalServerError)
			return
		}

		var keyList, janList []string
		keySet, janSet := make(map[string]struct{}), make(map[string]struct{})
		for _, rec := range recordsToProcess {
			if rec.JanCode != "" && rec.JanCode != "0000000000000" {
				if _, seen := janSet[rec.JanCode]; !seen {
					janSet[rec.JanCode] = struct{}{}
					janList = append(janList, rec.JanCode)
				}
			}
			key := rec.JanCode
			if key == "" || key == "0000000000000" {
				key = fmt.Sprintf("9999999999999%s", rec.ProductName)
			}
			if _, seen := keySet[key]; !seen {
				keySet[key] = struct{}{}
				keyList = append(keyList, key)
			}
		}

		// ▼▼▼ [修正点] マスター取得をコネクション(conn)ではなくトランザクション(tx)で行う ▼▼▼
		mastersMap, err := db.GetProductMastersByCodesMap(tx, keyList)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to pre-fetch product masters", http.StatusInternalServerError)
			return
		}
		jcshmsMap, err := db.GetJcshmsByCodesMap(tx, janList)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to pre-fetch JCSHMS data", http.StatusInternalServerError)
			return
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		stmt, err := tx.Prepare(insertTransactionQuery)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to prepare statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		const batchSize = 500
		var finalRecords []model.TransactionRecord
		receiptNumber := fmt.Sprintf("INV%s", date)

		for i, rec := range recordsToProcess {
			tr := model.TransactionRecord{
				Flag: 0, JanCode: rec.JanCode, ProductName: rec.ProductName, YjQuantity: rec.YjQuantity,
				TransactionDate: date, ReceiptNumber: receiptNumber, LineNumber: fmt.Sprintf("%d", i+1),
			}

			master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("mastermanager failed for jan %s: %v", rec.JanCode, err), http.StatusInternalServerError)
				return
			}

			if master.JanPackInnerQty > 0 {
				tr.JanQuantity = tr.YjQuantity / master.JanPackInnerQty
			}
			mappers.MapProductMasterToTransaction(&tr, master)

			if master.Origin == "JCSHMS" {
				tr.ProcessFlagMA = "COMPLETE"
				tr.ProcessingStatus = sql.NullString{String: "completed", Valid: true}
			} else {
				tr.ProcessFlagMA = "PROVISIONAL"
				tr.ProcessingStatus = sql.NullString{String: "provisional", Valid: true}
			}

			_, err = stmt.Exec(
				tr.TransactionDate, tr.ClientCode, tr.ReceiptNumber, tr.LineNumber, tr.Flag,
				tr.JanCode, tr.YjCode, tr.ProductName, tr.KanaName, tr.UsageClassification, tr.PackageForm, tr.PackageSpec, tr.MakerName,
				tr.DatQuantity, tr.JanPackInnerQty, tr.JanQuantity, tr.JanPackUnitQty, tr.JanUnitName, tr.JanUnitCode,
				tr.YjQuantity, tr.YjPackUnitQty, tr.YjUnitName, tr.UnitPrice, tr.PurchasePrice, tr.SupplierWholesale,
				tr.Subtotal, tr.TaxAmount, tr.TaxRate, tr.ExpiryDate, tr.LotNumber, tr.FlagPoison,
				tr.FlagDeleterious, tr.FlagNarcotic, tr.FlagPsychotropic, tr.FlagStimulant,
				tr.FlagStimulantRaw, tr.ProcessFlagMA, tr.ProcessingStatus,
			)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Failed to insert record for JAN %s: %v", tr.JanCode, err), http.StatusInternalServerError)
				return
			}

			finalRecords = append(finalRecords, tr)

			if (i+1)%batchSize == 0 && i < len(recordsToProcess)-1 {
				if err := tx.Commit(); err != nil {
					log.Printf("transaction commit error (batch): %v", err)
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}
				tx, err = conn.Begin()
				if err != nil {
					http.Error(w, "Failed to begin next transaction", http.StatusInternalServerError)
					return
				}
				stmt, err = tx.Prepare(insertTransactionQuery)
				if err != nil {
					tx.Rollback()
					http.Error(w, "Failed to re-prepare statement", http.StatusInternalServerError)
					return
				}
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("transaction commit error (final): %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("%d件の棚卸データを登録しました。", len(finalRecords)),
			"details": finalRecords,
		})
	}
}
