// C:\Dev\WASABI\dat\handler.go

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

// UploadDatHandler はDATファイルのアップロードを処理します。
func UploadDatHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// (データベース設定の変更・復元ロジックは変更なし)
		var originalJournalMode string
		conn.QueryRow("PRAGMA journal_mode").Scan(&originalJournalMode)
		conn.Exec("PRAGMA journal_mode = MEMORY;")
		conn.Exec("PRAGMA synchronous = OFF;")
		defer func() {
			conn.Exec("PRAGMA synchronous = FULL;")
			conn.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", originalJournalMode))
			log.Println("Database settings restored for DAT handler.")
		}()

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()

		var allProcessedRecords []model.TransactionRecord
		var allFilePaths []string

		// ▼▼▼ [修正点] まず全てのファイルをディスクに保存する処理に専念させる ▼▼▼
		for _, fileHeader := range r.MultipartForm.File["file"] {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Failed to open uploaded file %s: %v", fileHeader.Filename, err)
				continue
			}

			// (一時ファイルへの保存、ファイル名の解析・整理ロジックは変更なし)
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
				log.Printf("Warning: Could not parse timestamp from %s. Saving to %s", fileHeader.Filename, destDir)
			}
			if err := os.MkdirAll(destDir, 0755); err != nil {
				log.Printf("Failed to create destination directory %s: %v", destDir, err)
				tempFile.Close()
				os.Remove(tempFile.Name())
				continue
			}
			ext := filepath.Ext(fileHeader.Filename)
			destPath := filepath.Join(destDir, newBaseName+ext)
			for i := 1; ; i++ {
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					break
				}
				destPath = filepath.Join(destDir, fmt.Sprintf("%s(%d)%s", newBaseName, i, ext))
			}
			tempFile.Close()

			// ▼▼▼ [修正点] ファイルハンドル解放のために短い待機時間を追加 ▼▼▼
			time.Sleep(200 * time.Millisecond)
			// ▲▲▲ 修正ここまで ▲▲▲

			if err := os.Rename(tempFile.Name(), destPath); err != nil {
				log.Printf("Failed to move temp file to %s: %v", destPath, err)
				os.Remove(tempFile.Name())
				continue
			}
			log.Printf("Successfully saved and organized file to: %s", destPath)
			allFilePaths = append(allFilePaths, destPath)
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		// ▼▼▼ [修正点] 保存したファイルパスをループし、新しい共通関数で処理する ▼▼▼
		for _, path := range allFilePaths {
			processed, err := ProcessDatFile(conn, path)
			if err != nil {
				// 1つのファイル処理でエラーが発生した場合でも、他のファイルは処理を試みる
				log.Printf("Failed to process DAT file %s: %v", path, err)
				continue
			}
			allProcessedRecords = append(allProcessedRecords, processed...)
		}

		if len(allProcessedRecords) == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "No new records to process, or failed to process all files.",
				"records": []model.TransactionRecord{},
			})
			return
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("Parsed and processed %d DAT files successfully.", len(allFilePaths)),
			"records": allProcessedRecords,
		})
	}
}

// ▼▼▼ [修正点] DATファイルの処理ロジックをこの新しい関数に集約 ▼▼▼
// ProcessDatFile は指定されたパスのDATファイルを解析し、DBに登録します。
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
		return []model.TransactionRecord{}, nil // 新規レコードがない場合は空スライスとnilエラーを返す
	}

	tx, err := conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// (マスターデータ取得、DBへのバッチインサート、発注残の消込処理は、元のハンドラからそのまま移動)
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
			return nil, fmt.Errorf("mastermanager failed for jan %s: %w", rec.JanCode, err)
		}
		if master.JanPackUnitQty > 0 {
			ar.JanQuantity = ar.DatQuantity * master.JanPackUnitQty
		}
		if master.YjPackUnitQty > 0 {
			ar.YjQuantity = ar.DatQuantity * master.YjPackUnitQty
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

		if (i+1)%batchSize == 0 && i < len(filteredRecords)-1 {
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("transaction commit error (batch): %w", err)
			}
			tx, err = conn.Begin()
			if err != nil {
				return nil, fmt.Errorf("failed to begin next transaction: %w", err)
			}
			stmt, err = tx.Prepare(insertTransactionQuery)
			if err != nil {
				return nil, fmt.Errorf("failed to re-prepare statement: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("transaction commit error (final): %w", err)
	}

	var deliveredItems []model.Backorder
	for _, rec := range finalRecords {
		if rec.Flag == 1 { // 納品フラグ
			deliveredItems = append(deliveredItems, model.Backorder{
				YjCode: rec.YjCode, PackageForm: rec.PackageForm, JanPackInnerQty: rec.JanPackInnerQty,
				YjUnitName: rec.YjUnitName, YjQuantity: rec.YjQuantity,
			})
		}
	}

	if len(deliveredItems) > 0 {
		if err := db.ReconcileBackorders(conn, deliveredItems); err != nil {
			log.Printf("WARN: Failed to reconcile backorders after DAT processing: %v", err)
		}
	}

	return finalRecords, nil
}

// ▲▲▲ 修正ここまで ▲▲▲

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
