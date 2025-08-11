package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/parsers" // <-- IMPORT ADDED
)

// UploadInventoryHandler handles the inventory file upload process.
func UploadInventoryHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "File upload error", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 1. Parse the inventory file
		parsedData, err := parsers.ParseInventoryFile(file) // <-- CORRECTED
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse file: %v", err), http.StatusBadRequest)
			return
		}
		date := parsedData.Date
		if date == "" {
			http.Error(w, "Inventory date not found in file's H record", http.StatusBadRequest)
			return
		}

		// 2. Pre-process records for the processor
		recordsToProcess := parsedData.Records
		for i := range recordsToProcess {
			// YjQuantity is a required input for the processor
			recordsToProcess[i].YjQuantity = recordsToProcess[i].JanQuantity * recordsToProcess[i].JanPackInnerQty
		}

		// 3. Start transaction and delete existing inventory data for the date
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.DeleteTransactionsByFlagAndDate(tx, 0, date); err != nil { // Flag 0 for inventory
			http.Error(w, "Failed to delete existing inventory data for date "+date, http.StatusInternalServerError)
			return
		}

		// 4. Call the processor to create transaction records
		finalRecords, err := ProcessInventoryRecords(tx, conn, recordsToProcess)
		if err != nil {
			http.Error(w, "Failed to process inventory records", http.StatusInternalServerError)
			return
		}

		// 5. Finalize transaction records with date, receipt, and line numbers
		receiptNumber := fmt.Sprintf("INV%s", date)
		for i := range finalRecords {
			finalRecords[i].TransactionDate = date
			finalRecords[i].ReceiptNumber = receiptNumber
			finalRecords[i].LineNumber = fmt.Sprintf("%d", i+1)
		}

		// 6. Persist records to the database
		if len(finalRecords) > 0 {
			if err := db.PersistTransactionRecordsInTx(tx, finalRecords); err != nil {
				http.Error(w, "Failed to save inventory records to transaction", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// 7. Render the result as a JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("%d件の棚卸データを登録しました。", len(finalRecords)),
			"details": finalRecords,
		})
	}
}
