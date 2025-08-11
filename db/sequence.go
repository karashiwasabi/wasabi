package db

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
)

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

// InitializeSequenceFromMaxYjCode resets the MA2Y sequence based on the max yj_code in product_master.
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

// InitializeSequenceFromMaxClientCode resets the CL sequence based on the max client_code in client_master.
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
