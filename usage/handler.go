// C:\Dev\WASABI\usage\handler.go

package usage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os" // osパッケージをインポート
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
	"wasabi/model"
	"wasabi/parsers"
)

const insertTransactionQuery = `
INSERT OR REPLACE INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
    subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// UploadUsageHandler は指定された固定パスからUSAGEファイルを読み込み処理します。
func UploadUsageHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ▼▼▼【ここからが修正箇所です】▼▼▼

		// Step 1: ユーザーに指示された固定ファイルパスを定義
		const targetFile = `C:\Users\wasab\OneDrive\デスクトップ\WASABI\download\USAGE\usage.csv`

		log.Printf("Processing fixed USAGE file: %s", targetFile)
		file, err := os.Open(targetFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("指定されたUSAGEファイルのオープンに失敗しました: %s。パスを確認してください。", targetFile), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Step 2: ファイルをパースする
		parsed, err := parsers.ParseUsage(file)
		if err != nil {
			http.Error(w, "USAGEファイルの解析に失敗しました: "+err.Error(), http.StatusBadRequest)
			return
		}

		// ▲▲▲【修正ここまで】▲▲▲

		// Step 3: これ以降は既存のDB登録ロジックを流用
		var originalJournalMode string
		conn.QueryRow("PRAGMA journal_mode").Scan(&originalJournalMode)
		conn.Exec("PRAGMA journal_mode = MEMORY;")
		conn.Exec("PRAGMA synchronous = OFF;")
		defer func() {
			conn.Exec("PRAGMA synchronous = FULL;")
			conn.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", originalJournalMode))
			log.Println("Database settings restored for USAGE handler.")
		}()

		filtered := removeUsageDuplicates(parsed)

		if len(filtered) == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"records": []model.TransactionRecord{}})
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction for usage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		minDate, maxDate := "99999999", "00000000"
		for _, rec := range filtered {
			if rec.Date < minDate {
				minDate = rec.Date
			}
			if rec.Date > maxDate {
				maxDate = rec.Date
			}
		}
		if err := db.DeleteUsageTransactionsInDateRange(tx, minDate, maxDate); err != nil {
			log.Printf("db.DeleteUsageTransactionsInDateRange error: %v", err)
			tx.Rollback()
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		var keyList, janList []string
		keySet, janSet := make(map[string]struct{}), make(map[string]struct{})
		for _, rec := range filtered {
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

		stmt, err := tx.Prepare(insertTransactionQuery)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to prepare statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		const batchSize = 500
		var finalRecords []model.TransactionRecord

		for i, rec := range filtered {
			ar := model.TransactionRecord{
				TransactionDate: rec.Date,
				Flag:            3,
				JanCode:         rec.JanCode,
				YjCode:          rec.YjCode,
				ProductName:     rec.ProductName,
				YjQuantity:      rec.YjQuantity,
				YjUnitName:      rec.YjUnitName,
			}

			master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("mastermanager failed for jan %s: %v", rec.JanCode, err), http.StatusInternalServerError)
				return
			}

			mappers.MapProductMasterToTransaction(&ar, master)
			if master.Origin == "JCSHMS" {
				ar.ProcessFlagMA = "COMPLETE"
			} else {
				ar.ProcessFlagMA = "PROVISIONAL"
			}

			_, err = stmt.Exec(
				ar.TransactionDate, ar.ClientCode, ar.ReceiptNumber, ar.LineNumber, ar.Flag,
				ar.JanCode, ar.YjCode, ar.ProductName, ar.KanaName, ar.UsageClassification, ar.PackageForm, ar.PackageSpec, ar.MakerName,
				ar.DatQuantity, ar.JanPackInnerQty, ar.JanQuantity, ar.JanPackUnitQty, ar.JanUnitName, ar.JanUnitCode,
				ar.YjQuantity, ar.YjPackUnitQty, ar.YjUnitName, ar.UnitPrice, ar.PurchasePrice, ar.SupplierWholesale,
				ar.Subtotal, ar.TaxAmount, ar.TaxRate, ar.ExpiryDate, ar.LotNumber, ar.FlagPoison,
				ar.FlagDeleterious, ar.FlagNarcotic, ar.FlagPsychotropic, ar.FlagStimulant,
				ar.FlagStimulantRaw, ar.ProcessFlagMA,
			)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Failed to insert record for JAN %s: %v", ar.JanCode, err), http.StatusInternalServerError)
				return
			}

			finalRecords = append(finalRecords, ar)

			if (i+1)%batchSize == 0 && i < len(filtered)-1 {
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

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"records": finalRecords,
		})
	}
}

func removeUsageDuplicates(records []model.UnifiedInputRecord) []model.UnifiedInputRecord {
	seen := make(map[string]struct{})
	var result []model.UnifiedInputRecord
	for _, r := range records {
		key := fmt.Sprintf("%s|%s|%s|%s", r.Date, r.JanCode, r.YjCode, r.ProductName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, r)
	}
	return result
}
