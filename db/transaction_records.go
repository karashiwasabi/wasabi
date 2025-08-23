// C:\Dev\WASABI\db\transaction_records.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"wasabi/mappers"
	"wasabi/model"
)

// ▼▼▼ [修正点] カラム一覧から processing_status を削除 ▼▼▼
const TransactionColumns = `
    id, transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma`

// ▲▲▲ 修正ここまで ▲▲▲

// ScanTransactionRecord maps a database row to a TransactionRecord struct.
func ScanTransactionRecord(row interface{ Scan(...interface{}) error }) (*model.TransactionRecord, error) {
	var r model.TransactionRecord
	// ▼▼▼ [修正点] スキャン対象から &r.ProcessingStatus を削除 ▼▼▼
	err := row.Scan(
		&r.ID, &r.TransactionDate, &r.ClientCode, &r.ReceiptNumber, &r.LineNumber, &r.Flag,
		&r.JanCode, &r.YjCode, &r.ProductName, &r.KanaName, &r.UsageClassification, &r.PackageForm, &r.PackageSpec, &r.MakerName,
		&r.DatQuantity, &r.JanPackInnerQty, &r.JanQuantity, &r.JanPackUnitQty, &r.JanUnitName, &r.JanUnitCode,
		&r.YjQuantity, &r.YjPackUnitQty, &r.YjUnitName, &r.UnitPrice, &r.PurchasePrice, &r.SupplierWholesale,
		&r.Subtotal, &r.TaxAmount, &r.TaxRate, &r.ExpiryDate, &r.LotNumber, &r.FlagPoison,
		&r.FlagDeleterious, &r.FlagNarcotic, &r.FlagPsychotropic, &r.FlagStimulant,
		&r.FlagStimulantRaw, &r.ProcessFlagMA,
	)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// PersistTransactionRecordsInTx inserts or replaces a slice of transaction records within a transaction.
func PersistTransactionRecordsInTx(tx *sql.Tx, records []model.TransactionRecord) error {
	// ▼▼▼ [修正点] INSERT文から processing_status と対応するプレースホルダを削除 ▼▼▼
	const q = `
INSERT OR REPLACE INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	// ▲▲▲ 修正ここまで ▲▲▲

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for transaction_records: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		// ▼▼▼ [修正点] Execの引数から rec.ProcessingStatus を削除 ▼▼▼
		_, err = stmt.Exec(
			rec.TransactionDate, rec.ClientCode, rec.ReceiptNumber, rec.LineNumber, rec.Flag,
			rec.JanCode, rec.YjCode, rec.ProductName, rec.KanaName, rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.MakerName,
			rec.DatQuantity, rec.JanPackInnerQty, rec.JanQuantity,
			rec.JanPackUnitQty,
			rec.JanUnitName, rec.JanUnitCode,
			rec.YjQuantity, rec.YjPackUnitQty, rec.YjUnitName, rec.UnitPrice, rec.PurchasePrice, rec.SupplierWholesale,
			rec.Subtotal, rec.TaxAmount, rec.TaxRate, rec.ExpiryDate, rec.LotNumber, rec.FlagPoison,
			rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic, rec.FlagStimulant,
			rec.FlagStimulantRaw, rec.ProcessFlagMA,
		)
		// ▲▲▲ 修正ここまで ▲▲▲
		if err != nil {
			log.Printf("FAILED to insert into transaction_records: JAN=%s, Error: %v", rec.JanCode, err)
			return fmt.Errorf("failed to exec statement for transaction_records (JAN: %s): %w", rec.JanCode, err)
		}
	}
	return nil
}

// PersistTransactionRecordsWithMasterMappingInTx は、マスター情報を自動で付与しながら複数の取引レコードを保存します。
func PersistTransactionRecordsWithMasterMappingInTx(tx *sql.Tx, records []model.TransactionRecord) error {
	// 処理対象の製品コードを収集
	var productCodes []string
	codeMap := make(map[string]struct{})
	for _, rec := range records {
		if _, exists := codeMap[rec.JanCode]; !exists {
			productCodes = append(productCodes, rec.JanCode)
			codeMap[rec.JanCode] = struct{}{}
		}
	}

	// マスター情報を一括取得
	masters, err := GetProductMastersByCodesMap(tx, productCodes)
	if err != nil {
		return fmt.Errorf("failed to pre-fetch masters for persisting records: %w", err)
	}

	// 既存のINSERT文を再利用
	const q = `
INSERT OR REPLACE INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for transaction_records: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		// マスター情報をマッピング
		if master, ok := masters[rec.JanCode]; ok {
			mappers.MapProductMasterToTransaction(&rec, master)
		}

		// DBへ保存
		_, err = stmt.Exec(
			rec.TransactionDate, rec.ClientCode, rec.ReceiptNumber, rec.LineNumber, rec.Flag,
			rec.JanCode, rec.YjCode, rec.ProductName, rec.KanaName, rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.MakerName,
			rec.DatQuantity, rec.JanPackInnerQty, rec.JanQuantity,
			rec.JanPackUnitQty,
			rec.JanUnitName, rec.JanCode, // jan_unit_code にはJANコード自体を一時的に入れるなど、必要に応じて修正
			rec.YjQuantity, rec.YjPackUnitQty, rec.YjUnitName, rec.UnitPrice, rec.PurchasePrice, rec.SupplierWholesale,
			rec.Subtotal, rec.TaxAmount, rec.TaxRate, rec.ExpiryDate, rec.LotNumber, rec.FlagPoison,
			rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic, rec.FlagStimulant,
			rec.FlagStimulantRaw, rec.ProcessFlagMA,
		)
		if err != nil {
			log.Printf("FAILED to insert into transaction_records: JAN=%s, Error: %v", rec.JanCode, err)
			return fmt.Errorf("failed to exec statement for transaction_records (JAN: %s): %w", rec.JanCode, err)
		}
	}
	return nil
}

