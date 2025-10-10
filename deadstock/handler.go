package deadstock

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func GetDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		now := time.Now()
		endDate := "99999999"
		startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)

		filters := model.DeadStockFilters{
			StartDate:        startDate.Format("20060102"),
			EndDate:          endDate,
			ExcludeZeroStock: q.Get("excludeZeroStock") == "true",
			Coefficient:      coefficient,
			KanaName:         q.Get("kanaName"),
			DosageForm:       q.Get("dosageForm"),
		}

		results, err := db.GetDeadStockList(conn, filters)
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

		productCodesMap := make(map[string]struct{})
		for _, rec := range payload {
			if rec.ProductCode != "" {
				productCodesMap[rec.ProductCode] = struct{}{}
			}
		}
		var productCodes []string
		for code := range productCodesMap {
			productCodes = append(productCodes, code)
		}

		if len(productCodes) > 0 {
			if err := db.DeleteDeadStockByProductCodesInTx(tx, productCodes); err != nil {
				http.Error(w, "Failed to delete old dead stock records: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := db.SaveDeadStockListInTx(tx, payload); err != nil {
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

func ImportDeadStockHandler(conn *sql.DB) http.HandlerFunc {
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
		rows, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file: "+err.Error(), http.StatusBadRequest)
			return
		}

		var payload []model.DeadStockRecord
		productCodesMap := make(map[string]struct{})

		for i, row := range rows {
			if i == 0 || len(row) < 9 {
				continue
			}

			quantity, _ := strconv.ParseFloat(row[3], 64)
			if quantity <= 0 {
				continue
			}
			janPackInnerQty, _ := strconv.ParseFloat(row[8], 64)
			productCode := strings.Trim(strings.TrimSpace(row[1]), `="`)

			rec := model.DeadStockRecord{
				YjCode:           strings.Trim(strings.TrimSpace(row[0]), `="`),
				ProductCode:      productCode,
				StockQuantityJan: quantity,
				YjUnitName:       strings.TrimSpace(row[4]),
				ExpiryDate:       strings.TrimSpace(row[5]),
				LotNumber:        strings.TrimSpace(row[6]),
				PackageForm:      strings.TrimSpace(row[7]),
				JanPackInnerQty:  janPackInnerQty,
			}
			payload = append(payload, rec)
			if productCode != "" {
				productCodesMap[productCode] = struct{}{}
			}
		}

		if len(payload) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "インポートする有効なデータがありませんでした。"})
			return
		}

		var productCodes []string
		for code := range productCodesMap {
			productCodes = append(productCodes, code)
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if len(productCodes) > 0 {
			if err := db.DeleteDeadStockByProductCodesInTx(tx, productCodes); err != nil {
				http.Error(w, "Failed to delete old dead stock records: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := db.SaveDeadStockListInTx(tx, payload); err != nil {
			http.Error(w, "Failed to save imported dead stock records: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件のロット・期限情報をインポートしました。", len(payload)),
		})
	}
}

// ▼▼▼【ここから追加】▼▼▼
// ExportDeadStockHandler は画面のフィルタ条件に基づいて不動在庫リストをCSV形式でエクスポートします。
func ExportDeadStockHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		now := time.Now()
		startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)

		filters := model.DeadStockFilters{
			StartDate:        startDate.Format("20060102"),
			EndDate:          "99999999",
			ExcludeZeroStock: q.Get("excludeZeroStock") == "true",
			KanaName:         q.Get("kanaName"),
			DosageForm:       q.Get("dosageForm"),
		}

		results, err := db.GetDeadStockList(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get dead stock list for export: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fileName := fmt.Sprintf("不動在庫リスト_%s.csv", now.Format("20060102"))
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		headers := []string{
			"yj_code", "product_code", "product_name", "stock_quantity",
			"yj_unit_name", "expiry_date", "lot_number", "package_form", "jan_pack_inner_qty",
		}
		if err := csvWriter.Write(headers); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, group := range results {
			for _, pkg := range group.PackageGroups {
				for _, prod := range pkg.Products {
					// 保存済みのロット・期限情報がある場合は、そのレコードごとに出力
					if len(prod.SavedRecords) > 0 {
						for _, rec := range prod.SavedRecords {
							record := []string{
								prod.YjCode,
								fmt.Sprintf("=%q", prod.ProductCode),
								prod.ProductName,
								strconv.FormatFloat(rec.StockQuantityJan, 'f', -1, 64),
								prod.YjUnitName,
								rec.ExpiryDate,
								rec.LotNumber,
								prod.PackageForm,
								strconv.FormatFloat(prod.JanPackInnerQty, 'f', -1, 64),
							}
							if err := csvWriter.Write(record); err != nil {
								log.Printf("Failed to write dead stock row to CSV (Code: %s): %v", prod.ProductCode, err)
							}
						}
					} else {
						// 保存済みのロット・期限情報がない場合は、製品情報と現在の理論在庫を出力
						record := []string{
							prod.YjCode,
							fmt.Sprintf("=%q", prod.ProductCode),
							prod.ProductName,
							strconv.FormatFloat(prod.CurrentStock, 'f', -1, 64),
							prod.YjUnitName,
							"", // expiry_date
							"", // lot_number
							prod.PackageForm,
							strconv.FormatFloat(prod.JanPackInnerQty, 'f', -1, 64),
						}
						if err := csvWriter.Write(record); err != nil {
							log.Printf("Failed to write dead stock row to CSV (Code: %s): %v", prod.ProductCode, err)
						}
					}
				}
			}
		}
	}
}

// ▲▲▲【追加ここまで】▲▲▲
