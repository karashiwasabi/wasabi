package usage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"wasabi/config"
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

// UploadUsageHandler は自動または手動でのUSAGEファイルアップロードを処理します。
func UploadUsageHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var file io.Reader
		var err error

		if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			log.Println("Processing manual USAGE file upload...")
			var f multipart.File
			f, _, err = r.FormFile("file")
			if err != nil {
				http.Error(w, "ファイルの取得に失敗しました: "+err.Error(), http.StatusBadRequest)
				return
			}
			defer f.Close()
			file = f
		} else {
			log.Println("Processing automatic USAGE file import...")
			cfg, cfgErr := config.LoadConfig()
			if cfgErr != nil {
				http.Error(w, "設定ファイルの読み込みに失敗: "+cfgErr.Error(), http.StatusInternalServerError)
				return
			}
			if cfg.UsageFolderPath == "" {
				http.Error(w, "USAGEファイル取込パスが設定されていません。", http.StatusBadRequest)
				return
			}

			rawPath := cfg.UsageFolderPath
			// ▼▼▼【ここが修正箇所】パスの前後の空白と " を自動的に削除 ▼▼▼
			unquotedPath := strings.Trim(strings.TrimSpace(rawPath), "\"")
			// ▲▲▲【修正ここまで】▲▲▲

			filePath := strings.ReplaceAll(unquotedPath, "\\", "/")

			log.Printf("Opening specified USAGE file: %s", filePath)
			f, fErr := os.Open(filePath)
			if fErr != nil {
				displayError := fmt.Sprintf("設定されたパスのファイルを開けませんでした。\nパス: %s\nエラー: %v", filePath, fErr)
				http.Error(w, displayError, http.StatusInternalServerError)
				return
			}
			defer f.Close()
			file = f
		}

		processedRecords, procErr := processUsageFile(conn, file)
		if procErr != nil {
			http.Error(w, procErr.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"records": processedRecords,
		})
	}
}

// processUsageFile はファイルストリームから処方データを解析しDBに登録する共通関数です。
func processUsageFile(conn *sql.DB, file io.Reader) ([]model.TransactionRecord, error) {
	parsed, err := parsers.ParseUsage(file)
	if err != nil {
		return nil, fmt.Errorf("USAGEファイルの解析に失敗しました: %w", err)
	}

	var originalJournalMode string
	conn.QueryRow("PRAGMA journal_mode").Scan(&originalJournalMode)
	conn.Exec("PRAGMA journal_mode = MEMORY;")
	conn.Exec("PRAGMA synchronous = OFF;")
	defer func() {
		conn.Exec("PRAGMA synchronous = FULL;")
		conn.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", originalJournalMode))
	}()

	filtered := removeUsageDuplicates(parsed)
	if len(filtered) == 0 {
		return []model.TransactionRecord{}, nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
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
		return nil, fmt.Errorf("既存の処方データ削除に失敗: %w", err)
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
		return nil, err
	}
	jcshmsMap, err := db.GetJcshmsByCodesMap(tx, janList)
	if err != nil {
		return nil, err
	}

	stmt, err := tx.Prepare(insertTransactionQuery)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var finalRecords []model.TransactionRecord
	for _, rec := range filtered {
		ar := model.TransactionRecord{
			TransactionDate: rec.Date, Flag: 3, JanCode: rec.JanCode,
			YjCode: rec.YjCode, ProductName: rec.ProductName,
			YjQuantity: rec.YjQuantity, YjUnitName: rec.YjUnitName,
		}
		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			return nil, err
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
			return nil, fmt.Errorf("failed to insert record for JAN %s: %w", ar.JanCode, err)
		}

		finalRecords = append(finalRecords, ar)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return finalRecords, nil
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
