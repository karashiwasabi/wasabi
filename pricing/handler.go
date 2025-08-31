// C:\Users\wasab\OneDrive\デスクトップ\WASABI\pricing\handler.go

package pricing

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
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// ▼▼▼ [ここから修正] GetExportDataHandler内のフィルタリングロジックを変更 ▼▼▼
// GetExportDataHandler generates a CSV file for price quotation requests (template).
func GetExportDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wholesalerName := r.URL.Query().Get("wholesalerName")
		unregisteredOnlyStr := r.URL.Query().Get("unregisteredOnly")
		unregisteredOnly := unregisteredOnlyStr == "true"

		if wholesalerName == "" {
			http.Error(w, "Wholesaler name is required", http.StatusBadRequest)
			return
		}

		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products for export", http.StatusInternalServerError)
			return
		}

		// --- ここからが新しいフィルタリング処理 ---
		var mastersToProcess []*model.ProductMaster
		for _, p := range allMasters {
			// product_codeが'99999'で始まり、かつ長さが13文字を超える仮コードは除外する
			if strings.HasPrefix(p.ProductCode, "99999") && len(p.ProductCode) > 13 {
				continue // スキップ
			}
			mastersToProcess = append(mastersToProcess, p)
		}
		// --- フィルタリング処理ここまで ---

		var dataToExport []*model.ProductMaster
		if unregisteredOnly {
			for _, p := range mastersToProcess { // allMastersの代わりにフィルタ済みのmastersToProcessを使用
				if p.SupplierWholesale == "" {
					uc := p.UsageClassification
					if uc == "機" || uc == "他" || ((uc == "内" || uc == "外" || uc == "歯" || uc == "注") && p.Origin == "JCSHMS") {
						dataToExport = append(dataToExport, p)
					}
				}
			}
			if len(dataToExport) == 0 {
				http.Error(w, "Export target not found", http.StatusNotFound)
				return
			}
		} else {
			dataToExport = mastersToProcess // allMastersの代わりにフィルタ済みのmastersToProcessを使用
		}

		dateStr := r.URL.Query().Get("date")
		fileType := "ALL"
		if unregisteredOnly {
			fileType = "UNREGISTERED"
		}
		fileName := fmt.Sprintf("価格見積依頼_%s_%s_%s.csv", wholesalerName, fileType, dateStr)

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		headers := []string{"product_code", "product_name", "maker_name", "package_spec", "purchase_price"}
		csvWriter.Write(headers)

		for _, m := range dataToExport {
			tempJcshms := model.JCShms{
				JC037: m.PackageForm,
				JC039: m.YjUnitName,
				JC044: m.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			record := []string{
				fmt.Sprintf("=%q", m.ProductCode),
				m.ProductName,
				m.MakerName,
				formattedSpec,
				"", // purchase_price is left empty
			}
			csvWriter.Write(record)
		}
	}
}

// ▲▲▲ [修正ここまで] ▲▲▲

type QuoteDataWithSpec struct {
	model.ProductMaster
	FormattedPackageSpec string             `json:"formattedPackageSpec"`
	Quotes               map[string]float64 `json:"quotes"`
}

type UploadResponse struct {
	ProductData     []QuoteDataWithSpec `json:"productData"`
	WholesalerOrder []string            `json:"wholesalerOrder"`
}

