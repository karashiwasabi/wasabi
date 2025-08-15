package precomp

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
	"wasabi/model"
)

type PrecompRecordInput struct {
	ProductCode string  `json:"productCode"`
	JanQuantity float64 `json:"janQuantity"`
}

type PrecompPayload struct {
	PatientNumber string               `json:"patientNumber"`
	Records       []PrecompRecordInput `json:"records"`
}

func SavePrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload PrecompPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if payload.PatientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var productCodes []string
		for _, rec := range payload.Records {
			productCodes = append(productCodes, rec.ProductCode)
		}

		masters, err := db.GetProductMastersByCodesMap(tx, productCodes)
		if err != nil {
			http.Error(w, "Failed to get product masters", http.StatusInternalServerError)
			return
		}

		var recordsToSave []model.PreCompoundingRecord
		for _, rec := range payload.Records {
			master, ok := masters[rec.ProductCode]
			if !ok {
				continue
			}

			yjQuantity := rec.JanQuantity * master.JanPackInnerQty
			recordsToSave = append(recordsToSave, model.PreCompoundingRecord{
				ProductCode: rec.ProductCode,
				Quantity:    yjQuantity,
			})
		}

		if err := db.UpsertPreCompoundingRecordsInTx(tx, payload.PatientNumber, recordsToSave); err != nil {
			http.Error(w, "Failed to save pre-compounding records", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製データを保存しました。"})
	}
}

func LoadPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		records, err := db.GetPreCompoundingRecordsByPatient(conn, patientNumber)
		if err != nil {
			http.Error(w, "Failed to load pre-compounding records", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	}
}

func ClearPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		if err := db.DeletePreCompoundingRecordsByPatient(conn, patientNumber); err != nil {
			http.Error(w, "Failed to clear pre-compounding records", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製データを中断しました。"})
	}
}
