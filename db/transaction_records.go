package db

import (
	"database/sql"
	"fmt"
	"log"
	"wasabi/model"
)

// TransactionColumns is a reusable list of all columns in the transaction_records table.
const TransactionColumns = `
    id, transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma, processing_status`

// ScanTransactionRecord maps a database row to a TransactionRecord struct.
func ScanTransactionRecord(row interface{ Scan(...interface{}) error }) (*model.TransactionRecord, error) {
	var r model.TransactionRecord
	err := row.Scan(
		&r.ID, &r.TransactionDate, &r.ClientCode, &r.ReceiptNumber, &r.LineNumber, &r.Flag,
		&r.JanCode, &r.YjCode, &r.ProductName, &r.KanaName, &r.UsageClassification, &r.PackageForm, &r.PackageSpec, &r.MakerName,
		&r.DatQuantity, &r.JanPackInnerQty, &r.JanQuantity, &r.JanPackUnitQty, &r.JanUnitName, &r.JanUnitCode,
		&r.YjQuantity, &r.YjPackUnitQty, &r.YjUnitName, &r.UnitPrice, &r.PurchasePrice, &r.SupplierWholesale,
		&r.Subtotal, &r.TaxAmount, &r.TaxRate, &r.ExpiryDate, &r.LotNumber, &r.FlagPoison,
		&r.FlagDeleterious, &r.FlagNarcotic, &r.FlagPsychotropic, &r.FlagStimulant,
		&r.FlagStimulantRaw, &r.ProcessFlagMA, &r.ProcessingStatus,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// PersistTransactionRecordsInTx inserts or replaces a slice of transaction records within a transaction.
func PersistTransactionRecordsInTx(tx *sql.Tx, records []model.TransactionRecord) error {
	const q = `
INSERT OR REPLACE INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma, processing_status
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for transaction_records: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		_, err := stmt.Exec(
			rec.TransactionDate, rec.ClientCode, rec.ReceiptNumber, rec.LineNumber, rec.Flag,
			rec.JanCode, rec.YjCode, rec.ProductName, rec.KanaName, rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.MakerName,
			rec.DatQuantity, rec.JanPackInnerQty, rec.JanQuantity,
			rec.JanPackUnitQty,
			rec.JanUnitName, rec.JanUnitCode,
			rec.YjQuantity, rec.YjPackUnitQty, rec.YjUnitName, rec.UnitPrice, rec.PurchasePrice, rec.SupplierWholesale,
			rec.Subtotal, rec.TaxAmount, rec.TaxRate, rec.ExpiryDate, rec.LotNumber, rec.FlagPoison,
			rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic, rec.FlagStimulant,
			rec.FlagStimulantRaw, rec.ProcessFlagMA, rec.ProcessingStatus,
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
		if err := rows.Scan(&number); err != nil {
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
func GetProvisionalTransactions(conn *sql.DB) ([]model.TransactionRecord, error) {
	q := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE processing_status = 'provisional'`
	rows, err := conn.Query(q)
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
	const q = `
		UPDATE transaction_records SET
			yj_code = ?, product_name = ?, kana_name = ?, usage_classification = ?, package_form = ?, 
			package_spec = ?, maker_name = ?, jan_pack_inner_qty = ?, jan_pack_unit_qty = ?, 
			jan_unit_name = ?, jan_unit_code = ?, yj_pack_unit_qty = ?, yj_unit_name = ?,
			unit_price = ?, purchase_price = ?, supplier_wholesale = ?,
			flag_poison = ?, flag_deleterious = ?, flag_narcotic = ?, flag_psychotropic = ?,
			flag_stimulant = ?, flag_stimulant_raw = ?,
			process_flag_ma = ?, processing_status = ?
		WHERE id = ?`

	_, err := tx.Exec(q,
		record.YjCode, record.ProductName, record.KanaName, record.UsageClassification, record.PackageForm,
		record.PackageSpec, record.MakerName, record.JanPackInnerQty, record.JanPackUnitQty,
		record.JanUnitName, record.JanUnitCode, record.YjPackUnitQty, record.YjUnitName,
		record.UnitPrice, record.PurchasePrice, record.SupplierWholesale,
		record.FlagPoison, record.FlagDeleterious, record.FlagNarcotic, record.FlagPsychotropic,
		record.FlagStimulant, record.FlagStimulantRaw,
		record.ProcessFlagMA, record.ProcessingStatus,
		record.ID,
	)
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
	if _, err := tx.Exec(q, flag, date); err != nil {
		return fmt.Errorf("failed to delete transactions for flag %d, date %s: %w", flag, date, err)
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