// UploadQuotesHandler handles uploading quote CSV files from wholesalers.
func UploadQuotesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "File upload error", http.StatusBadRequest)
			return
		}

		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get all product masters", http.StatusInternalServerError)
			return
		}

		quotesByProduct := make(map[string]map[string]float64)
		wholesalerFiles := r.MultipartForm.File["files"]
		wholesalerNames := r.MultipartForm.Value["wholesalerNames"]

		if len(wholesalerFiles) != len(wholesalerNames) {
			http.Error(w, "File and wholesaler name mismatch", http.StatusBadRequest)
			return
		}

		for i, fileHeader := range wholesalerFiles {
			wholesalerName := wholesalerNames[i]
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("WARN: could not open uploaded file for %s: %v", wholesalerName, err)
				continue
			}
			defer file.Close()

			csvReader := csv.NewReader(file)
			csvReader.LazyQuotes = true
			rows, err := csvReader.ReadAll()
			if err != nil || len(rows) < 1 {
				log.Printf("WARN: could not parse CSV for %s: %v", wholesalerName, err)
				continue
			}

			header := rows[0]
			codeIndex, priceIndex := -1, -1
			for i, h := range header {
				if h == "product_code" {
					codeIndex = i
				}
				if h == "purchase_price" {
					priceIndex = i
				}
			}

			if codeIndex == -1 || priceIndex == -1 {
				log.Printf("WARN: required columns not found in file from %s", wholesalerName)
				continue
			}

			for _, row := range rows[1:] {
				if len(row) <= codeIndex || len(row) <= priceIndex {
					continue
				}

				productCode := strings.Trim(strings.TrimSpace(row[codeIndex]), `="`)
				priceStr := row[priceIndex]

				if productCode == "" || priceStr == "" {
					continue
				}

				price, err := strconv.ParseFloat(priceStr, 64)
				if err != nil {
					continue
				}

				if _, ok := quotesByProduct[productCode]; !ok {
					quotesByProduct[productCode] = make(map[string]float64)
				}
				quotesByProduct[productCode][wholesalerName] = price
			}
		}

		var responseData []QuoteDataWithSpec
		for _, master := range allMasters {
			tempJcshms := model.JCShms{
				JC037: master.PackageForm,
				JC039: master.YjUnitName,
				JC044: master.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			responseData = append(responseData, QuoteDataWithSpec{
				ProductMaster:        *master,
				FormattedPackageSpec: formattedSpec,
				Quotes:               quotesByProduct[master.ProductCode],
			})
		}

		finalResponse := UploadResponse{
			ProductData:     responseData,
			WholesalerOrder: wholesalerNames,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(finalResponse)
	}
}

func BulkUpdateHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []model.PriceUpdate
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

		if err := db.UpdatePricesAndSuppliersInTx(tx, payload); err != nil {
			http.Error(w, "Failed to update prices: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の医薬品マスターを更新しました。", len(payload)),
		})
	}
}

func GetAllMastersForPricingHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get all product masters for pricing", http.StatusInternalServerError)
			return
		}

		responseData := make([]QuoteDataWithSpec, 0, len(allMasters))
		for _, master := range allMasters {
			tempJcshms := model.JCShms{
				JC037: master.PackageForm,
				JC039: master.YjUnitName,
				JC044: master.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			responseData = append(responseData, QuoteDataWithSpec{
				ProductMaster:        *master,
				FormattedPackageSpec: formattedSpec,
				Quotes:               make(map[string]float64),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responseData)
	}
}

func DirectImportHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		csvReader := csv.NewReader(file)
		csvReader.LazyQuotes = true
		rows, err := csvReader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to parse CSV file: "+err.Error(), http.StatusBadRequest)
			return
		}

		var updates []model.PriceUpdate
		for i, row := range rows {
			if i == 0 { // Skip header
				continue
			}
			if len(row) < 6 { // Expecting at least 6 columns
				continue
			}

			productCode := strings.Trim(strings.TrimSpace(row[0]), `="`)
			priceStr := strings.TrimSpace(row[4])
			supplierCode := strings.TrimSpace(row[5])

			if productCode == "" || priceStr == "" || supplierCode == "" {
				continue
			}

			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				continue
			}

			updates = append(updates, model.PriceUpdate{
				ProductCode:      productCode,
				NewPurchasePrice: price,
				NewSupplier:      supplierCode,
			})
		}

		if len(updates) == 0 {
			http.Error(w, "No valid data to import found in the file.", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.UpdatePricesAndSuppliersInTx(tx, updates); err != nil {
			http.Error(w, "Failed to update prices: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の納入価と卸情報を更新しました。", len(updates)),
		})
	}
}

// BackupExportHandler exports the current purchase prices and wholesalers as a backup CSV.
func BackupExportHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products for backup export", http.StatusInternalServerError)
			return
		}

		now := time.Now()
		fileName := fmt.Sprintf("納入価・卸バックアップ_%s.csv", now.Format("20060102_150405"))
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		headers := []string{"product_code", "product_name", "maker_name", "package_spec", "purchase_price", "supplier_wholesale"}
		csvWriter.Write(headers)

		for _, m := range allMasters {
			tempJcshms := model.JCShms{
				JC037: m.PackageForm,
				JC039: m.YjUnitName,
				JC044: m.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			record := []string{
				fmt.Sprintf("=%q", m.ProductCode),
				m.ProductName,
				m.MakerName,
				formattedSpec,
				strconv.FormatFloat(m.PurchasePrice, 'f', 2, 64),
				m.SupplierWholesale,
			}
			csvWriter.Write(record)
		}
	}
}
