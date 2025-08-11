package usage

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

// UploadUsageHandler handles the USAGE file upload process.
func UploadUsageHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()

		// 1. Parse all uploaded files
		var allParsed []model.UnifiedInputRecord
		for _, fh := range r.MultipartForm.File["file"] {
			f, err := fh.Open()
			if err != nil {
				log.Printf("Failed to open file %s: %v", fh.Filename, err)
				continue
			}
			defer f.Close()
			recs, err := parsers.ParseUsage(f) // <-- CORRECTED
			if err != nil {
				log.Printf("Failed to parse file %s: %v", fh.Filename, err)
				continue
			}
			allParsed = append(allParsed, recs...)
		}

		// 2. Remove duplicates specific to USAGE data
		filtered := removeUsageDuplicates(allParsed)

		if len(filtered) == 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"records": []model.TransactionRecord{}})
			return
		}

		// 3. Start transaction and delete existing usage data for the affected date range
		tx, err := conn.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction for usage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		minDate, maxDate := "99999999", "00000000"
		for _, rec := range filtered {
			if rec.Date < minDate {
				minDate = rec.Date
			}
			if rec.Date > maxDate {
				maxDate = rec.Date
			}
		}
		if err := db.DeleteUsageTransactionsInDateRange(tx, minDate, maxDate); err != nil {
			log.Printf("db.DeleteUsageTransactionsInDateRange error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// 4. Call the processor to create transaction records
		finalRecords, err := ProcessUsageRecords(tx, conn, filtered)
		if err != nil {
			log.Printf("ProcessUsageRecords failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if len(finalRecords) > 0 {
			if err := db.PersistTransactionRecordsInTx(tx, finalRecords); err != nil {
				log.Printf("PersistTransactionRecordsInTx error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("transaction commit error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// 5. Render the result as a JSON response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"records": finalRecords,
		})
	}
}

// removeUsageDuplicates is the de-duplication logic specific to USAGE records.
func removeUsageDuplicates(records []model.UnifiedInputRecord) []model.UnifiedInputRecord {
	seen := make(map[string]struct{})
	var result []model.UnifiedInputRecord
	for _, r := range records {
		// Key is based on date and product identifiers
		key := fmt.Sprintf("%s|%s|%s|%s", r.Date, r.JanCode, r.YjCode, r.ProductName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, r)
	}
	return result
}
