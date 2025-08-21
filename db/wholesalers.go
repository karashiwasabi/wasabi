// C:\Dev\WASABI\db\wholesalers.go

package db

import (
	"database/sql"
	"fmt"
	"wasabi/model"
)

// GetAllWholesalers は全ての卸業者を取得します。
func GetAllWholesalers(conn *sql.DB) ([]model.Wholesaler, error) {
	rows, err := conn.Query("SELECT wholesaler_code, wholesaler_name FROM wholesalers ORDER BY wholesaler_code")
	if err != nil {
		return nil, fmt.Errorf("failed to get all wholesalers: %w", err)
	}
	defer rows.Close()

	// ▼▼▼ [修正点] nilスライスではなく、空のスライスで初期化する ▼▼▼
	wholesalers := make([]model.Wholesaler, 0)
	// ▲▲▲ 修正ここまで ▲▲▲
	for rows.Next() {
		var w model.Wholesaler
		if err := rows.Scan(&w.Code, &w.Name); err != nil {
			return nil, err
		}
		wholesalers = append(wholesalers, w)
	}
	return wholesalers, nil
}

// CreateWholesaler は新しい卸業者を作成します。
func CreateWholesaler(conn *sql.DB, code, name string) error {
	const q = `INSERT INTO wholesalers (wholesaler_code, wholesaler_name) VALUES (?, ?)`
	_, err := conn.Exec(q, code, name)
	if err != nil {
		return fmt.Errorf("CreateWholesaler failed: %w", err)
	}
	return nil
}

// DeleteWholesaler は指定されたコードの卸業者を削除します。
func DeleteWholesaler(conn *sql.DB, code string) error {
	const q = `DELETE FROM wholesalers WHERE wholesaler_code = ?`
	_, err := conn.Exec(q, code)
	if err != nil {
		return fmt.Errorf("failed to delete wholesaler with code %s: %w", code, err)
	}
	return nil
}