package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"wasabi/model"
)

type PreCompoundingRecordView struct {
	model.PreCompoundingRecord
	ProductName     string  `json:"productName"`
	YjUnitName      string  `json:"yjUnitName"`
	JanPackInnerQty float64 `json:"janPackInnerQty"`
}

func GetPreCompoundingRecordsByPatient(conn *sql.DB, patientNumber string) ([]PreCompoundingRecordView, error) {
	const q = `
		SELECT
			pcr.id, pcr.patient_number, pcr.product_code, pcr.quantity, pcr.created_at,
			pm.product_name, pm.yj_unit_name, pm.jan_pack_inner_qty
		FROM pre_compounding_records pcr
		JOIN product_master pm ON pcr.product_code = pm.product_code
		WHERE pcr.patient_number = ? ORDER BY pcr.id`

	rows, err := conn.Query(q, patientNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to query pre-compounding records for patient %s: %w", patientNumber, err)
	}
	defer rows.Close()

	var records []PreCompoundingRecordView
	for rows.Next() {
		var r PreCompoundingRecordView
		if err := rows.Scan(
			&r.ID, &r.PatientNumber, &r.ProductCode, &r.Quantity, &r.CreatedAt,
			&r.ProductName, &r.YjUnitName, &r.JanPackInnerQty,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pre-compounding record: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}

func DeletePreCompoundingRecordsByPatient(conn *sql.DB, patientNumber string) error {
	const q = `DELETE FROM pre_compounding_records WHERE patient_number = ?`
	if _, err := conn.Exec(q, patientNumber); err != nil {
		return fmt.Errorf("failed to delete records for patient %s: %w", patientNumber, err)
	}
	return nil
}

func UpsertPreCompoundingRecordsInTx(tx *sql.Tx, patientNumber string, records []model.PreCompoundingRecord) error {
	if _, err := tx.Exec("DELETE FROM pre_compounding_records WHERE patient_number = ?", patientNumber); err != nil {
		return fmt.Errorf("failed to delete old records for patient %s: %w", patientNumber, err)
	}

	if len(records) == 0 {
		return nil
	}

	stmt, err := tx.Prepare("INSERT INTO pre_compounding_records (patient_number, product_code, quantity, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		_, err := stmt.Exec(patientNumber, rec.ProductCode, rec.Quantity, time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			return fmt.Errorf("failed to insert record for product %s: %w", rec.ProductCode, err)
		}
	}
	return nil
}

func GetPreCompoundingTotals(conn *sql.DB) (map[string]float64, error) {
	const q = `SELECT product_code, SUM(quantity) FROM pre_compounding_records GROUP BY product_code`
	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query pre-compounding totals: %w", err)
	}
	defer rows.Close()
	totals := make(map[string]float64)
	for rows.Next() {
		var productCode string
		var totalQuantity float64
		if err := rows.Scan(&productCode, &totalQuantity); err != nil {
			return nil, fmt.Errorf("failed to scan pre-compounding total: %w", err)
		}
		totals[productCode] = totalQuantity
	}
	return totals, nil
}

// ▼▼▼ 以下をファイル末尾に追加 ▼▼▼

// PreCompoundingDetailView は予製明細表示用の構造体です
type PreCompoundingDetailView struct {
	model.PreCompoundingRecord
	ProductName     string  `json:"productName"`
	YjUnitName      string  `json:"yjUnitName"`
	JanPackInnerQty float64 `json:"janPackInnerQty"`
}

// GetPreCompoundingRecordsByProductCodes は複数の製品コードに紐づく有効な予製レコードを全て取得します。
func GetPreCompoundingRecordsByProductCodes(conn *sql.DB, productCodes []string) ([]PreCompoundingDetailView, error) {
	if len(productCodes) == 0 {
		return []PreCompoundingDetailView{}, nil
	}

	// SQLインジェクションを防ぐため、プレースホルダを動的に生成
	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf(`
		SELECT
			pcr.id, pcr.patient_number, pcr.product_code, pcr.quantity, pcr.created_at,
			pm.product_name, pm.yj_unit_name, pm.jan_pack_inner_qty
		FROM pre_compounding_records AS pcr
		JOIN product_master AS pm ON pcr.product_code = pm.product_code
		WHERE pcr.product_code IN (%s)
		ORDER BY pcr.created_at, pcr.patient_number`, placeholders)

	// interface{}のスライスに引数をまとめる
	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pre-compounding records by product codes: %w", err)
	}
	defer rows.Close()

	var records []PreCompoundingDetailView
	for rows.Next() {
		var r PreCompoundingDetailView
		if err := rows.Scan(
			&r.ID, &r.PatientNumber, &r.ProductCode, &r.Quantity, &r.CreatedAt,
			&r.ProductName, &r.YjUnitName, &r.JanPackInnerQty,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pre-compounding detail record: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}
