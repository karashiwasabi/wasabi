package pricing

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// ExportData is the struct for CSV export data
type ExportData struct {
	ProductCode   string `json:"productCode"`
	ProductName   string `json:"productName"`
	MakerName     string `json:"makerName"`
	PackageSpec   string `json:"packageSpec"`
	PurchasePrice string `json:"purchasePrice"`
}

// GetExportDataHandler は価格見積もり依頼用のCSVの元データをJSONで返します。
func GetExportDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`SELECT ` + db.SelectColumns + ` FROM product_master ORDER BY kana_name`)
		if err != nil {
			http.Error(w, "Failed to get products for export", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var exportData []ExportData
		for rows.Next() {
			m, err := db.ScanProductMaster(rows)
			if err != nil {
				continue
			}

			// ▼▼▼ [修正点] 組み立て包装の生成に必要なデータを全て渡すように修正 ▼▼▼
			tempJcshms := model.JCShms{
				JC037: m.PackageForm,
				JC039: m.YjUnitName,
				JC044: m.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
			}
			// ▲▲▲ 修正ここまで ▲▲▲
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			exportData = append(exportData, ExportData{
				ProductCode:   m.ProductCode,
				ProductName:   m.ProductName,
				MakerName:     m.MakerName,
				PackageSpec:   formattedSpec,
				PurchasePrice: "", // 空欄
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportData)
	}
}

// QuoteDataWithSpec は見積価格と比較データを保持する構造体です。
type QuoteDataWithSpec struct {
	model.ProductMaster
	FormattedPackageSpec string             `json:"formattedPackageSpec"`
	Quotes               map[string]float64 `json:"quotes"`
}

// UploadResponse は卸からの見積もりCSVアップロードに対するレスポンスの構造体です。
type UploadResponse struct {
	ProductData     []QuoteDataWithSpec `json:"productData"`
	WholesalerOrder []string            `json:"wholesalerOrder"`
}

// UploadQuotesHandler は卸からの見積もりCSVをアップロードして解析し、比較用データを返します。
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

			rd := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
			csvReader := csv.NewReader(rd)
			csvReader.LazyQuotes = true
			csvReader.FieldsPerRecord = -1

			header, err := csvReader.Read()
			if err != nil {
				continue
			}

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

			for {
				row, err := csvReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil || len(row) <= codeIndex || len(row) <= priceIndex {
					continue
				}

				productCode := strings.TrimSpace(row[codeIndex])
				priceStr := strings.ReplaceAll(strings.TrimSpace(row[priceIndex]), ",", "") // カンマを除去
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
			// ▼▼▼ [修正点] 組み立て包装の生成に必要なデータを全て渡すように修正 ▼▼▼
			tempJcshms := model.JCShms{
				JC037: master.PackageForm,
				JC039: master.YjUnitName,
				JC044: master.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
			}
			// ▲▲▲ 修正ここまで ▲▲▲
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

// BulkUpdateHandler は選択された納入価格と卸でマスターを一括更新します。
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

// GetAllMastersForPricingHandler は、価格比較画面の初期表示用に全マスターデータを返します。
func GetAllMastersForPricingHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get all product masters for pricing", http.StatusInternalServerError)
			return
		}

		// ▼▼▼ [修正点] 以下の重複した型定義を削除します ▼▼▼
		/*
			type QuoteDataWithSpec struct {
				model.ProductMaster
				FormattedPackageSpec string             `json:"formattedPackageSpec"`
				Quotes               map[string]float64 `json:"quotes"`
			}
		*/
		// ▲▲▲ 修正ここまで ▲▲▲

		responseData := make([]QuoteDataWithSpec, 0, len(allMasters))
		for _, master := range allMasters {
			// ▼▼▼ [修正点] 組み立て包装の生成に必要なデータを全て渡すように修正 ▼▼▼
			tempJcshms := model.JCShms{
				JC037: master.PackageForm,
				JC039: master.YjUnitName,
				JC044: master.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
			}
			// ▲▲▲ 修正ここまで ▲▲▲
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			responseData = append(responseData, QuoteDataWithSpec{
				ProductMaster:        *master,
				FormattedPackageSpec: formattedSpec,
				Quotes:               make(map[string]float64), // 初期表示では見積もりは空
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responseData)
	}
}