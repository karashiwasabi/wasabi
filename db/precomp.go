package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"wasabi/mappers"
	"wasabi/model"
)

// PrecompRecordInput はフロントエンドから受け取る予製レコードの構造体です。
type PrecompRecordInput struct {
	ProductCode string  `json:"productCode"`
	JanQuantity float64 `json:"janQuantity"`
}

// PrecompRecordView は予製データを画面に表示するための構造体です。
type PrecompRecordView struct {
	model.TransactionRecord
	FormattedPackageSpec string `json:"formattedPackageSpec"`
	JanUnitName          string `json:"janUnitName"`
}

// UpsertPreCompoundingRecordsInTx は、特定の患者の予製レコードを安全に同期します。
func UpsertPreCompoundingRecordsInTx(tx *sql.Tx, patientNumber string, records []PrecompRecordInput) error {
	// If the frontend sends an empty list, it means all records for this patient should be deleted.
	if len(records) == 0 {
		if _, err := tx.Exec("DELETE FROM precomp_records WHERE client_code = ?", patientNumber); err != nil {
			return fmt.Errorf("failed to delete all precomp records for patient %s: %w", patientNumber, err)
		}
		return nil
	}

	// Step 1: Prepare a list of product codes from the payload for the `NOT IN` clause.
	productCodesInPayload := make([]interface{}, len(records)+1)
	placeholders := make([]string, len(records))
	productCodesInPayload[0] = patientNumber
	for i, rec := range records {
		placeholders[i] = "?"
		productCodesInPayload[i+1] = rec.ProductCode
	}

	// Step 2: Delete records from the DB that are no longer in the list from the frontend.
	deleteQuery := fmt.Sprintf("DELETE FROM precomp_records WHERE client_code = ? AND jan_code NOT IN (%s)", strings.Join(placeholders, ","))
	if _, err := tx.Exec(deleteQuery, productCodesInPayload...); err != nil {
		return fmt.Errorf("failed to delete removed precomp records for patient %s: %w", patientNumber, err)
	}

	// Step 3: Upsert the records from the payload.
	// Note: This requires a UNIQUE constraint on (client_code, jan_code) in the precomp_records table schema.
	var productCodes []string
	for _, rec := range records {
		productCodes = append(productCodes, rec.ProductCode)
	}
	mastersMap, err := GetProductMastersByCodesMap(tx, productCodes)
	if err != nil {
		return fmt.Errorf("failed to get product masters for precomp: %w", err)
	}

	const q = `INSERT INTO precomp_records (
		transaction_date, client_code, receipt_number, line_number, jan_code, yj_code, product_name, kana_name,
		usage_classification, package_form, package_spec, maker_name, jan_pack_inner_qty, jan_quantity,
		jan_pack_unit_qty, jan_unit_name, jan_unit_code, yj_quantity, yj_pack_unit_qty, yj_unit_name,
		purchase_price, supplier_wholesale, created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(client_code, jan_code) DO UPDATE SET
		jan_quantity = excluded.jan_quantity,
		yj_quantity = excluded.yj_quantity,
		created_at = excluded.created_at`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare precomp upsert statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	dateStr := now.Format("20060102")
	receiptNumber := fmt.Sprintf("PRECOMP-%s", patientNumber)

	for i, rec := range records {
		master, ok := mastersMap[rec.ProductCode]
		if !ok {
			continue
		}

		tr := model.TransactionRecord{
			TransactionDate: dateStr,
			ClientCode:      patientNumber,
			ReceiptNumber:   receiptNumber,
			LineNumber:      fmt.Sprintf("%d", i+1),
			JanCode:         rec.ProductCode,
			JanQuantity:     rec.JanQuantity,
		}
		if master.JanPackInnerQty > 0 {
			tr.YjQuantity = rec.JanQuantity * master.JanPackInnerQty
		}

		mappers.MapProductMasterToTransaction(&tr, master)

		_, err := stmt.Exec(
			tr.TransactionDate, tr.ClientCode, tr.ReceiptNumber, tr.LineNumber, tr.JanCode, tr.YjCode, tr.ProductName, tr.KanaName,
			tr.UsageClassification, tr.PackageForm, tr.PackageSpec, tr.MakerName, tr.JanPackInnerQty, tr.JanQuantity,
			tr.JanPackUnitQty, tr.JanUnitName, tr.JanUnitCode, tr.YjQuantity, tr.YjPackUnitQty, tr.YjUnitName,
			tr.PurchasePrice, tr.SupplierWholesale, now.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			return fmt.Errorf("failed to upsert precomp record for product %s: %w", rec.ProductCode, err)
		}
	}

	return nil
}

