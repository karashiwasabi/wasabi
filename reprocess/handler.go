package reprocess

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
)

// ReProcessTransactionsHandler is the HTTP handler that triggers the reprocessing logic.
func ReProcessTransactionsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		count, err := reProcessProvisionalRecords(tx, conn)
		if err != nil {
			http.Error(w, "Failed to reprocess provisional records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d 件の仮登録データを更新しました。", count),
		})
	}
}

// reProcessProvisionalRecords contains the core logic for reprocessing.
func reProcessProvisionalRecords(tx *sql.Tx, conn *sql.DB) (int, error) {
	// 1. Get all provisional records
	provisionalRecords, err := db.GetProvisionalTransactions(conn)
	if err != nil {
		return 0, fmt.Errorf("failed to get provisional transactions: %w", err)
	}
	if len(provisionalRecords) == 0 {
		return 0, nil
	}

	// 2. Prepare necessary data (masters and JCSHMS) in bulk
	var keyList, janList []string
	keySet, janSet := make(map[string]struct{}), make(map[string]struct{})
	for _, rec := range provisionalRecords {
		if rec.JanCode != "" && rec.JanCode != "0000000000000" {
			if _, seen := janSet[rec.JanCode]; !seen {
				janSet[rec.JanCode] = struct{}{}
				janList = append(janList, rec.JanCode)
			}
		}
		key := rec.JanCode
		if key == "" || key == "0000000000000" {
			key = fmt.Sprintf("9999999999999%s", rec.ProductName)
		}
		if _, seen := keySet[key]; !seen {
			keySet[key] = struct{}{}
			keyList = append(keyList, key)
		}
	}
	mastersMap, err := db.GetProductMastersByCodesMap(conn, keyList)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk get product masters for reprocessing: %w", err)
	}
	jcshmsMap, err := db.GetJcshmsByCodesMap(conn, janList)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk get jcshms for reprocessing: %w", err)
	}

	// 3. Loop through each provisional record and try to resolve it
	updatedCount := 0
	for _, rec := range provisionalRecords {
		// Call mastermanager to find or create a master. This uses the newly added JCSHMS data.
		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			// Log the error but continue processing other records
			log.Printf("Reprocess: mastermanager failed for jan %s, skipping: %v", rec.JanCode, err)
			continue
		}

		// If the master is no longer provisional, the reprocessing was successful.
		if master.Origin != "PROVISIONAL" {
			mappers.MapProductMasterToTransaction(&rec, master)
			rec.ProcessFlagMA = "COMPLETE"
			rec.ProcessingStatus = sql.NullString{String: "completed", Valid: true}

			// Save the updated record to the database
			if err := db.UpdateFullTransactionInTx(tx, &rec); err != nil {
				return 0, fmt.Errorf("failed to update reprocessed transaction ID %d: %w", rec.ID, err)
			}
			updatedCount++
		}
	}

	return updatedCount, nil
}