// GetReceiptNumbersByDate returns a list of unique receipt numbers for a given date.
func GetReceiptNumbersByDate(conn *sql.DB, date string) ([]string, error) {
	const q = `SELECT DISTINCT receipt_number FROM transaction_records WHERE transaction_date = ? ORDER BY receipt_number`
	rows, err := conn.Query(q, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt numbers by date: %w", err)
	}
	defer rows.Close()

	var numbers []string
	for rows.Next() {
		var number string
		if err = rows.Scan(&number); err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

// GetTransactionsByReceiptNumber returns all transactions for a given receipt number.
func GetTransactionsByReceiptNumber(conn *sql.DB, receiptNumber string) ([]model.TransactionRecord, error) {
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE receipt_number = ? ORDER BY line_number`
	rows, err := conn.Query(q, receiptNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by receipt number: %w", err)
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}
	return records, nil
}

// GetProvisionalTransactions retrieves all records marked as 'provisional'.
func GetProvisionalTransactions(tx *sql.Tx) ([]model.TransactionRecord, error) {
	// ▼▼▼ [修正点] WHERE句の条件を process_flag_ma に変更 ▼▼▼
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE process_flag_ma = 'PROVISIONAL'`
	// ▲▲▲ 修正ここまで ▲▲▲
	rows, err := tx.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to get provisional transactions: %w", err)
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}
	return records, nil
}

// UpdateFullTransactionInTx updates an existing transaction record with enriched master data.
func UpdateFullTransactionInTx(tx *sql.Tx, record *model.TransactionRecord) error {
	// ▼▼▼ [修正点] UPDATE文から processing_status を削除 ▼▼▼
	const q = `
		UPDATE transaction_records SET
			jan_code = ?, yj_code = ?, product_name = ?, kana_name = ?, usage_classification = ?, package_form = ?, 
			package_spec = ?, maker_name = ?, jan_pack_inner_qty = ?, jan_pack_unit_qty = ?, 
			jan_unit_name = ?, jan_unit_code = ?, yj_pack_unit_qty = ?, yj_unit_name = ?,
			unit_price = ?, purchase_price = ?, supplier_wholesale = ?,
			flag_poison = ?, flag_deleterious = ?, flag_narcotic = ?, flag_psychotropic = ?,
			flag_stimulant = ?, flag_stimulant_raw = ?,
			process_flag_ma = ?
		WHERE id = ?`
	// ▲▲▲ 修正ここまで ▲▲▲

	// ▼▼▼ [修正点] Execの引数から record.ProcessingStatus を削除 ▼▼▼
	_, err := tx.Exec(q,
		record.JanCode, record.YjCode, record.ProductName, record.KanaName, record.UsageClassification, record.PackageForm,
		record.PackageSpec, record.MakerName, record.JanPackInnerQty, record.JanPackUnitQty,
		record.JanUnitName, record.JanUnitCode, record.YjPackUnitQty, record.YjUnitName,
		record.UnitPrice, record.PurchasePrice, record.SupplierWholesale,
		record.FlagPoison, record.FlagDeleterious, record.FlagNarcotic, record.FlagPsychotropic,
		record.FlagStimulant, record.FlagStimulantRaw,
		record.ProcessFlagMA,
		record.ID,
	)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return fmt.Errorf("failed to update transaction ID %d: %w", record.ID, err)
	}
	return nil
}

// DeleteTransactionsByReceiptNumberInTx deletes all transaction records with a given receipt number.
func DeleteTransactionsByReceiptNumberInTx(tx *sql.Tx, receiptNumber string) error {
	const q = `DELETE FROM transaction_records WHERE receipt_number = ?`
	_, err := tx.Exec(q, receiptNumber)
	if err != nil {
		return fmt.Errorf("failed to delete transactions for receipt %s: %w", receiptNumber, err)
	}
	return nil
}