// GetPreCompoundingRecordsByPatient は、特定の患者の予製レコードをリストで取得します。
func GetPreCompoundingRecordsByPatient(conn *sql.DB, patientNumber string) ([]model.TransactionRecord, error) {
	const q = `SELECT
		id, transaction_date, client_code, receipt_number, line_number, 5 AS flag,
		jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
		0.0, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
		yj_quantity, yj_pack_unit_qty, yj_unit_name, 0.0, purchase_price, supplier_wholesale,
		0.0, 0.0, 0.0, '', '', 0, 0, 0, 0, 0, 0, ''
		FROM precomp_records WHERE client_code = ? ORDER BY id`

	rows, err := conn.Query(q, patientNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to query precomp records for patient %s: %w", patientNumber, err)
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan precomp record: %w", err)
		}
		records = append(records, *r)
	}
	return records, nil
}

// DeletePreCompoundingRecordsByPatient は、特定の患者の予製レコードをすべて削除します。
func DeletePreCompoundingRecordsByPatient(conn *sql.DB, patientNumber string) error {
	const q = `DELETE FROM precomp_records WHERE client_code = ?`
	if _, err := conn.Exec(q, patientNumber); err != nil {
		return fmt.Errorf("failed to delete precomp records for patient %s: %w", patientNumber, err)
	}
	return nil
}

// GetPreCompoundingTotals は、全製品の有効な予製合計数量をマップで返します。
func GetPreCompoundingTotals(conn *sql.DB) (map[string]float64, error) {
	const q = `SELECT jan_code, SUM(yj_quantity) FROM precomp_records GROUP BY jan_code`
	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query precomp totals: %w", err)
	}
	defer rows.Close()

	totals := make(map[string]float64)
	for rows.Next() {
		var productCode string
		var totalQuantity float64
		if err := rows.Scan(&productCode, &totalQuantity); err != nil {
			return nil, fmt.Errorf("failed to scan precomp total: %w", err)
		}
		totals[productCode] = totalQuantity
	}
	return totals, nil
}

// ▼▼▼ [修正点] PreCompoundingDetailView構造体を削除し、GetPreCompoundingDetailsByProductCodesを修正 ▼▼▼

// GetPreCompoundingDetailsByProductCodes は複数の製品コードに紐づく有効な予製レコードを全て取得します。
// 返り値の型を []model.TransactionRecord に変更
func GetPreCompoundingDetailsByProductCodes(conn *sql.DB, productCodes []string) ([]model.TransactionRecord, error) {
	if len(productCodes) == 0 {
		return []model.TransactionRecord{}, nil
	}

	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	// TransactionRecordとしてスキャンできるように、全ての必要カラムを取得するクエリに変更
	query := fmt.Sprintf(`
		SELECT
			p.id, p.transaction_date, p.client_code, p.receipt_number, p.line_number, 5 AS flag,
			p.jan_code, p.yj_code, p.product_name, p.kana_name, p.usage_classification, p.package_form, p.package_spec, p.maker_name,
			0.0, p.jan_pack_inner_qty, p.jan_quantity, p.jan_pack_unit_qty, p.jan_unit_name, p.jan_unit_code,
			p.yj_quantity, p.yj_pack_unit_qty, p.yj_unit_name, 0.0, p.purchase_price, p.supplier_wholesale,
			0.0, 0.0, 0.0, '', '', 0, 0, 0, 0, 0, 0, ''
		FROM precomp_records AS p
		WHERE p.jan_code IN (%s)
		ORDER BY p.created_at, p.client_code`, placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query precomp details by product codes: %w", err)
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		// 共通のスキャン関数を使用
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan precomp detail record: %w", err)
		}
		records = append(records, *r)
	}
	return records, nil
}

// ▲▲▲ 修正ここまで ▲▲▲

// ▼▼▼ [修正点] 以下の関数をファイル末尾に追加 ▼▼▼
// GetAllPreCompoundingRecords は、全患者の有効な予製レコードを全て取得します。
func GetAllPreCompoundingRecords(conn *sql.DB) ([]model.TransactionRecord, error) {
	// GetPreCompoundingRecordsByPatient から WHERE句を削除したクエリ
	const q = `SELECT
		id, transaction_date, client_code, receipt_number, line_number, 5 AS flag,
		jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
		0.0, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
		yj_quantity, yj_pack_unit_qty, yj_unit_name, 0.0, purchase_price, supplier_wholesale,
		0.0, 0.0, 0.0, '', '', 0, 0, 0, 0, 0, 0, ''
		FROM precomp_records 
		ORDER BY client_code, id` // 患者番号、ID順でソート

	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query all precomp records: %w", err)
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan precomp record: %w", err)
		}
		records = append(records, *r)
	}
	return records, nil
}

// ▲▲▲ 修正ここまで ▲▲▲
