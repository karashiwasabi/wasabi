// C:\Users\wasab\OneDrive\デスクトップ\WASABI\cleanup\handler.go

package cleanup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
)

// GetCandidatesHandler は整理対象のマスター候補をリストアップします。
func GetCandidatesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidates, err := db.GetCleanupCandidates(conn)
		if err != nil {
			http.Error(w, "Failed to get cleanup candidates: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(candidates)
	}
}

// ExecuteCleanupHandler は指定されたマスターを削除します。
func ExecuteCleanupHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ProductCodes []string `json:"productCodes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		rowsAffected, err := db.DeleteMastersByCodesInTx(tx, payload.ProductCodes)
		if err != nil {
			http.Error(w, "Failed to execute cleanup: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の製品マスターを削除しました。", rowsAffected),
		})
	}
}
