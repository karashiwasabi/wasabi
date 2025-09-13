// C:\Users\wasab\OneDrive\デスクトップ\WASABI\transaction\handler.go

package transaction

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv" // strconvをインポート
	"strings"
	"wasabi/db"
)

// GetReceiptsHandler returns a list of receipt numbers for a given date.
func GetReceiptsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		}
		// Assuming you will add GetReceiptNumbersByDate to db package
		receipts, err := db.GetReceiptNumbersByDate(conn, date)
		if err != nil {
			http.Error(w, "Failed to get receipt numbers", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(receipts)
	}
}

// GetTransactionHandler returns all line items for a given receipt number.
func GetTransactionHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receiptNumber := strings.TrimPrefix(r.URL.Path, "/api/transaction/")
		if receiptNumber == "" {
			http.Error(w, "Receipt number is required", http.StatusBadRequest)
			return
		}
		records, err := db.GetTransactionsByReceiptNumber(conn, receiptNumber)
		if err != nil {
			http.Error(w, "Failed to get transaction details", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	}
}

// DeleteTransactionHandler handles the deletion of all records for a given receipt number.
func DeleteTransactionHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receiptNumber := strings.TrimPrefix(r.URL.Path, "/api/transaction/delete/")
		if receiptNumber == "" {
			http.Error(w, "Receipt number is required", http.StatusBadRequest)
			return
		}
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.DeleteTransactionsByReceiptNumberInTx(tx, receiptNumber); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Deleted successfully"})
	}
}

// ▼▼▼【ここから追加】▼▼▼

// GetInventoryByDateHandler は指定された日付の棚卸レコードを返します。
func GetInventoryByDateHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			http.Error(w, "date parameter is required", http.StatusBadRequest)
			return
		}
		records, err := db.GetInventoryTransactionsByDate(conn, date)
		if err != nil {
			http.Error(w, "Failed to get inventory transactions by date", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	}
}

// DeleteTransactionByIDHandler はIDを指定して単一の取引レコードを削除します。
func DeleteTransactionByIDHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// URLからIDを取得 (e.g., /api/transaction/delete_by_id/123)
		idStr := strings.TrimPrefix(r.URL.Path, "/api/transaction/delete_by_id/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.DeleteTransactionByIDInTx(tx, id); err != nil {
			http.Error(w, "Failed to delete transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "レコードを削除しました。"})
	}
}

// ▲▲▲【追加ここまで】▲▲▲
