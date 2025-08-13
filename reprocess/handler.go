package reprocess

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
)

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
			"message": fmt.Sprintf("%d 件の仮登録データの情報を更新しました。", count),
		})
	}
}

func reProcessProvisionalRecords(tx *sql.Tx, conn *sql.DB) (int, error) {
	// 1. Get all provisional transactions.
	provisionalRecords, err := db.GetProvisionalTransactions(conn)
	if err != nil {
		return 0, fmt.Errorf("failed to get provisional transactions: %w", err)
	}
	if len(provisionalRecords) == 0 {
		return 0, nil
	}

	// 2. Get all relevant existing product masters.
	var keyList []string
	keySet := make(map[string]struct{})
	for _, rec := range provisionalRecords {
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

	// 3. Process each provisional record.
	updatedCount := 0
	for _, rec := range provisionalRecords {
		key := rec.JanCode
		if key == "" || key == "0000000000000" {
			key = fmt.Sprintf("9999999999999%s", rec.ProductName)
		}

		// Check if a definitive master (MANUAL or JCSHMS) exists.
		if master, ok := mastersMap[key]; ok && master.Origin != "PROVISIONAL" {
			// A definitive master was found.
			// Enrich the transaction's data with the master's data.
			mappers.MapProductMasterToTransaction(&rec, master)

			// DO NOT CHANGE THE STATUS. It remains PROVISIONAL.

			// Save the enriched data.
			if err := db.UpdateFullTransactionInTx(tx, &rec); err != nil {
				return 0, fmt.Errorf("failed to update reprocessed transaction ID %d: %w", rec.ID, err)
			}
			updatedCount++ // Count any record that had its data enriched.
		}
	}

	return updatedCount, nil
}
