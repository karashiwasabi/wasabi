// C:\Users\wasab\OneDrive\デス-
// WASABI/dat/handler.go

package dat

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
	"wasabi/model"
	"wasabi/parsers"
)

// insertTransactionQuery は取引レコードをデータベースに挿入または置換するためのSQLクエリです。
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

// UploadDatHandler はDATファイルのアップロードを処理するHTTPハンドラです。
func UploadDatHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()

		var allFilePaths []string
		for _, fileHeader := range r.MultipartForm.File["file"] {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Failed to open uploaded file %s: %v", fileHeader.Filename, err)
				continue
			}

			tempFile, err := os.CreateTemp("", "dat-*.tmp")
			if err != nil {
				log.Printf("Failed to create temp file: %v", err)
				file.Close()
				continue
			}
			_, err = io.Copy(tempFile, file)
			file.Close()
			if err != nil {
				log.Printf("Failed to copy to temp file: %v", err)
				tempFile.Close()
				os.Remove(tempFile.Name())
				continue
			}
			tempFile.Seek(0, 0)
			scanner := bufio.NewScanner(tempFile)
			var destDir string
			var newBaseName string
			if scanner.Scan() {
				firstLine := scanner.Text()
				if strings.HasPrefix(firstLine, "S") && len(firstLine) >= 39 {
					timestampStr := firstLine[27:39]
					yy, mm, dd, h, m, s := timestampStr[0:2], timestampStr[2:4], timestampStr[4:6], timestampStr[6:8], timestampStr[8:10], timestampStr[10:12]
					newBaseName = fmt.Sprintf("20%s%s%s_%s%s%s", yy, mm, dd, h, m, s)
				}
			}
			if newBaseName != "" {
				destDir = filepath.Join("download", "DAT")
			} else {
				destDir = filepath.Join("download", "DAT", "unorganized")
				newBaseName = time.Now().Format("20060102150405")
			}
			os.MkdirAll(destDir, 0755)
			destPath := filepath.Join(destDir, newBaseName+filepath.Ext(fileHeader.Filename))

			if err := os.Rename(tempFile.Name(), destPath); err != nil {
				tempFile.Seek(0, 0)
				destFile, createErr := os.Create(destPath)
				if createErr != nil {
					log.Printf("Failed to create destination file for copying: %v", createErr)
					tempFile.Close()
					os.Remove(tempFile.Name())
					continue
				}
				_, copyErr := io.Copy(destFile, tempFile)
				destFile.Close()
				tempFile.Close()
				os.Remove(tempFile.Name())
				if copyErr != nil {
					log.Printf("Failed to copy temp file to destination: %v", copyErr)
					os.Remove(destPath)
					continue
				}
			}

			log.Printf("Successfully saved and organized file to: %s", destPath)
			allFilePaths = append(allFilePaths, destPath)
		}

		var allProcessedRecords []model.TransactionRecord
		for _, path := range allFilePaths {
			processed, err := ProcessDatFile(conn, path)
			if err != nil {
				log.Printf("Failed to process DAT file %s: %v", path, err)
				continue
			}
			allProcessedRecords = append(allProcessedRecords, processed...)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("Parsed and processed %d DAT files successfully.", len(allFilePaths)),
			"records": allProcessedRecords,
		})
	}
}

// ProcessDatFile は単一のDATファイルを解析し、内容をデータベースに登録します。
func ProcessDatFile(conn *sql.DB, filePath string) ([]model.TransactionRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open organized file %s: %w", filePath, err)
	}
	defer file.Close()

	parsed, err := parsers.ParseDat(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	filteredRecords := removeDatDuplicates(parsed)
	if len(filteredRecords) == 0 {
		return []model.TransactionRecord{}, nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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

	mastersMap, err := db.GetProductMastersByCodesMap(tx, keyList)
	if err != nil {
		return nil, fmt.Errorf("failed to pre-fetch product masters: %w", err)
	}

	jcshmsMap, err := db.GetJcshmsByCodesMap(tx, janList)
	if err != nil {
		return nil, fmt.Errorf("failed to pre-fetch JCSHMS data: %w", err)
	}

	stmt, err := tx.Prepare(insertTransactionQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	var finalRecords []model.TransactionRecord
	for _, rec := range filteredRecords {
		ar := model.TransactionRecord{
			TransactionDate: rec.Date, ClientCode: rec.ClientCode, ReceiptNumber: rec.ReceiptNumber,
			LineNumber: rec.LineNumber, Flag: rec.Flag, JanCode: rec.JanCode,
			ProductName: rec.ProductName, DatQuantity: rec.DatQuantity, UnitPrice: rec.UnitPrice,
			Subtotal: rec.Subtotal, ExpiryDate: rec.ExpiryDate, LotNumber: rec.LotNumber,
		}

		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			return nil, fmt.Errorf("mastermanager failed for jan %s: %w", rec.JanCode, err)
		}

		if master.YjPackUnitQty > 0 {
			ar.YjQuantity = ar.DatQuantity * master.YjPackUnitQty
		}
		if master.JanPackUnitQty > 0 {
			ar.JanQuantity = ar.DatQuantity * master.JanPackUnitQty
		}

		mappers.MapProductMasterToTransaction(&ar, master)
		ar.ProcessFlagMA = "COMPLETE"

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
			return nil, fmt.Errorf("failed to insert record for JAN %s: %w", ar.JanCode, err)
		}
		finalRecords = append(finalRecords, ar)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("transaction commit error (final): %w", err)
	}

	// (backorder reconciliation logic is omitted as in tkr)

	return finalRecords, nil
}

// removeDatDuplicates はDATレコードから重複を除外します。
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
