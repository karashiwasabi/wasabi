// C:\Dev\WASABI\db\stock.go

package db

import (
	"database/sql"
	"fmt"
)

// ▼▼▼ [修正点] 関数全体を、同日棚卸の合算ロジックに修正 ▼▼▼
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

// ▲▲▲ 修正ここまで ▲▲▲