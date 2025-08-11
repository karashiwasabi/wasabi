package backup

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// ExportClientsHandler handles exporting the client master to a CSV file.
func ExportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer
		sjisWriter := transform.NewWriter(&buf, japanese.ShiftJIS.NewEncoder())
		csvWriter := csv.NewWriter(sjisWriter)

		if err := csvWriter.Write([]string{"client_code", "client_name"}); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}
		for _, client := range clients {
			if err := csvWriter.Write([]string{client.Code, client.Name}); err != nil {
				http.Error(w, "Failed to write CSV row", http.StatusInternalServerError)
				return
			}
		}
		csvWriter.Flush()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="client_master.csv"`)
		w.Write(buf.Bytes())
	}
}

// ImportClientsHandler handles importing clients from a CSV file.
func ImportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		sjisReader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(sjisReader)
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

		stmt, err := tx.Prepare("INSERT OR REPLACE INTO client_master (client_code, client_name) VALUES (?, ?)")
		if err != nil {
			http.Error(w, "Failed to prepare DB statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		var importedCount int
		for i, row := range records {
			if i == 0 { // Skip header
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

// ExportProductsHandler handles exporting the product master to a CSV file.
func ExportProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Note: Using a direct DB call here as GetEditableProductMasters is for the view.
		// A new DB function could be made, but for simplicity, we query directly.
		products, err := db.GetAllProductMasters(conn) // Assuming you'll create this simple getter
		if err != nil {
			http.Error(w, "Failed to get products", http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer
		sjisWriter := transform.NewWriter(&buf, japanese.ShiftJIS.NewEncoder())
		csvWriter := csv.NewWriter(sjisWriter)

		// Updated header for WASABI schema
		header := []string{
			"product_code", "yj_code", "product_name", "origin", "kana_name", "maker_name",
			"usage_classification", "package_form", "package_spec",
			"yj_unit_name", "yj_pack_unit_qty", "flag_poison", "flag_deleterious", "flag_narcotic",
			"flag_psychotropic", "flag_stimulant", "flag_stimulant_raw", "jan_pack_inner_qty",
			"jan_unit_code", "jan_pack_unit_qty", "nhi_price", "purchase_price", "supplier_wholesale",
		}
		if err := csvWriter.Write(header); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, p := range products {
			row := []string{
				p.ProductCode, p.YjCode, p.ProductName, p.Origin, p.KanaName, p.MakerName,
				p.UsageClassification, p.PackageForm, p.PackageSpec,
				p.YjUnitName, fmt.Sprintf("%f", p.YjPackUnitQty), fmt.Sprintf("%d", p.FlagPoison),
				fmt.Sprintf("%d", p.FlagDeleterious), fmt.Sprintf("%d", p.FlagNarcotic),
				fmt.Sprintf("%d", p.FlagPsychotropic), fmt.Sprintf("%d", p.FlagStimulant),
				fmt.Sprintf("%d", p.FlagStimulantRaw), fmt.Sprintf("%f", p.JanPackInnerQty),
				fmt.Sprintf("%d", p.JanUnitCode), fmt.Sprintf("%f", p.JanPackUnitQty),
				fmt.Sprintf("%f", p.NhiPrice), fmt.Sprintf("%f", p.PurchasePrice), p.SupplierWholesale,
			}
			if err := csvWriter.Write(row); err != nil {
				http.Error(w, "Failed to write CSV row", http.StatusInternalServerError)
				return
			}
		}
		csvWriter.Flush()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="product_master.csv"`)
		w.Write(buf.Bytes())
	}
}

// ImportProductsHandler handles importing products from a CSV file.
func ImportProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		sjisReader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		csvReader := csv.NewReader(sjisReader)
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

		var importedCount int
		for i, row := range records {
			if i == 0 { // Skip header
				continue
			}
			if len(row) < 23 { // Ensure row has enough columns for WASABI schema
				continue
			}

			// Parse values from row based on new WASABI CSV format
			yjPackUnitQty, _ := strconv.ParseFloat(row[10], 64)
			flagPoison, _ := strconv.Atoi(row[11])
			flagDeleterious, _ := strconv.Atoi(row[12])
			flagNarcotic, _ := strconv.Atoi(row[13])
			flagPsychotropic, _ := strconv.Atoi(row[14])
			flagStimulant, _ := strconv.Atoi(row[15])
			flagStimulantRaw, _ := strconv.Atoi(row[16])
			janPackInnerQty, _ := strconv.ParseFloat(row[17], 64)
			janUnitCode, _ := strconv.Atoi(row[18])
			janPackUnitQty, _ := strconv.ParseFloat(row[19], 64)
			nhiPrice, _ := strconv.ParseFloat(row[20], 64)
			purchasePrice, _ := strconv.ParseFloat(row[21], 64)

			input := model.ProductMasterInput{
				ProductCode: row[0], YjCode: row[1], ProductName: row[2], Origin: row[3], KanaName: row[4],
				MakerName: row[5], UsageClassification: row[6], PackageForm: row[7], PackageSpec: row[8],
				YjUnitName: row[9], YjPackUnitQty: yjPackUnitQty, FlagPoison: flagPoison,
				FlagDeleterious: flagDeleterious, FlagNarcotic: flagNarcotic, FlagPsychotropic: flagPsychotropic,
				FlagStimulant: flagStimulant, FlagStimulantRaw: flagStimulantRaw,
				JanPackInnerQty: janPackInnerQty, JanUnitCode: janUnitCode, JanPackUnitQty: janPackUnitQty,
				NhiPrice: nhiPrice, PurchasePrice: purchasePrice, SupplierWholesale: row[22],
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
