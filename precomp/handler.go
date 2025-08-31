package precomp

import (
	"database/sql"
	"encoding/csv" // encoding/csvを追加
	"encoding/json"
	"fmt" // fmtを追加
	"log"
	"net/http"
	"strconv" // strconvを追加
	"wasabi/db"

	"os"
	"path/filepath"

	"golang.org/x/text/encoding/japanese" // transformを追加
	"golang.org/x/text/transform"
)

type PrecompPayload struct {
	PatientNumber string                  `json:"patientNumber"`
	Records       []db.PrecompRecordInput `json:"records"`
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

		if err := db.UpsertPreCompoundingRecordsInTx(tx, payload.PatientNumber, payload.Records); err != nil {
			log.Printf("ERROR: Failed to save pre-compounding records for patient %s: %v", payload.PatientNumber, err) // この行を追加
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

func LoadPrecompHandler(conn *sql.DB) http.HandlerFunc {
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

		// The simple list view does not require a complex view model, so we return the records directly.
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
			http.Error(w, "Failed to clear pre-compounding records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "予製データを中断しました。"})
	}
}

// ▼▼▼【ここから追加】▼▼▼

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

// ImportPrecompHandler はCSVから予製リストをインポートします。
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

		// Shift_JISからUTF-8への変換
		reader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(reader)
		csvReader.LazyQuotes = true

		records, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}

		var precompRecords []db.PrecompRecordInput
		// ヘッダー行をスキップ (i=1から開始)
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

// ▲▲▲【追加ここまで】▲▲▲
// ▼▼▼ [修正点] 以下の関数をファイル末尾に追加 ▼▼▼
// ExportAllPrecompHandler は全患者の予製リストをCSVとしてエクスポートします。
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

		// ヘッダーに「患者番号」を追加
		header := []string{"patient_number", "product_code", "product_name", "quantity_jan", "unit_name"}
		if err := csvWriter.Write(header); err != nil {
			log.Printf("Failed to write CSV header for all precomp export: %v", err)
			return
		}

		// データに患者番号(ClientCode)を追加して書き込み
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

// ▲▲▲ 修正ここまで ▲▲▲
// ▼▼▼【ここから追加】▼▼▼
// BulkImportPrecompHandler は複数患者の予製リストをCSVから一括でインポートします。
func BulkImportPrecompHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Shift_JISからUTF-8への変換
		reader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(reader)
		csvReader.LazyQuotes = true

		records, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}

		// 患者番号ごとにレコードをグループ化
		recordsByPatient := make(map[string][]db.PrecompRecordInput)
		var importedCount int
		// ヘッダー行をスキップ (i=1から開始)
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

		// 患者ごとにUpsert処理を実行
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

// ▲▲▲【追加ここまで】▲▲▲
