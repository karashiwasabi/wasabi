package db

import (
	"database/sql"
	"fmt"
	"wasabi/model" // <-- IMPORT ADDED
)

// CreateClientInTx creates a new client record within a transaction.
func CreateClientInTx(tx *sql.Tx, code, name string) error {
	const q = `INSERT INTO client_master (client_code, client_name) VALUES (?, ?)`
	_, err := tx.Exec(q, code, name)
	if err != nil {
		return fmt.Errorf("CreateClientInTx failed: %w", err)
	}
	return nil
}

// CheckClientExistsByName checks if a client with the given name already exists.
func CheckClientExistsByName(tx *sql.Tx, name string) (bool, error) {
	var exists int
	const q = `SELECT 1 FROM client_master WHERE client_name = ? LIMIT 1`
	err := tx.QueryRow(q, name).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Does not exist
		}
		return false, fmt.Errorf("CheckClientExistsByName failed: %w", err) // Other error
	}
	return true, nil // Exists
}

// GetAllClients retrieves all clients from the client_master table.
func GetAllClients(conn *sql.DB) ([]model.Client, error) {
	rows, err := conn.Query("SELECT client_code, client_name FROM client_master ORDER BY client_code")
	if err != nil {
		return nil, fmt.Errorf("failed to get all clients: %w", err)
	}
	defer rows.Close()

	// ▼▼▼ [修正点] nilスライスではなく、空のスライスで初期化する ▼▼▼
	clients := make([]model.Client, 0)
	// ▲▲▲ 修正ここまで ▲▲▲
	for rows.Next() {
		var c model.Client
		if err := rows.Scan(&c.Code, &c.Name); err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}