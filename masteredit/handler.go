package masteredit

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
	"wasabi/model"
)

// GetEditableMastersHandler returns a list of editable product masters.
func GetEditableMastersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		masters, err := db.GetEditableProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get editable masters", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masters)
	}
}

// UpdateMasterHandler updates or inserts a product master record from the edit screen.
func UpdateMasterHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var input model.ProductMasterInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// A product code is mandatory for saving a record.
		if input.ProductCode == "" {
			http.Error(w, "Product Code (JAN) cannot be empty.", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.UpsertProductMasterInTx(tx, input); err != nil {
			http.Error(w, "Failed to upsert product master", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Saved successfully."})
	}
}