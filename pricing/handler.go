// C:\Users\wasab\OneDrive\デスクトップ\WASABI\pricing\handler.go

package pricing

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"

	"github.com/xuri/excelize/v2"
)

// GetExportDataHandler generates an Excel file for price quotation requests (template).
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

		var dataToExport []*model.ProductMaster
		if unregisteredOnly {
			for _, p := range allMasters {
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
			dataToExport = allMasters
		}

		f := excelize.NewFile()
		sheetName := "見積依頼"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1")

		headers := []string{"product_code", "product_name", "maker_name", "package_spec", "purchase_price"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}

		style, _ := f.NewStyle(&excelize.Style{
			NumFmt: 49, // Text
		})
		f.SetColStyle(sheetName, "A", style)

		for i, m := range dataToExport {
			rowNum := i + 2
			tempJcshms := model.JCShms{
				JC037: m.PackageForm,
				JC039: m.YjUnitName,
				JC044: m.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), m.ProductCode)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), m.ProductName)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), m.MakerName)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), formattedSpec)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), "")
		}

		dateStr := r.URL.Query().Get("date")
		fileType := "ALL"
		if unregisteredOnly {
			fileType = "UNREGISTERED"
		}
		fileName := fmt.Sprintf("価格見積依頼_%s_%s_%s.xlsx", wholesalerName, fileType, dateStr)

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)

		if err := f.Write(w); err != nil {
			http.Error(w, "Failed to write excel file", http.StatusInternalServerError)
		}
	}
}

type QuoteDataWithSpec struct {
	model.ProductMaster
	FormattedPackageSpec string             `json:"formattedPackageSpec"`
	Quotes               map[string]float64 `json:"quotes"`
}

type UploadResponse struct {
	ProductData     []QuoteDataWithSpec `json:"productData"`
	WholesalerOrder []string            `json:"wholesalerOrder"`
}

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
				continue
			}
			defer file.Close()

			buf, err := io.ReadAll(file)
			if err != nil {
				continue
			}

			f, err := excelize.OpenReader(bytes.NewReader(buf))
			if err != nil {
				continue
			}

			sheetList := f.GetSheetList()
			if len(sheetList) == 0 {
				continue
			}
			sheetName := sheetList[0]

			rows, err := f.GetRows(sheetName)
			if err != nil || len(rows) < 1 {
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
				continue
			}

			for _, row := range rows[1:] {
				if len(row) <= codeIndex || len(row) <= priceIndex {
					continue
				}

				productCode := row[codeIndex]
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

		f, err := excelize.OpenReader(file)
		if err != nil {
			http.Error(w, "Failed to read excel file: "+err.Error(), http.StatusBadRequest)
			return
		}

		sheetList := f.GetSheetList()
		if len(sheetList) == 0 {
			http.Error(w, "No sheets in the excel file", http.StatusBadRequest)
			return
		}
		rows, err := f.GetRows(sheetList[0])
		if err != nil {
			http.Error(w, "Failed to get rows from sheet: "+err.Error(), http.StatusBadRequest)
			return
		}

		var updates []model.PriceUpdate
		for i, row := range rows {
			if i == 0 {
				continue
			}
			if len(row) < 6 {
				continue
			}

			productCode := strings.TrimSpace(row[0])
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

// BackupExportHandler exports the current purchase prices and wholesalers as a backup.
func BackupExportHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products for backup export", http.StatusInternalServerError)
			return
		}

		f := excelize.NewFile()
		sheetName := "納入価・卸バックアップ"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1")

		headers := []string{"product_code", "product_name", "maker_name", "package_spec", "purchase_price", "supplier_wholesale"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}

		style, _ := f.NewStyle(&excelize.Style{NumFmt: 49}) // Text
		f.SetColStyle(sheetName, "A", style)
		f.SetColStyle(sheetName, "F", style)

		for i, m := range allMasters {
			rowNum := i + 2
			tempJcshms := model.JCShms{
				JC037: m.PackageForm,
				JC039: m.YjUnitName,
				JC044: m.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), m.ProductCode)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), m.ProductName)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), m.MakerName)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), formattedSpec)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), m.PurchasePrice)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), m.SupplierWholesale)
		}

		now := time.Now()
		fileName := fmt.Sprintf("納入価・卸バックアップ_%s.xlsx", now.Format("20060102_150405"))

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)

		if err := f.Write(w); err != nil {
			http.Error(w, "Failed to write excel file", http.StatusInternalServerError)
		}
	}
}
