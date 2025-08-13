// C:\Dev\WASABI\dat\handler.go

package dat

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

// UploadDatHandler はDATファイルのアップロードを処理します。
func UploadDatHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ▼▼▼ [修正点] パフォーマンス向上のため、処理中だけDB設定を一時的に変更 ▼▼▼
		var originalJournalMode string
		conn.QueryRow("PRAGMA journal_mode").Scan(&originalJournalMode)

		conn.Exec("PRAGMA journal_mode = MEMORY;")
		conn.Exec("PRAGMA synchronous = OFF;")

		defer func() {
			conn.Exec("PRAGMA synchronous = FULL;")
			conn.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", originalJournalMode))
			log.Println("Database settings restored for DAT handler.")
		}()
		// ▲▲▲ 修正ここまで ▲▲▲

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()

		// 1. Parse all uploaded files
		var allParsedRecords []model.UnifiedInputRecord
		for _, fileHeader := range r.MultipartForm.File["file"] {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Failed to open file %s: %v", fileHeader.Filename, err)
				continue
			}
			defer file.Close()
			parsed, err := parsers.ParseDat(file)
			if err != nil {
				log.Printf("Failed to parse file %s: %v", fileHeader.Filename, err)
				continue
			}
			allParsedRecords = append(allParsedRecords, parsed...)
		}

		// 2. Remove duplicates
		filteredRecords := removeDatDuplicates(allParsedRecords)

		if len(filteredRecords) == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "No new records to process.",
				"records": []model.TransactionRecord{},
			})
			return
		}

		// ▼▼▼ [修正点] バッチ処理の導入 ▼▼▼
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
			return
		}

		// 3. Pre-fetch master data
		var keyList, janList []string
		keySet, janSet := make(map[string]struct{}), make(map[string]struct{})
		for _, rec := range filteredRecords {
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
		mastersMap, err := db.GetProductMastersByCodesMap(conn, keyList)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to pre-fetch product masters", http.StatusInternalServerError)
			return
		}
		jcshmsMap, err := db.GetJcshmsByCodesMap(conn, janList)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to pre-fetch JCSHMS data", http.StatusInternalServerError)
			return
		}

		// 4. Prepare statement for batch insert
		stmt, err := tx.Prepare(insertTransactionQuery)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to prepare statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		// 5. Process records in batches
		const batchSize = 500
		var finalRecords []model.TransactionRecord

		for i, rec := range filteredRecords {
			ar := model.TransactionRecord{
				TransactionDate: rec.Date, ClientCode: rec.ClientCode, ReceiptNumber: rec.ReceiptNumber,
				LineNumber: rec.LineNumber, Flag: rec.Flag, JanCode: rec.JanCode,
				ProductName: rec.ProductName, DatQuantity: rec.DatQuantity, UnitPrice: rec.UnitPrice,
				Subtotal: rec.Subtotal, ExpiryDate: rec.ExpiryDate, LotNumber: rec.LotNumber,
			}

			master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("mastermanager failed for jan %s: %v", rec.JanCode, err), http.StatusInternalServerError)
				return
			}

			if master.JanPackUnitQty > 0 {
				ar.JanQuantity = ar.DatQuantity * master.JanPackUnitQty
			}
			if master.YjPackUnitQty > 0 {
				ar.YjQuantity = ar.DatQuantity * master.YjPackUnitQty
			}
			mappers.MapProductMasterToTransaction(&ar, master)

			if master.Origin == "JCSHMS" {
				ar.ProcessFlagMA = "COMPLETE"
				ar.ProcessingStatus = sql.NullString{String: "completed", Valid: true}
			} else {
				ar.ProcessFlagMA = "PROVISIONAL"
				ar.ProcessingStatus = sql.NullString{String: "provisional", Valid: true}
			}

			_, err = stmt.Exec(
				ar.TransactionDate, ar.ClientCode, ar.ReceiptNumber, ar.LineNumber, ar.Flag,
				ar.JanCode, ar.YjCode, ar.ProductName, ar.KanaName, ar.UsageClassification, ar.PackageForm, ar.PackageSpec, ar.MakerName,
				ar.DatQuantity, ar.JanPackInnerQty, ar.JanQuantity, ar.JanPackUnitQty, ar.JanUnitName, ar.JanUnitCode,
				ar.YjQuantity, ar.YjPackUnitQty, ar.YjUnitName, ar.UnitPrice, ar.PurchasePrice, ar.SupplierWholesale,
				ar.Subtotal, ar.TaxAmount, ar.TaxRate, ar.ExpiryDate, ar.LotNumber, ar.FlagPoison,
				ar.FlagDeleterious, ar.FlagNarcotic, ar.FlagPsychotropic, ar.FlagStimulant,
				ar.FlagStimulantRaw, ar.ProcessFlagMA, ar.ProcessingStatus,
			)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Failed to insert record for JAN %s: %v", ar.JanCode, err), http.StatusInternalServerError)
				return
			}

			finalRecords = append(finalRecords, ar)

			if (i+1)%batchSize == 0 && i < len(filteredRecords)-1 {
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
		// ▲▲▲ 修正ここまで ▲▲▲

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Parsed and processed DAT files successfully",
			"records": finalRecords,
		})
	}
}

// removeDatDuplicates はDATレコードの重複を削除します。
func removeDatDuplicates(records []model.UnifiedInputRecord) []model.UnifiedInputRecord {
	seen := make(map[string]struct{})
	var result []model.UnifiedInputRecord
	for _, r := range records {
		key := fmt.Sprintf("%s|%s|%s|%s", r.Date, r.ClientCode, r.ReceiptNumber, r.LineNumber)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, r)
	}
	return result
}
