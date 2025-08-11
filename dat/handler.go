package dat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wasabi/db"
	"wasabi/model"
	"wasabi/parsers" // <-- IMPORT ADDED
)

// UploadDatHandler はDATファイルのアップロードを処理します。
func UploadDatHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()

		// 1. パース
		var allParsedRecords []model.UnifiedInputRecord
		for _, fileHeader := range r.MultipartForm.File["file"] {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Failed to open file %s: %v", fileHeader.Filename, err)
				continue
			}
			defer file.Close()
			parsed, err := parsers.ParseDat(file) // <-- CORRECTED
			if err != nil {
				log.Printf("Failed to parse file %s: %v", fileHeader.Filename, err)
				continue
			}
			allParsedRecords = append(allParsedRecords, parsed...)
		}

		// 2. 重複削除
		filteredRecords := removeDatDuplicates(allParsedRecords)

		// 3. トランザクション処理
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// 4. プロセッサ呼び出し
		finalRecords, err := ProcessDatRecords(tx, conn, filteredRecords)
		if err != nil {
			log.Printf("ProcessDatRecords failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if len(finalRecords) > 0 {
			if err := db.PersistTransactionRecordsInTx(tx, finalRecords); err != nil {
				log.Printf("db.PersistTransactionRecordsInTx error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("transaction commit error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// 5. 描画（JSONレスポンス）
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Parsed and processed DAT files successfully",
			"records": finalRecords,
		})
	}
}

// removeDatDuplicates はDATレコードの重複を削除します。
func removeDatDuplicates(records []model.UnifiedInputRecord) []model.UnifiedInputRecord {
	seen := make(map[string]struct{})
	var result []model.UnifiedInputRecord
	for _, r := range records {
		key := fmt.Sprintf("%s|%s|%s|%s", r.Date, r.ClientCode, r.ReceiptNumber, r.LineNumber)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, r)
	}
	return result
}
