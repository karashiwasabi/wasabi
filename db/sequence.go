// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\sequence.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
)

/**
 * @brief 指定されたシーケンスの次の値を発行します。
 * @param tx SQLトランザクションオブジェクト
 * @param name シーケンス名 (例: "MA2Y", "CL")
 * @param prefix 新しいコードに付与する接頭辞 (例: "MA2Y", "CL")
 * @param padding ゼロ埋めする桁数
 * @return string 生成された新しいコード (例: "CL0001")
 * @return error 処理中にエラーが発生した場合
 * @details
 * code_sequencesテーブルから現在の最終番号を取得し、1加算して更新し、
 * フォーマットされた新しいコード文字列を返します。
 * この処理はアトミック性を保証するため、必ずトランザクション内で実行されます。
 */
func NextSequenceInTx(tx *sql.Tx, name, prefix string, padding int) (string, error) {
	var lastNo int
	err := tx.QueryRow("SELECT last_no FROM code_sequences WHERE name = ?", name).Scan(&lastNo)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("sequence '%s' not found", name)
		}
		return "", fmt.Errorf("failed to get sequence '%s': %w", name, err)
	}

	newNo := lastNo + 1
	_, err = tx.Exec("UPDATE code_sequences SET last_no = ? WHERE name = ?", newNo, name)
	if err != nil {
		return "", fmt.Errorf("failed to update sequence '%s': %w", name, err)
	}

	format := fmt.Sprintf("%s%%0%dd", prefix, padding)
	return fmt.Sprintf(format, newNo), nil
}

/**
 * @brief 製品マスターのyj_codeからMA2Yシーケンスのカウンターを初期化（リセット）します。
 * @param conn データベース接続
 * @return error 処理中にエラーが発生した場合
 * @details
 * 製品マスターの一括インポート後などに呼び出され、既存のyj_codeの最大値を取得し、
 * code_sequencesテーブルのカウンターをその値に設定することで、コードの重複を防ぎます。
 */
func InitializeSequenceFromMaxYjCode(conn *sql.DB) error {
	var maxNo int64 = 0
	prefix := "MA2Y"
	rows, err := conn.Query("SELECT yj_code FROM product_master WHERE yj_code LIKE ?", prefix+"%")
	if err != nil {
		return fmt.Errorf("failed to query existing yj_codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var yjCode string
		if err := rows.Scan(&yjCode); err != nil {
			log.Printf("Warn: could not scan yj_code: %v", err)
			continue
		}
		numPart := strings.TrimPrefix(yjCode, prefix)
		if num, err := strconv.ParseInt(numPart, 10, 64); err == nil {
			if num > maxNo {
				maxNo = num
			}
		}
	}

	if maxNo > 0 {
		_, err = conn.Exec("UPDATE code_sequences SET last_no = ? WHERE name = ?", maxNo, "MA2Y")
		if err != nil {
			return fmt.Errorf("failed to update MA2Y sequence with max value %d: %w", maxNo, err)
		}
		log.Printf("MA2Y sequence initialized to %d.", maxNo)
	}
	return nil
}

/**
 * @brief 得意先マスターのclient_codeからCLシーケンスのカウンターを初期化（リセット）します。
 * @param conn データベース接続
 * @return error 処理中にエラーが発生した場合
 * @details
 * 得意先マスターの一括インポート後などに呼び出され、既存のclient_codeの最大値を取得し、
 * code_sequencesテーブルのカウンターをその値に設定することで、コードの重複を防ぎます。
 */
func InitializeSequenceFromMaxClientCode(conn *sql.DB) error {
	var maxNo int64 = 0
	prefix := "CL"
	rows, err := conn.Query("SELECT client_code FROM client_master WHERE client_code LIKE ?", prefix+"%")
	if err != nil {
		return fmt.Errorf("failed to query existing client_codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var clientCode string
		if err := rows.Scan(&clientCode); err != nil {
			log.Printf("Warn: could not scan client_code: %v", err)
			continue
		}
		numPart := strings.TrimPrefix(clientCode, prefix)
		if num, err := strconv.ParseInt(numPart, 10, 64); err == nil {
			if num > maxNo {
				maxNo = num
			}
		}
	}

	if maxNo > 0 {
		_, err = conn.Exec("UPDATE code_sequences SET last_no = ? WHERE name = ?", maxNo, "CL")
		if err != nil {
			return fmt.Errorf("failed to update CL sequence with max value %d: %w", maxNo, err)
		}
		log.Printf("CL sequence initialized to %d.", maxNo)
	}
	return nil
}
