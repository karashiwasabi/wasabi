// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\precomp.go

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
// NOTE: この構造体は現在直接使用されていませんが、将来的な拡張のために残されています。
type PrecompRecordView struct {
	model.TransactionRecord
	FormattedPackageSpec string `json:"formattedPackageSpec"`
	JanUnitName          string `json:"janUnitName"`
}

// ▼▼▼【ここから修正・追加】▼▼▼

/**
 * @brief 特定の患者の予製レコードをデータベースと安全に同期します。
 * @param tx SQLトランザクションオブジェクト
 * @param patientNumber 対象の患者番号
 * @param records フロントエンドから送信された最新の予製レコードのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * データベースの状態をフロントエンドの状態と完全に一致させます。
 * この際、ステータスは常に 'active' (有効) に設定されます。
 */
func UpsertPreCompoundingRecordsInTx(tx *sql.Tx, patientNumber string, records []PrecompRecordInput) error {
	if len(records) == 0 {
		if _, err := tx.Exec("DELETE FROM precomp_records WHERE client_code = ?", patientNumber); err != nil {
			return fmt.Errorf("failed to delete all precomp records for patient %s: %w", patientNumber, err)
		}
		return nil
	}

	productCodesInPayload := make([]interface{}, len(records)+1)
	placeholders := make([]string, len(records))
	productCodesInPayload[0] = patientNumber
	for i, rec := range records {
		placeholders[i] = "?"
		productCodesInPayload[i+1] = rec.ProductCode
	}

	deleteQuery := fmt.Sprintf("DELETE FROM precomp_records WHERE client_code = ? AND jan_code NOT IN (%s)", strings.Join(placeholders, ","))
	if _, err := tx.Exec(deleteQuery, productCodesInPayload...); err != nil {
		return fmt.Errorf("failed to delete removed precomp records for patient %s: %w", patientNumber, err)
	}

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
		purchase_price, supplier_wholesale, created_at, status
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(client_code, jan_code) DO UPDATE SET
		jan_quantity = excluded.jan_quantity,
		yj_quantity = excluded.yj_quantity,
		created_at = excluded.created_at,
		status = excluded.status`

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
			tr.PurchasePrice, tr.SupplierWholesale, now.Format("2006-01-02 15:04:05"), "active",
		)
		if err != nil {
			return fmt.Errorf("failed to upsert precomp record for product %s: %w", rec.ProductCode, err)
		}
	}

	return nil
}

/**
 * @brief 特定の患者の予製レコードをリストで取得します。
 * @param conn データベース接続
 * @param patientNumber 対象の患者番号
 * @return []model.TransactionRecord 予製レコードのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 予製レコードをTransactionRecordの形式で取得します。flagは5（予製）として固定値を設定します。
 */
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

/**
 * @brief 特定の患者の予製レコードをすべて削除します。
 * @param conn データベース接続
 * @param patientNumber 対象の患者番号
 * @return error 処理中にエラーが発生した場合
 */
func DeletePreCompoundingRecordsByPatient(conn *sql.DB, patientNumber string) error {
	const q = `DELETE FROM precomp_records WHERE client_code = ?`
	if _, err := conn.Exec(q, patientNumber); err != nil {
		return fmt.Errorf("failed to delete precomp records for patient %s: %w", patientNumber, err)
	}
	return nil
}

/**
 * @brief 全製品の有効な予製引当数量の合計をマップで返します。
 * @param conn データベース接続
 * @return map[string]float64 JANコードをキー、YJ単位での合計引当数量を値とするマップ
 * @return error 処理中にエラーが発生した場合
 * @details
 * この関数が返す値は、在庫元帳の計算において発注点の調整に使用されます。
 * statusが'active'のレコードのみを集計対象とします。
 */
func GetPreCompoundingTotals(conn *sql.DB) (map[string]float64, error) {
	const q = `SELECT jan_code, SUM(yj_quantity) FROM precomp_records WHERE status = 'active' GROUP BY jan_code`
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

/**
 * @brief 複数の製品コードに紐づく有効な予製レコードを全て取得します。
 * @param conn データベース接続
 * @param productCodes 取得対象の製品コードのスライス
 * @return []model.TransactionRecord 予製レコードのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 「棚卸調整」画面で、理論在庫と実在庫の差を分析する際の参考情報として使用されます。
 */
func GetPreCompoundingDetailsByProductCodes(conn *sql.DB, productCodes []string) ([]model.TransactionRecord, error) {
	if len(productCodes) == 0 {
		return []model.TransactionRecord{}, nil
	}

	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
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
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan precomp detail record: %w", err)
		}
		records = append(records, *r)
	}
	return records, nil
}

/**
 * @brief 全患者の有効な予製レコードを全て取得します。
 * @param conn データベース接続
 * @return []model.TransactionRecord 予製レコードのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 予製データの一括CSVエクスポート機能で使用されます。
 */
func GetAllPreCompoundingRecords(conn *sql.DB) ([]model.TransactionRecord, error) {
	const q = `SELECT
		id, transaction_date, client_code, receipt_number, line_number, 5 AS flag,
		jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
		0.0, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
		yj_quantity, yj_pack_unit_qty, yj_unit_name, 0.0, purchase_price, supplier_wholesale,
		0.0, 0.0, 0.0, '', '', 0, 0, 0, 0, 0, 0, ''
		FROM precomp_records 
		ORDER BY client_code, id`

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

/**
 * @brief 指定された患者の予製レコードを中断状態（inactive）にします。
 * @param tx SQLトランザクションオブジェクト
 * @param patientNumber 対象の患者番号
 * @return error 処理中にエラーが発生した場合
 */
func SuspendPreCompoundingRecordsByPatient(tx *sql.Tx, patientNumber string) error {
	const q = `UPDATE precomp_records SET status = 'inactive' WHERE client_code = ?`
	if _, err := tx.Exec(q, patientNumber); err != nil {
		return fmt.Errorf("failed to suspend precomp records for patient %s: %w", patientNumber, err)
	}
	return nil
}

/**
 * @brief 指定された患者の予製レコードを再開状態（active）にします。
 * @param tx SQLトランザクションオブジェクト
 * @param patientNumber 対象の患者番号
 * @return error 処理中にエラーが発生した場合
 */
func ResumePreCompoundingRecordsByPatient(tx *sql.Tx, patientNumber string) error {
	const q = `UPDATE precomp_records SET status = 'active' WHERE client_code = ?`
	if _, err := tx.Exec(q, patientNumber); err != nil {
		return fmt.Errorf("failed to resume precomp records for patient %s: %w", patientNumber, err)
	}
	return nil
}

/**
 * @brief 指定された患者の現在の予製ステータスを取得します。
 * @param conn データベース接続
 * @param patientNumber 対象の患者番号
 * @return string ステータス ('active', 'inactive', 'none')
 * @return error 処理中にエラーが発生した場合
 */
func GetPreCompoundingStatusByPatient(conn *sql.DB, patientNumber string) (string, error) {
	var status string
	const q = `SELECT status FROM precomp_records WHERE client_code = ? LIMIT 1`
	err := conn.QueryRow(q, patientNumber).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "none", nil // レコードが存在しない
		}
		return "", fmt.Errorf("failed to get precomp status for patient %s: %w", patientNumber, err)
	}
	return status, nil
}

// ▲▲▲【修正ここまで】▲▲▲