// DeleteTransactionsByFlagAndDate deletes transactions with a specific flag on a specific date.
func DeleteTransactionsByFlagAndDate(tx *sql.Tx, flag int, date string) error {
	const q = `DELETE FROM transaction_records WHERE flag = ? AND transaction_date = ?`
	_, err := tx.Exec(q, flag, date)
	if err != nil {
		return fmt.Errorf("failed to delete transactions for flag %d, date %s: %w", flag, date, err)
	}
	return nil
}

// DeleteTransactionsByFlagAndDateAndCodes は、フラグと日付に加えて製品コードでも絞って削除します。
func DeleteTransactionsByFlagAndDateAndCodes(tx *sql.Tx, flag int, date string, productCodes []string) error {
	if len(productCodes) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	q := fmt.Sprintf(`DELETE FROM transaction_records WHERE flag = ? AND transaction_date = ? AND jan_code IN (%s)`, placeholders)

	args := make([]interface{}, 0, len(productCodes)+2)
	args = append(args, flag, date)
	for _, code := range productCodes {
		args = append(args, code)
	}

	_, err := tx.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("failed to delete transactions by flag, date, and codes: %w", err)
	}
	return nil
}

// DeleteUsageTransactionsInDateRange deletes usage transactions (flag=3) in a date range.
func DeleteUsageTransactionsInDateRange(tx *sql.Tx, minDate, maxDate string) error {
	const q = `DELETE FROM transaction_records WHERE flag = 3 AND transaction_date BETWEEN ? AND ?`
	_, err := tx.Exec(q, minDate, maxDate)
	if err != nil {
		return fmt.Errorf("failed to delete usage transactions: %w", err)
	}
	return nil
}

// DeleteZeroFillInventoryTransactionsは、指定された日付とJANコードリストに一致する
// ゼロ埋め用の棚卸レコード（LineNumberが'Z'で始まるもの）を削除します。
func DeleteZeroFillInventoryTransactions(tx *sql.Tx, date string, janCodes []string) error {
	if len(janCodes) == 0 {
		return nil // 削除対象がなければ何もしない
	}

	// SQLインジェクションを防ぐため、プレースホルダ(?)を動的に生成
	placeholders := strings.Repeat("?,", len(janCodes)-1) + "?"

	// 'Z'で始まるゼロ埋めレコードのみを対象とするDELETE文
	q := fmt.Sprintf(`
		DELETE FROM transaction_records 
		WHERE flag = 0 
		  AND transaction_date = ? 
		  AND line_number LIKE 'Z%%' 
		  AND jan_code IN (%s)`, placeholders)

	// interface{}のスライスに引数をまとめる
	args := make([]interface{}, 0, len(janCodes)+1)
	args = append(args, date)
	for _, jan := range janCodes {
		args = append(args, jan)
	}

	_, err := tx.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("failed to delete zero-fill inventory transactions for date %s: %w", date, err)
	}
	return nil
}

// ClearAllTransactions はtransaction_recordsテーブルの全レコードを削除します。
func ClearAllTransactions(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction for clearing transactions: %w", err)
	}
	defer tx.Rollback()

	// 全レコードを削除
	if _, err := tx.Exec(`DELETE FROM transaction_records`); err != nil {
		return fmt.Errorf("failed to execute delete from transaction_records: %w", err)
	}

	// SQLiteの自動採番シーケンスをリセット
	// これにより、次に登録されるデータのIDが1から始まります。
	if _, err := tx.Exec(`UPDATE sqlite_sequence SET seq = 0 WHERE name = 'transaction_records'`); err != nil {
		// テーブルが空の場合、sqlite_sequenceにエントリがないことがあるため、エラーを無視しても問題ない
		log.Printf("Could not reset sequence for transaction_records (this is normal if table was empty): %v", err)
	}

	return tx.Commit()
}

// ▼▼▼ [修正点] 以下の関数をファイル末尾に追加 ▼▼▼
// GetLastInventoryDateMap は全製品の最終棚卸日をマップ形式で取得します。
func GetLastInventoryDateMap(conn *sql.DB) (map[string]string, error) {
	rows, err := conn.Query(`
		SELECT jan_code, MAX(transaction_date) 
		FROM transaction_records 
		WHERE flag = 0 AND jan_code != ''
		GROUP BY jan_code
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get last inventory dates: %w", err)
	}
	defer rows.Close()

	dateMap := make(map[string]string)
	for rows.Next() {
		var janCode string
		var lastDate sql.NullString
		if err := rows.Scan(&janCode, &lastDate); err != nil {
			return nil, err
		}
		if lastDate.Valid {
			dateMap[janCode] = lastDate.String
		}
	}
	return dateMap, nil
}

// ▲▲▲ 修正ここまで ▲▲▲
