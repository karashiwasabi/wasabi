// C:\Dev\WASABI\db\stock.go

package db

import (
	"database/sql"
	"fmt"
)

// CalculateCurrentStockForProduct は、棚卸を考慮した正確な現在庫を計算します
func CalculateCurrentStockForProduct(executor DBTX, janCode string) (float64, error) {
	// 1. 最新の棚卸日を取得
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
		// 2. 最新棚卸日の在庫を合算して基点在庫とする
		err := executor.QueryRow(`
			SELECT SUM(yj_quantity) FROM transaction_records
			WHERE jan_code = ? AND flag = 0 AND transaction_date = ?`,
			janCode, latestInventoryDate.String).Scan(&baseStock)
		if err != nil {
			return 0, fmt.Errorf("failed to sum inventory for %s on %s: %w", janCode, latestInventoryDate.String, err)
		}

		// 3. 最新棚卸日以降の変動を計算するクエリを準備
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
		// 棚卸履歴がない場合、全期間の変動を計算
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

	// 4. 変動を計算
	var nullNetChange sql.NullFloat64
	err = executor.QueryRow(netChangeQuery, args...).Scan(&nullNetChange)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to calculate net change for %s: %w", janCode, err)
	}
	netChange := nullNetChange.Float64

	// 5. 最終在庫を計算
	return baseStock + netChange, nil
}

// GetAllCurrentStockMap は全製品の現在庫を効率的に計算し、マップで返します
func GetAllCurrentStockMap(conn *sql.DB) (map[string]float64, error) {
	// 全トランザクションを製品コードと日付でソートして取得
	rows, err := conn.Query(`
		SELECT jan_code, transaction_date, flag, yj_quantity 
		FROM transaction_records 
		ORDER BY jan_code, transaction_date, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions for stock calculation: %w", err)
	}
	defer rows.Close()

	stockMap := make(map[string]float64)
	// ▼▼▼ [修正点] 未使用の変数を削除 ▼▼▼
	// var currentJanCode string
	// ▲▲▲ 修正ここまで ▲▲▲

	type txRecord struct {
		Date string
		Flag int
		Qty  float64
	}
	recordsByJan := make(map[string][]txRecord)

	// まず全レコードをJANコードごとにメモリにグループ化
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

	// JANコードごとに在庫を計算
	for janCode, records := range recordsByJan {
		var latestInvDate string
		baseStock := 0.0

		// 最新の棚卸日と、その日の在庫合計を検索
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

		// 棚卸以降（または全期間）の変動を計算
		netChange := 0.0
		for _, r := range records {
			startDate := "00000000"
			if latestInvDate != "" {
				startDate = latestInvDate
			}

			if r.Date > startDate {
				switch r.Flag {
				case 1, 4, 11: // 入庫系
					netChange += r.Qty
				case 2, 3, 5, 12: // 出庫系
					netChange -= r.Qty
				}
			}
		}
		stockMap[janCode] = baseStock + netChange
	}

	return stockMap, nil
}
