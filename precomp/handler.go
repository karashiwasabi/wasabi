package precomp

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"wasabi/db"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// PrecompPayload は保存・更新時にフロントエンドから受け取るデータ構造です
type PrecompPayload struct {
	PatientNumber string                  `json:"patientNumber"`
	Records       []db.PrecompRecordInput `json:"records"`
}

// LoadResponse は呼び出し時にフロントエンドへ返すデータ構造です
type LoadResponse struct {
	Status  string                    `json:"status"`
	Records []model.TransactionRecord `json:"records"`
}

// SavePrecompHandler は予製データを保存・更新します
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

		if err := db.UpsertPreCompoundingRecordsInTx(tx, payload.PatientNumber, payload.Records); err != nil {
			log.Printf("ERROR: Failed to save pre-compounding records for patient %s: %v", payload.PatientNumber, err)
			http.Error(w, "Failed to save pre-compounding records: "+err.Error(), http.StatusInternalServerError)
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

// LoadPrecompHandler は患者の予製データと現在のステータスを返します
func LoadPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		status, err := db.GetPreCompoundingStatusByPatient(conn, patientNumber)
		if err != nil {
			http.Error(w, "Failed to get pre-compounding status: "+err.Error(), http.StatusInternalServerError)
			return
		}
		records, err := db.GetPreCompoundingRecordsByPatient(conn, patientNumber)
		if err != nil {
			http.Error(w, "Failed to load pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := LoadResponse{
			Status:  status,
			Records: records,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// ClearPrecompHandler は予製データを完全に削除します
func ClearPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		if err := db.DeletePreCompoundingRecordsByPatient(conn, patientNumber); err != nil {
			http.Error(w, "Failed to clear pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製データを完全に削除しました。"})
	}
}

// SuspendPrecompHandler は予製を中断状態にします
func SuspendPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			PatientNumber string `json:"patientNumber"`
		}
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
		if err := db.SuspendPreCompoundingRecordsByPatient(tx, payload.PatientNumber); err != nil {
			http.Error(w, "Failed to suspend pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製を中断しました。"})
	}
}

// ResumePrecompHandler は予製を再開状態にします
func ResumePrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			PatientNumber string `json:"patientNumber"`
		}
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
		if err := db.ResumePreCompoundingRecordsByPatient(tx, payload.PatientNumber); err != nil {
			http.Error(w, "Failed to resume pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製を再開しました。"})
	}
}

// GetStatusPrecompHandler は予製の現在の状態を返します
func GetStatusPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}
		status, err := db.GetPreCompoundingStatusByPatient(conn, patientNumber)
		if err != nil {
			http.Error(w, "Failed to get status: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}

func ExportPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patientNumber := r.URL.Query().Get("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		records, err := db.GetPreCompoundingRecordsByPatient(conn, patientNumber)
		if err != nil {
			http.Error(w, "Failed to load pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		dirPath := filepath.Join("download", "exports")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}
		fileName := fmt.Sprintf("precomp_%s.csv", patientNumber)
		filePath := filepath.Join(dirPath, fileName)

		file, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		file.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
		csvWriter := csv.NewWriter(file)

		header := []string{"product_code", "product_name", "quantity_jan", "unit_name"}
		csvWriter.Write(header)

		for _, rec := range records {
			row := []string{
				fmt.Sprintf(`="%s"`, rec.JanCode), // Excel対応
				rec.ProductName,
				fmt.Sprintf("%f", rec.JanQuantity),
				rec.JanUnitName,
			}
			csvWriter.Write(row)
		}
		csvWriter.Flush()

		http.ServeFile(w, r, filePath)
	}
}

func ImportPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB limit
			http.Error(w, "File upload error", http.StatusBadRequest)
			return
		}

		patientNumber := r.FormValue("patientNumber")
		if patientNumber == "" {
			http.Error(w, "Patient number is required", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		reader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(reader)
		csvReader.LazyQuotes = true

		records, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}

		var precompRecords []db.PrecompRecordInput
		for i, row := range records {
			if i == 0 || len(row) < 3 {
				continue
			}
			productCode := row[0]
			quantity, err := strconv.ParseFloat(row[2], 64)
			if err != nil || productCode == "" {
				continue
			}
			precompRecords = append(precompRecords, db.PrecompRecordInput{
				ProductCode: productCode,
				JanQuantity: quantity,
			})
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.UpsertPreCompoundingRecordsInTx(tx, patientNumber, precompRecords); err != nil {
			http.Error(w, "Failed to save pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の予製データをインポートしました。", len(precompRecords)),
		})
	}
}

func ExportAllPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := db.GetAllPreCompoundingRecords(conn)
		if err != nil {
			http.Error(w, "Failed to load all pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fileName := "precomp_all.csv"
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		header := []string{"patient_number", "product_code", "product_name", "quantity_jan", "unit_name"}
		if err := csvWriter.Write(header); err != nil {
			log.Printf("Failed to write CSV header for all precomp export: %v", err)
			return
		}

		for _, rec := range records {
			row := []string{
				rec.ClientCode,
				rec.JanCode,
				rec.ProductName,
				fmt.Sprintf("%f", rec.JanQuantity),
				rec.JanUnitName,
			}
			if err := csvWriter.Write(row); err != nil {
				log.Printf("Failed to write CSV row for all precomp export: %v", err)
				continue
			}
		}
	}
}

func BulkImportPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		reader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(reader)
		csvReader.LazyQuotes = true

		records, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}

		recordsByPatient := make(map[string][]db.PrecompRecordInput)
		var importedCount int
		for i, row := range records {
			if i == 0 || len(row) < 4 {
				continue
			}
			patientNumber := row[0]
			productCode := row[1]
			quantity, err := strconv.ParseFloat(row[3], 64)
			if err != nil || productCode == "" || patientNumber == "" {
				continue
			}
			recordsByPatient[patientNumber] = append(recordsByPatient[patientNumber], db.PrecompRecordInput{
				ProductCode: productCode,
				JanQuantity: quantity,
			})
			importedCount++
		}

		if len(recordsByPatient) == 0 {
			http.Error(w, "No valid data found in CSV file.", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		for patientNumber, precompRecords := range recordsByPatient {
			if err := db.UpsertPreCompoundingRecordsInTx(tx, patientNumber, precompRecords); err != nil {
				http.Error(w, fmt.Sprintf("Failed to save records for patient %s: %s", patientNumber, err.Error()), http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d名の患者の予製データ（計%d件）をインポートしました。", len(recordsByPatient), importedCount),
		})
	}
}
