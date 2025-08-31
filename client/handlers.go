// C:\Users\wasab\OneDrive\デスクトップ\WASABI\client\handler.go

package client

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
)

/**
 * @brief 全ての得意先リストを取得するAPIハンドラ (/api/clients)
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 */
func GetAllClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clients)
	}
}
