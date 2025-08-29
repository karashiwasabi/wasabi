// C:\Users\wasab\OneDrive\デスクトップ\WASABI\deadstock\handler.go

package deadstock

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

func GetDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5
		}

		filters := model.DeadStockFilters{
			StartDate:        q.Get("startDate"),
			EndDate:          q.Get("endDate"),
			ExcludeZeroStock: q.Get("excludeZeroStock") == "true",
			Coefficient:      coefficient,
			KanaName:         q.Get("kanaName"),
			DosageForm:       q.Get("dosageForm"),
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		results, err := db.GetDeadStockList(tx, filters)
		if err != nil {
			http.Error(w, "Failed to get dead stock list: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func SaveDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []model.DeadStockRecord
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.UpsertDeadStockRecordsInTx(tx, payload); err != nil {
			http.Error(w, "Failed to save dead stock records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "保存しました。"})
	}
}

// ImportDeadStockHandler handles importing dead stock records from a CSV file.
func ImportDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// ▼▼▼【ここが修正箇所です】▼▼▼
		// ファイルはUTF-8であることを前提とし、Shift_JISからの文字コード変換処理を削除します。
		csvReader := csv.NewReader(file)
		// ▲▲▲【修正ここまで】▲▲▲

		csvReader.LazyQuotes = true
		records, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var deadStockRecords []model.DeadStockRecord
		var importedCount int
		for i, row := range records {
			if i == 0 { // Skip header
				continue
			}
			if len(row) < 9 {
				continue
			}

			quantity, _ := strconv.ParseFloat(row[3], 64)
			janPackInnerQty, _ := strconv.ParseFloat(row[8], 64)

			if quantity <= 0 {
				continue
			}

			dsRecord := model.DeadStockRecord{
				YjCode:           row[0],
				ProductCode:      row[1],
				StockQuantityJan: quantity,
				YjUnitName:       row[4],
				ExpiryDate:       row[5],
				LotNumber:        row[6],
				PackageForm:      row[7],
				JanPackInnerQty:  janPackInnerQty,
			}
			deadStockRecords = append(deadStockRecords, dsRecord)
			importedCount++
		}

		if err := db.UpsertDeadStockRecordsInTx(tx, deadStockRecords); err != nil {
			http.Error(w, "Failed to save dead stock records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件のデッドストック情報をインポートしました。", importedCount),
		})
	}
}
