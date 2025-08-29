// C:\Users\wasab\OneDrive\デスクトップ\WASABI\backup\handler.go

package backup

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"

	"github.com/xuri/excelize/v2" // Corrected import path
)

// ExportClientsHandler handles exporting the client master to an Excel file.
func ExportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}

		f := excelize.NewFile()
		sheetName := "得意先マスター"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1")

		headers := []string{"client_code", "client_name"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}

		style, _ := f.NewStyle(&excelize.Style{
			NumFmt: 49, // Text format
		})
		f.SetColStyle(sheetName, "A", style)

		for i, client := range clients {
			rowNum := i + 2
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), client.Code)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), client.Name)
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="client_master.xlsx"`)

		if err := f.Write(w); err != nil {
			http.Error(w, "Failed to write excel file", http.StatusInternalServerError)
		}
	}
}

// ImportClientsHandler handles importing clients from an Excel file.
func ImportClientsHandler(conn *sql.DB) http.HandlerFunc {
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
			http.Error(w, "No sheets found in the excel file", http.StatusBadRequest)
			return
		}
		rows, err := f.GetRows(sheetList[0])
		if err != nil {
			http.Error(w, "Failed to get rows from sheet: "+err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare("INSERT OR REPLACE INTO client_master (client_code, client_name) VALUES (?, ?)")
		if err != nil {
			http.Error(w, "Failed to prepare DB statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		var importedCount int
		for i, row := range rows {
			if i == 0 {
				continue
			}
			if len(row) < 2 {
				continue
			}
			if _, err := stmt.Exec(row[0], row[1]); err != nil {
				log.Printf("Failed to import client row %d: %v", i+1, err)
				http.Error(w, fmt.Sprintf("Failed to import client row %d", i+1), http.StatusInternalServerError)
				return
			}
			importedCount++
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		if err := db.InitializeSequenceFromMaxClientCode(conn); err != nil {
			log.Printf("Warning: failed to re-initialize client sequence after import: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の得意先をインポートしました。", importedCount),
		})
	}
}

// ExportProductsHandler handles exporting editable products to an Excel file.
func ExportProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetEditableProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products", http.StatusInternalServerError)
			return
		}

		f := excelize.NewFile()
		sheetName := "製品マスター"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1")

		header := []string{
			"product_code", "yj_code", "product_name", "origin", "kana_name", "maker_name",
			"usage_classification", "package_form",
			"yj_unit_name", "yj_pack_unit_qty", "flag_poison", "flag_deleterious", "flag_narcotic",
			"flag_psychotropic", "flag_stimulant", "flag_stimulant_raw", "jan_pack_inner_qty",
			"jan_unit_code", "jan_pack_unit_qty", "nhi_price", "purchase_price", "supplier_wholesale",
		}
		for i, h := range header {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}

		style, _ := f.NewStyle(&excelize.Style{
			NumFmt: 49, // Text format
		})
		f.SetColStyle(sheetName, "A", style)

		for i, p := range products {
			rowNum := i + 2
			row := []interface{}{
				p.ProductCode, p.YjCode, p.ProductName, p.Origin, p.KanaName, p.MakerName,
				p.UsageClassification, p.PackageForm,
				p.YjUnitName, p.YjPackUnitQty, p.FlagPoison,
				p.FlagDeleterious, p.FlagNarcotic,
				p.FlagPsychotropic, p.FlagStimulant,
				p.FlagStimulantRaw, p.JanPackInnerQty,
				p.JanUnitCode, p.JanPackUnitQty,
				p.NhiPrice, p.PurchasePrice, p.SupplierWholesale,
			}
			for j, val := range row {
				cell, _ := excelize.CoordinatesToCellName(j+1, rowNum)
				f.SetCellValue(sheetName, cell, val)
			}
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="product_master_editable.xlsx"`)

		if err := f.Write(w); err != nil {
			http.Error(w, "Failed to write excel file", http.StatusInternalServerError)
		}
	}
}

// ImportProductsHandler handles importing the product master from a CSV file.
func ImportProductsHandler(conn *sql.DB) http.HandlerFunc {
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

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var importedCount int
		for i, row := range rows {
			if i == 0 {
				continue
			}
			if len(row) < 22 {
				continue
			}

			yjPackUnitQty, _ := strconv.ParseFloat(row[9], 64)
			flagPoison, _ := strconv.Atoi(row[10])
			flagDeleterious, _ := strconv.Atoi(row[11])
			flagNarcotic, _ := strconv.Atoi(row[12])
			flagPsychotropic, _ := strconv.Atoi(row[13])
			flagStimulant, _ := strconv.Atoi(row[14])
			flagStimulantRaw, _ := strconv.Atoi(row[15])
			janPackInnerQty, _ := strconv.ParseFloat(row[16], 64)
			janUnitCode, _ := strconv.Atoi(row[17])
			janPackUnitQty, _ := strconv.ParseFloat(row[18], 64)
			nhiPrice, _ := strconv.ParseFloat(row[19], 64)
			purchasePrice, _ := strconv.ParseFloat(row[20], 64)

			input := model.ProductMasterInput{
				ProductCode:         strings.TrimSpace(row[0]),
				YjCode:              strings.TrimSpace(row[1]),
				ProductName:         strings.TrimSpace(row[2]),
				Origin:              strings.TrimSpace(row[3]),
				KanaName:            strings.TrimSpace(row[4]),
				MakerName:           strings.TrimSpace(row[5]),
				UsageClassification: strings.TrimSpace(row[6]),
				PackageForm:         strings.TrimSpace(row[7]),
				YjUnitName:          strings.TrimSpace(row[8]),
				YjPackUnitQty:       yjPackUnitQty,
				FlagPoison:          flagPoison,
				FlagDeleterious:     flagDeleterious,
				FlagNarcotic:        flagNarcotic,
				FlagPsychotropic:    flagPsychotropic,
				FlagStimulant:       flagStimulant,
				FlagStimulantRaw:    flagStimulantRaw,
				JanPackInnerQty:     janPackInnerQty,
				JanUnitCode:         janUnitCode,
				JanPackUnitQty:      janPackUnitQty,
				NhiPrice:            nhiPrice,
				PurchasePrice:       purchasePrice,
				SupplierWholesale:   strings.TrimSpace(row[21]),
			}

			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				log.Printf("Failed to import product row %d: %v", i+1, err)
				http.Error(w, fmt.Sprintf("Failed to import product row %d", i+1), http.StatusInternalServerError)
				return
			}
			importedCount++
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		if err := db.InitializeSequenceFromMaxYjCode(conn); err != nil {
			log.Printf("Warning: failed to re-initialize YJ sequence after import: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の製品をインポートしました。", importedCount),
		})
	}
}
