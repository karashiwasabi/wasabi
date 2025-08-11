package db

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// GetAllClientsHandler は /api/clients のリクエストを処理します。
func GetAllClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clients)
	}
}

// SearchJcshmsByNameHandler は /api/products/search のリクエストを処理します。
func SearchJcshmsByNameHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if len(query) < 2 {
			http.Error(w, "Query must be at least 2 characters", http.StatusBadRequest)
			return
		}
		results, err := SearchJcshmsByName(conn, query)
		if err != nil {
			http.Error(w, "Failed to search products", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
