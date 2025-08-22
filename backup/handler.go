// C:\Dev\WASABI\backup\handler.go

package backup

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

// (ExportClientsHandler と ImportClientsHandler は変更なし)
func ExportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="client_master.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF})

		csvWriter := csv.NewWriter(w)

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
	}
}
func ImportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		csvReader := csv.NewReader(file)
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

// ExportProductsHandler handles exporting the product master to a CSV file.
func ExportProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetEditableProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="product_master_editable.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)

		// ▼▼▼ [修正点] headerから "package_spec" を削除 ▼▼▼
		header := []string{
			"product_code", "yj_code", "product_name", "origin", "kana_name", "maker_name",
			"usage_classification", "package_form", // "package_spec" を削除
			"yj_unit_name", "yj_pack_unit_qty", "flag_poison", "flag_deleterious", "flag_narcotic",
			"flag_psychotropic", "flag_stimulant", "flag_stimulant_raw", "jan_pack_inner_qty",
			"jan_unit_code", "jan_pack_unit_qty", "nhi_price", "purchase_price", "supplier_wholesale",
		}
		// ▲▲▲ 修正ここまで ▲▲▲
		if err := csvWriter.Write(header); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, p := range products {
			// ▼▼▼ [修正点] rowから重複していた p.PackageForm を削除 ▼▼▼
			row := []string{
				p.ProductCode, p.YjCode, p.ProductName, p.Origin, p.KanaName, p.MakerName,
				p.UsageClassification, p.PackageForm, // p.PackageForm は1つだけ
				p.YjUnitName, fmt.Sprintf("%f", p.YjPackUnitQty), fmt.Sprintf("%d", p.FlagPoison),
				fmt.Sprintf("%d", p.FlagDeleterious), fmt.Sprintf("%d", p.FlagNarcotic),
				fmt.Sprintf("%d", p.FlagPsychotropic), fmt.Sprintf("%d", p.FlagStimulant),
				fmt.Sprintf("%d", p.FlagStimulantRaw), fmt.Sprintf("%f", p.JanPackInnerQty),
				fmt.Sprintf("%d", p.JanUnitCode), fmt.Sprintf("%f", p.JanPackUnitQty),
				fmt.Sprintf("%f", p.NhiPrice), fmt.Sprintf("%f", p.PurchasePrice), p.SupplierWholesale,
			}
			// ▲▲▲ 修正ここまで ▲▲▲
			if err := csvWriter.Write(row); err != nil {
				http.Error(w, "Failed to write CSV row", http.StatusInternalServerError)
				return
			}
		}
		csvWriter.Flush()
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

		csvReader := csv.NewReader(file)
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
			if i == 0 {
				continue
			}
			// ▼▼▼ [修正点] 列数を23から22に変更 ▼▼▼
			if len(row) < 22 {
				continue
			}
			// ▲▲▲ 修正ここまで ▲▲▲

			// ▼▼▼ [修正点] 全ての列インデックスを8列目以降について1つずつずらす ▼▼▼
			yjPackUnitQty, _ := strconv.ParseFloat(row[9], 64)    // 10 -> 9
			flagPoison, _ := strconv.Atoi(row[10])                // 11 -> 10
			flagDeleterious, _ := strconv.Atoi(row[11])           // 12 -> 11
			flagNarcotic, _ := strconv.Atoi(row[12])              // 13 -> 12
			flagPsychotropic, _ := strconv.Atoi(row[13])          // 14 -> 13
			flagStimulant, _ := strconv.Atoi(row[14])             // 15 -> 14
			flagStimulantRaw, _ := strconv.Atoi(row[15])          // 16 -> 15
			janPackInnerQty, _ := strconv.ParseFloat(row[16], 64) // 17 -> 16
			janUnitCode, _ := strconv.Atoi(row[17])               // 18 -> 17
			janPackUnitQty, _ := strconv.ParseFloat(row[18], 64)  // 19 -> 18
			nhiPrice, _ := strconv.ParseFloat(row[19], 64)        // 20 -> 19
			purchasePrice, _ := strconv.ParseFloat(row[20], 64)   // 21 -> 20

			input := model.ProductMasterInput{
				ProductCode: row[0], YjCode: row[1], ProductName: row[2], Origin: row[3], KanaName: row[4],
				MakerName: row[5], UsageClassification: row[6], PackageForm: row[7],
				YjUnitName: row[8], YjPackUnitQty: yjPackUnitQty, FlagPoison: flagPoison, // 9 -> 8
				FlagDeleterious: flagDeleterious, FlagNarcotic: flagNarcotic, FlagPsychotropic: flagPsychotropic,
				FlagStimulant: flagStimulant, FlagStimulantRaw: flagStimulantRaw,
				JanPackInnerQty: janPackInnerQty, JanUnitCode: janUnitCode, JanPackUnitQty: janPackUnitQty,
				NhiPrice: nhiPrice, PurchasePrice: purchasePrice, SupplierWholesale: row[21], // 22 -> 21
			}
			// ▲▲▲ 修正ここまで ▲▲▲

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
