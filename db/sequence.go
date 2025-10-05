package db

import (
	"database/sql"
	"fmt"
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
 */
func NextSequenceInTx(tx *sql.Tx, name, prefix string, padding int) (string, error) {
	var lastNo int
	err := tx.QueryRow("SELECT last_no FROM code_sequences WHERE name = ?", name).Scan(&lastNo)
	if err != nil {
		if err == sql.ErrNoRows {
			// シーケンスが存在しない場合は、ここで作成するロジックを追加することも可能
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

func InitializeSequenceFromMaxClientCode(tx *sql.Tx) error {
	var maxCode string
	err := tx.QueryRow("SELECT client_code FROM client_master ORDER BY client_code DESC LIMIT 1").Scan(&maxCode)
	if err != nil {
		if err == sql.ErrNoRows {
			// レコードがない場合は0で初期化
			_, err = tx.Exec("UPDATE code_sequences SET last_no = 0 WHERE name = 'CL'")
			return err
		}
		return err
	}
	if strings.HasPrefix(maxCode, "CL") {
		numPart := strings.TrimPrefix(maxCode, "CL")
		maxNum, _ := strconv.Atoi(numPart)
		_, err = tx.Exec("UPDATE code_sequences SET last_no = ? WHERE name = 'CL'", maxNum)
		return err
	}
	return nil
}

func InitializeSequenceFromMaxYjCode(tx *sql.Tx) error {
	var maxYj string
	err := tx.QueryRow("SELECT yj_code FROM product_master WHERE yj_code LIKE 'MA2Y%' ORDER BY yj_code DESC LIMIT 1").Scan(&maxYj)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = tx.Exec("UPDATE code_sequences SET last_no = 0 WHERE name = 'MA2Y'")
			return err
		}
		return err
	}
	if strings.HasPrefix(maxYj, "MA2Y") {
		numPart := strings.TrimPrefix(maxYj, "MA2Y")
		maxNum, _ := strconv.Atoi(numPart)
		_, err = tx.Exec("UPDATE code_sequences SET last_no = ? WHERE name = 'MA2Y'", maxNum)
		return err
	}
	return nil
}
