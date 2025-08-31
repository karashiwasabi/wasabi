// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\wholesalers.go

package db

import (
	"database/sql"
	"fmt"
	"wasabi/model"
)

/**
 * @brief 全ての卸業者を wholesaler_code 順で取得します。
 * @param conn データベース接続
 * @return []model.Wholesaler 卸業者のスライス
 * @return error 処理中にエラーが発生した場合
 */
func GetAllWholesalers(conn *sql.DB) ([]model.Wholesaler, error) {
	rows, err := conn.Query("SELECT wholesaler_code, wholesaler_name FROM wholesalers ORDER BY wholesaler_code")
	if err != nil {
		return nil, fmt.Errorf("failed to get all wholesalers: %w", err)
	}
	defer rows.Close()

	// 空のスライスで初期化することで、卸業者が0件の場合にJSONでnullではなく空配列[]を返す
	wholesalers := make([]model.Wholesaler, 0)
	for rows.Next() {
		var w model.Wholesaler
		if err := rows.Scan(&w.Code, &w.Name); err != nil {
			return nil, err
		}
		wholesalers = append(wholesalers, w)
	}
	return wholesalers, nil
}

/**
 * @brief 新しい卸業者を作成します。
 * @param conn データベース接続
 * @param code 卸業者コード
 * @param name 卸業者名
 * @return error 処理中にエラーが発生した場合
 */
func CreateWholesaler(conn *sql.DB, code, name string) error {
	const q = `INSERT INTO wholesalers (wholesaler_code, wholesaler_name) VALUES (?, ?)`
	_, err := conn.Exec(q, code, name)
	if err != nil {
		return fmt.Errorf("CreateWholesaler failed: %w", err)
	}
	return nil
}

/**
 * @brief 指定されたコードの卸業者を削除します。
 * @param conn データベース接続
 * @param code 削除する卸業者のコード
 * @return error 処理中にエラーが発生した場合
 */
func DeleteWholesaler(conn *sql.DB, code string) error {
	const q = `DELETE FROM wholesalers WHERE wholesaler_code = ?`
	_, err := conn.Exec(q, code)
	if err != nil {
		return fmt.Errorf("failed to delete wholesaler with code %s: %w", code, err)
	}
	return nil
}
