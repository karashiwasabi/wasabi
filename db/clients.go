// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\clients.go

package db

import (
	"database/sql"
	"fmt"
	"wasabi/model"
)

/**
 * @brief 新しい得意先レコードをトランザクション内で作成します。
 * @param tx SQLトランザクションオブジェクト
 * @param code 新しい得意先コード
 * @param name 新しい得意先名
 * @return error 処理中にエラーが発生した場合
 */
func CreateClientInTx(tx *sql.Tx, code, name string) error {
	const q = `INSERT INTO client_master (client_code, client_name) VALUES (?, ?)`
	_, err := tx.Exec(q, code, name)
	if err != nil {
		return fmt.Errorf("CreateClientInTx failed: %w", err)
	}
	return nil
}

/**
 * @brief 指定された名前の得意先が既に存在するかをトランザクション内で確認します。
 * @param tx SQLトランザクションオブジェクト
 * @param name 確認する得意先名
 * @return bool 存在する場合は true, しない場合は false
 * @return error 処理中にエラーが発生した場合
 */
func CheckClientExistsByName(tx *sql.Tx, name string) (bool, error) {
	var exists int
	const q = `SELECT 1 FROM client_master WHERE client_name = ? LIMIT 1`
	err := tx.QueryRow(q, name).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			// レコードが存在しないのはエラーではない
			return false, nil
		}
		return false, fmt.Errorf("CheckClientExistsByName failed: %w", err)
	}
	return true, nil
}

/**
 * @brief 全ての得意先を client_code 順で取得します。
 * @param conn データベース接続
 * @return []model.Client 得意先のスライス
 * @return error 処理中にエラーが発生した場合
 */
func GetAllClients(conn *sql.DB) ([]model.Client, error) {
	rows, err := conn.Query("SELECT client_code, client_name FROM client_master ORDER BY client_code")
	if err != nil {
		return nil, fmt.Errorf("failed to get all clients: %w", err)
	}
	defer rows.Close()

	// 空のスライスで初期化することで、得意先が0件の場合にJSONでnullではなく空配列[]を返す
	clients := make([]model.Client, 0)
	for rows.Next() {
		var c model.Client
		if err := rows.Scan(&c.Code, &c.Name); err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}
