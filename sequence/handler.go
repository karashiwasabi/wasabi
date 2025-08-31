// C:\Users\wasab\OneDrive\デスクトップ\WASABI\sequence\handler.go

package sequence

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"wasabi/db"
)

// GetNextSequenceHandler は、指定されたシーケンスの次の値を返します。
func GetNextSequenceHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// URLからシーケンス名を取得 (例: /api/sequence/next/MA2Y -> MA2Y)
		sequenceName := strings.TrimPrefix(r.URL.Path, "/api/sequence/next/")
		if sequenceName == "" {
			http.Error(w, "Sequence name is required.", http.StatusBadRequest)
			return
		}

		var prefix string
		var padding int

		// シーケンス名に応じた設定
		switch sequenceName {
		case "MA2Y":
			prefix = "MA2Y"
			padding = 8
		case "CL":
			prefix = "CL"
			padding = 4
		default:
			http.Error(w, "Unknown sequence name.", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		nextCode, err := db.NextSequenceInTx(tx, sequenceName, prefix, padding)
		if err != nil {
			http.Error(w, "Failed to get next sequence: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"nextCode": nextCode})
	}
}
