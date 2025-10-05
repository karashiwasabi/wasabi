// C:\Users\wasab\OneDrive\デスクトップ\WASABI\WASABI\db\stock.go

package db

import (
	"database/sql"
	"fmt"
)

/**
 * @brief 指定された単一製品の現在の理論在庫を、棚卸を考慮して正確に計算します。
 */
func CalculateCurrentStockForProduct(executor DBTX, janCode string) (float64, error) {
	var latestInventoryDate sql.NullString
	err := executor.QueryRow(`
		SELECT MAX(transaction_date) FROM transaction_records
		WHERE jan_code = ? AND flag = 0`, janCode).Scan(&latestInventoryDate)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get latest inventory date for %s: %w", janCode, err)
	}

	var baseStock float64
	var netChangeQuery string
	var args []interface{}

	if latestInventoryDate.Valid && latestInventoryDate.String != "" {
		err := executor.QueryRow(`
			SELECT SUM(yj_quantity) FROM transaction_records
			WHERE jan_code = ? AND flag = 0 AND transaction_date = ?`,
			janCode, latestInventoryDate.String).Scan(&baseStock)
		if err != nil {
			return 0, fmt.Errorf("failed to sum inventory for %s on %s: %w", janCode, latestInventoryDate.String, err)
		}

		netChangeQuery = `
			SELECT
				SUM(CASE
					WHEN flag IN (1, 4, 11) THEN yj_quantity
					WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity
					ELSE 0
				END)
			FROM transaction_records
			WHERE jan_code = ? AND transaction_date > ?`
		args = []interface{}{janCode, latestInventoryDate.String}

	} else {
		baseStock = 0
		netChangeQuery = `
			SELECT
				SUM(CASE
					WHEN flag IN (1, 4, 11) THEN yj_quantity
					WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity
					ELSE 0
				END)
			FROM transaction_records
			WHERE jan_code = ?`
		args = []interface{}{janCode}
	}

	var nullNetChange sql.NullFloat64
	err = executor.QueryRow(netChangeQuery, args...).Scan(&nullNetChange)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to calculate net change for %s: %w", janCode, err)
	}
	netChange := nullNetChange.Float64

	return baseStock + netChange, nil
}

/**
 * @brief 全製品の現在庫を効率的に計算し、マップで返します。
 */
func GetAllCurrentStockMap(conn *sql.DB) (map[string]float64, error) {
	rows, err := conn.Query(`
		SELECT jan_code, transaction_date, flag, yj_quantity 
		FROM transaction_records 
		ORDER BY jan_code, transaction_date, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions for stock calculation: %w", err)
	}
	defer rows.Close()

	stockMap := make(map[string]float64)

	type txRecord struct {
		Date string
		Flag int
		Qty  float64
	}
	recordsByJan := make(map[string][]txRecord)

	for rows.Next() {
		var janCode, date string
		var flag int
		var qty float64
		if err := rows.Scan(&janCode, &date, &flag, &qty); err != nil {
			return nil, err
		}
		if janCode == "" {
			continue
		}
		recordsByJan[janCode] = append(recordsByJan[janCode], txRecord{Date: date, Flag: flag, Qty: qty})
	}

	for janCode, records := range recordsByJan {
		var latestInvDate string
		baseStock := 0.0

		invStocksOnDate := make(map[string]float64)
		for _, r := range records {
			if r.Flag == 0 {
				if r.Date > latestInvDate {
					latestInvDate = r.Date
				}
				invStocksOnDate[r.Date] += r.Qty
			}
		}
		if latestInvDate != "" {
			baseStock = invStocksOnDate[latestInvDate]
		}

		netChange := 0.0
		for _, r := range records {
			startDate := "00000000"
			if latestInvDate != "" {
				startDate = latestInvDate
			}

			if r.Date > startDate {
				switch r.Flag {
				case 1, 4, 11:
					netChange += r.Qty
				case 2, 3, 5, 12:
					netChange -= r.Qty
				}
			}
		}
		stockMap[janCode] = baseStock + netChange
	}

	return stockMap, nil
}

/**
 * @brief 指定された製品の、特定の日付時点での理論在庫を計算します。
 */
func CalculateStockOnDate(dbtx DBTX, productCode string, targetDate string) (float64, error) {
	var latestInventoryDate sql.NullString
	// 1. 基準日以前の最新の棚卸日を取得
	err := dbtx.QueryRow(`
		SELECT MAX(transaction_date) FROM transaction_records
		WHERE jan_code = ? AND flag = 0 AND transaction_date <= ?`,
		productCode, targetDate).Scan(&latestInventoryDate)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get latest inventory date for %s on or before %s: %w", productCode, targetDate, err)
	}

	if latestInventoryDate.Valid && latestInventoryDate.String != "" {
		// --- 棚卸履歴がある場合の計算 ---
		var baseStock float64
		// 1a. 棚卸日の在庫合計を基点とする
		err := dbtx.QueryRow(`
			SELECT SUM(yj_quantity) FROM transaction_records
			WHERE jan_code = ? AND flag = 0 AND transaction_date = ?`,
			productCode, latestInventoryDate.String).Scan(&baseStock)
		if err != nil {
			return 0, fmt.Errorf("failed to sum inventory for %s on %s: %w", productCode, latestInventoryDate.String, err)
		}

		// 1b. もし基準日が棚卸日当日なら、棚卸数量のみを返す
		if latestInventoryDate.String == targetDate {
			return baseStock, nil
		}

		// 1c. 棚卸日の翌日から基準日までの変動を計算
		var netChangeAfterInvDate sql.NullFloat64
		err = dbtx.QueryRow(`
			SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
			FROM transaction_records
			WHERE jan_code = ? AND flag != 0 AND transaction_date > ? AND transaction_date <= ?`,
			productCode, latestInventoryDate.String, targetDate).Scan(&netChangeAfterInvDate)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to calculate net change after inventory date for %s: %w", productCode, err)
		}

		return baseStock + netChangeAfterInvDate.Float64, nil

	} else {
		// --- 棚卸履歴がない場合の計算 ---
		var totalNetChange sql.NullFloat64
		err = dbtx.QueryRow(`
			SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
			FROM transaction_records
			WHERE jan_code = ? AND flag != 0 AND transaction_date <= ?`,
			productCode, targetDate).Scan(&totalNetChange)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to calculate total net change for %s: %w", productCode, err)
		}
		return totalNetChange.Float64, nil
	}
}
