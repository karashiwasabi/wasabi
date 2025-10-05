// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\transaction_records.go
package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"wasabi/mappers"
	"wasabi/model"
)

const TransactionColumns = `
    id, transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
    subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma`

func ScanTransactionRecord(row interface{ Scan(...interface{}) error }) (*model.TransactionRecord, error) {
	var r model.TransactionRecord
	err := row.Scan(
		&r.ID, &r.TransactionDate, &r.ClientCode, &r.ReceiptNumber, &r.LineNumber, &r.Flag,
		&r.JanCode, &r.YjCode, &r.ProductName, &r.KanaName, &r.UsageClassification, &r.PackageForm, &r.PackageSpec, &r.MakerName,
		&r.DatQuantity, &r.JanPackInnerQty, &r.JanQuantity, &r.JanPackUnitQty, &r.JanUnitName, &r.JanUnitCode,
		&r.YjQuantity, &r.YjPackUnitQty, &r.YjUnitName, &r.UnitPrice, &r.PurchasePrice, &r.SupplierWholesale,
		&r.Subtotal, &r.TaxAmount, &r.TaxRate, &r.ExpiryDate, &r.LotNumber, &r.FlagPoison,
		&r.FlagDeleterious, &r.FlagNarcotic, &r.FlagPsychotropic, &r.FlagStimulant,
		&r.FlagStimulantRaw, &r.ProcessFlagMA,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func PersistTransactionRecordsInTx(tx *sql.Tx, records []model.TransactionRecord) error {
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
		if err != nil {
			log.Printf("FAILED to insert into transaction_records: JAN=%s, Error: %v", rec.JanCode, err)
			return fmt.Errorf("failed to exec statement for transaction_records (JAN: %s): %w", rec.JanCode, err)
		}
	}
	return nil
}

func PersistTransactionRecordsWithMasterMappingInTx(tx *sql.Tx, records []model.TransactionRecord) error {
	var productCodes []string
	codeMap := make(map[string]struct{})
	for _, rec := range records {
		if _, exists := codeMap[rec.JanCode]; !exists {
			productCodes = append(productCodes, rec.JanCode)
			codeMap[rec.JanCode] = struct{}{}
		}
	}

	masters, err := GetProductMastersByCodesMap(tx, productCodes)
	if err != nil {
		return fmt.Errorf("failed to pre-fetch masters for persisting records: %w", err)
	}

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
		if master, ok := masters[rec.JanCode]; ok {
			mappers.MapProductMasterToTransaction(&rec, master)
		}

		_, err = stmt.Exec(
			rec.TransactionDate, rec.ClientCode, rec.ReceiptNumber, rec.LineNumber, rec.Flag,
			rec.JanCode, rec.YjCode, rec.ProductName, rec.KanaName, rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.MakerName,
			rec.DatQuantity, rec.JanPackInnerQty, rec.JanQuantity,
			rec.JanPackUnitQty,
			rec.JanUnitName, rec.JanCode,
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

func GetReceiptNumbersByDate(conn *sql.DB, date string) ([]string, error) {
	const q = `SELECT DISTINCT receipt_number FROM transaction_records 
               WHERE transaction_date = ? AND receipt_number LIKE 'io%' 
               ORDER BY receipt_number`
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

func DeleteTransactionsByReceiptNumberInTx(tx *sql.Tx, receiptNumber string) error {
	const q = `DELETE FROM transaction_records WHERE receipt_number = ?`
	_, err := tx.Exec(q, receiptNumber)
	if err != nil {
		return fmt.Errorf("failed to delete transactions for receipt %s: %w", receiptNumber, err)
	}
	return nil
}

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

func GetInventoryTransactionsByDate(conn *sql.DB, date string) ([]model.TransactionRecord, error) {
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE flag = 0 AND transaction_date = ? ORDER BY product_name`
	rows, err := conn.Query(q, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory transactions by date: %w", err)
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

func DeleteTransactionByIDInTx(tx *sql.Tx, id int) error {
	const q = `DELETE FROM transaction_records WHERE id = ?`
	res, err := tx.Exec(q, id)
	if err != nil {
		return fmt.Errorf("failed to delete transaction with id %d: %w", id, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected for id %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no transaction found to delete with id %d", id)
	}
	return nil
}

func GetAllTransactionsForProductAfterDate(conn *sql.DB, janCode string, date string) ([]model.TransactionRecord, error) {
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records 
          WHERE jan_code = ? AND transaction_date > ? AND flag != 0
          ORDER BY transaction_date, id`

	rows, err := conn.Query(q, janCode, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions after date for %s: %w", janCode, err)
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

func GetTransactionsForProductInDateRange(conn *sql.DB, janCode string, startDate string, endDate string) ([]model.TransactionRecord, error) {
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records 
          WHERE jan_code = ? AND transaction_date BETWEEN ? AND ?
          ORDER BY transaction_date DESC, id DESC`

	rows, err := conn.Query(q, janCode, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions in date range for %s: %w", janCode, err)
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

func DeleteTransactionsByFlagAndDate(tx *sql.Tx, flag int, date string) error {
	const q = `DELETE FROM transaction_records WHERE flag = ? AND transaction_date = ?`
	_, err := tx.Exec(q, flag, date)
	if err != nil {
		return fmt.Errorf("failed to delete transactions for flag %d, date %s: %w", flag, date, err)
	}
	return nil
}

func DeleteZeroFillInventoryTransactions(tx *sql.Tx, date string, janCodes []string) error {
	if len(janCodes) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(janCodes)-1) + "?"
	q := fmt.Sprintf(`
        DELETE FROM transaction_records 
        WHERE flag = 0 
          AND transaction_date = ? 
          AND line_number LIKE 'Z%%' 
          AND jan_code IN (%s)`, placeholders)

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

func UpdateFullTransactionInTx(tx *sql.Tx, record *model.TransactionRecord) error {
	const q = `
        UPDATE transaction_records SET
            jan_code = ?, yj_code = ?, product_name = ?, kana_name = ?, usage_classification = ?, package_form = ?, 
            package_spec = ?, maker_name = ?, jan_pack_inner_qty = ?, jan_pack_unit_qty = ?, 
            jan_unit_name = ?, jan_unit_code = ?, yj_pack_unit_qty = ?, yj_unit_name = ?,
            unit_price = ?, purchase_price = ?, supplier_wholesale = ?,
            flag_poison = ?, flag_deleterious = ?, flag_narcotic = ?, flag_psychotropic = ?,
            flag_stimulant = ?, flag_stimulant_raw = ?,
            yj_quantity = ?, subtotal = ?,
            process_flag_ma = ?
        WHERE id = ?`

	_, err := tx.Exec(q,
		record.JanCode, record.YjCode, record.ProductName, record.KanaName, record.UsageClassification, record.PackageForm,
		record.PackageSpec, record.MakerName, record.JanPackInnerQty, record.JanPackUnitQty,
		record.JanUnitName, record.JanUnitCode, record.YjPackUnitQty, record.YjUnitName,
		record.UnitPrice, record.PurchasePrice, record.SupplierWholesale,
		record.FlagPoison, record.FlagDeleterious, record.FlagNarcotic, record.FlagPsychotropic,
		record.FlagStimulant, record.FlagStimulantRaw,
		record.YjQuantity, record.Subtotal,
		record.ProcessFlagMA,
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update transaction ID %d: %w", record.ID, err)
	}
	return nil
}

func ClearAllTransactions(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction for clearing transactions: %w",
			err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM transaction_records`); err != nil {
		return fmt.Errorf("failed to execute delete from transaction_records: %w", err)
	}

	if _, err := tx.Exec(`UPDATE sqlite_sequence SET seq = 0 WHERE name = 'transaction_records'`); err != nil {
		log.Printf("Could not reset sequence for transaction_records (this is normal if table was empty): %v", err)
	}

	return tx.Commit()
}

func DeleteUsageTransactionsInDateRange(tx *sql.Tx, minDate, maxDate string) error {
	const q = `DELETE FROM transaction_records WHERE flag = 3 AND transaction_date BETWEEN ? AND ?`
	_, err := tx.Exec(q, minDate, maxDate)
	if err != nil {
		return fmt.Errorf("failed to delete usage transactions: %w", err)
	}
	return nil
}

func GetLastInventoryDate(tx *sql.Tx, janCode string) string {
	var lastDate sql.NullString
	tx.QueryRow("SELECT MAX(transaction_date) FROM transaction_records WHERE jan_code = ? AND flag = 0", janCode).Scan(&lastDate)
	if lastDate.Valid {
		return lastDate.String
	}
	return ""
}
