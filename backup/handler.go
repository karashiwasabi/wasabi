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
)

// ▼▼▼【ここから追加】▼▼▼

// ExportCustomersHandler は得意先と卸業者の両方を1つのCSVファイルに出力します。
func ExportCustomersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}

		wholesalers, err := db.GetAllWholesalers(conn)
		if err != nil {
			http.Error(w, "Failed to get wholesalers", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="customer_master.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		headers := []string{"種別", "コード", "名称"}
		if err := csvWriter.Write(headers); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		// 得意先を書き込み
		for _, client := range clients {
			record := []string{
				"得意先",
				fmt.Sprintf("=%q", client.Code),
				client.Name,
			}
			if err := csvWriter.Write(record); err != nil {
				log.Printf("Failed to write client row to CSV (Code: %s): %v", client.Code, err)
			}
		}

		// 卸業者を書き込み
		for _, wholesaler := range wholesalers {
			record := []string{
				"卸業者",
				fmt.Sprintf("=%q", wholesaler.Code),
				wholesaler.Name,
			}
			if err := csvWriter.Write(record); err != nil {
				log.Printf("Failed to write wholesaler row to CSV (Code: %s): %v", wholesaler.Code, err)
			}
		}
	}
}

// ImportCustomersHandler は得意先と卸業者の両方を含むCSVファイルをインポートします。
func ImportCustomersHandler(conn *sql.DB) http.HandlerFunc {
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

		clientStmt, err := tx.Prepare("INSERT OR REPLACE INTO client_master (client_code, client_name) VALUES (?, ?)")
		if err != nil {
			http.Error(w, "Failed to prepare client DB statement", http.StatusInternalServerError)
			return
		}
		defer clientStmt.Close()

		wholesalerStmt, err := tx.Prepare("INSERT OR REPLACE INTO wholesalers (wholesaler_code, wholesaler_name) VALUES (?, ?)")
		if err != nil {
			http.Error(w, "Failed to prepare wholesaler DB statement", http.StatusInternalServerError)
			return
		}
		defer wholesalerStmt.Close()

		var clientCount, wholesalerCount int
		for i, row := range rows {
			if i == 0 || len(row) < 3 { // Skip header or short rows
				continue
			}
			customerType := strings.TrimSpace(row[0])
			code := strings.Trim(strings.TrimSpace(row[1]), `="`)
			name := strings.TrimSpace(row[2])

			switch customerType {
			case "得意先":
				if _, err := clientStmt.Exec(code, name); err != nil {
					log.Printf("Failed to import client row %d: %v", i+1, err)
					http.Error(w, fmt.Sprintf("Failed to import client row %d", i+1), http.StatusInternalServerError)
					return
				}
				clientCount++
			case "卸業者":
				if _, err := wholesalerStmt.Exec(code, name); err != nil {
					log.Printf("Failed to import wholesaler row %d: %v", i+1, err)
					http.Error(w, fmt.Sprintf("Failed to import wholesaler row %d", i+1), http.StatusInternalServerError)
					return
				}
				wholesalerCount++
			default:
				// Unknown type, skip
			}
		}

		if err := db.InitializeSequenceFromMaxClientCode(tx); err != nil {
			log.Printf("Warning: failed to re-initialize client sequence after import: %v", err)
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の得意先、%d件の卸業者をインポートしました。", clientCount, wholesalerCount),
		})
	}
}

// ▲▲▲【追加ここまで】▲▲▲

// ExportClientsHandler は古い関数として残しますが、新しいUIからは呼び出されません。
func ExportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// (元のコードのまま)
		clients, err := db.GetAllClients(conn)
		if err != nil {
			http.Error(w, "Failed to get clients", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="client_master.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		headers := []string{"client_code", "client_name"}
		if err := csvWriter.Write(headers); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, client := range clients {
			record := []string{
				fmt.Sprintf("=%q", client.Code),
				client.Name,
			}
			if err := csvWriter.Write(record); err != nil {
				log.Printf("Failed to write client row to CSV (Code: %s): %v", client.Code, err)
			}
		}
	}
}

// ImportClientsHandler は古い関数として残しますが、新しいUIからは呼び出されません。
func ImportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// (元のコードのまま)
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

		stmt, err := tx.Prepare("INSERT OR REPLACE INTO client_master (client_code, client_name) VALUES (?, ?)")
		if err != nil {
			http.Error(w, "Failed to prepare DB statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		var importedCount int
		for i, row := range rows {
			if i == 0 || len(row) < 2 { // Skip header or short rows
				continue
			}
			clientCode := strings.Trim(strings.TrimSpace(row[0]), `="`)
			clientName := strings.TrimSpace(row[1])

			if _, err := stmt.Exec(clientCode, clientName); err != nil {
				log.Printf("Failed to import client row %d: %v", i+1, err)
				http.Error(w, fmt.Sprintf("Failed to import client row %d", i+1), http.StatusInternalServerError)
				return
			}
			importedCount++
		}

		if err := db.InitializeSequenceFromMaxClientCode(tx); err != nil {
			log.Printf("Warning: failed to re-initialize client sequence after import: %v", err)
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の得意先をインポートしました。", importedCount),
		})
	}
}

func ExportProductsHandler(conn *sql.DB) http.HandlerFunc {
	// (この関数は変更なし)
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get products", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="product_master_editable.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM for Excel compatibility

		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		header := []string{
			"product_code", "yj_code", "gs1_code", "product_name", "specification", "kana_name", "maker_name",
			"usage_classification", "package_form", "yj_unit_name", "yj_pack_unit_qty",
			"jan_pack_inner_qty", "jan_unit_code", "jan_pack_unit_qty", "origin",
			"nhi_price", "purchase_price",
			"flag_poison", "flag_deleterious", "flag_narcotic", "flag_psychotropic", "flag_stimulant", "flag_stimulant_raw",
			"is_order_stopped", "supplier_wholesale",
			"group_code", "shelf_number", "category", "user_notes",
		}
		if err := csvWriter.Write(header); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, p := range products {
			record := []string{
				fmt.Sprintf("=%q", p.ProductCode),
				p.YjCode,
				p.Gs1Code,
				p.ProductName,
				p.Specification,
				p.KanaName,
				p.MakerName,
				p.UsageClassification,
				p.PackageForm,
				p.YjUnitName,
				strconv.FormatFloat(p.YjPackUnitQty, 'f', -1, 64),
				strconv.FormatFloat(p.JanPackInnerQty, 'f', -1, 64),
				strconv.Itoa(p.JanUnitCode),
				strconv.FormatFloat(p.JanPackUnitQty, 'f', -1, 64),
				p.Origin,
				strconv.FormatFloat(p.NhiPrice, 'f', -1, 64),
				strconv.FormatFloat(p.PurchasePrice, 'f', -1, 64),
				strconv.Itoa(p.FlagPoison),
				strconv.Itoa(p.FlagDeleterious),
				strconv.Itoa(p.FlagNarcotic),
				strconv.Itoa(p.FlagPsychotropic),
				strconv.Itoa(p.FlagStimulant),
				strconv.Itoa(p.FlagStimulantRaw),
				strconv.Itoa(p.IsOrderStopped),
				p.SupplierWholesale,
				p.GroupCode,
				p.ShelfNumber,
				p.Category,
				p.UserNotes,
			}
			if err := csvWriter.Write(record); err != nil {
				log.Printf("Failed to write product row to CSV (JAN: %s): %v", p.ProductCode, err)
			}
		}
	}
}

func ImportProductsHandler(conn *sql.DB) http.HandlerFunc {
	// (この関数は変更なし)
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
			if i == 0 || len(row) < 29 {
				continue
			}

			yjPackUnitQty, _ := strconv.ParseFloat(row[10], 64)
			janPackInnerQty, _ := strconv.ParseFloat(row[11], 64)
			janUnitCode, _ := strconv.Atoi(row[12])
			janPackUnitQty, _ := strconv.ParseFloat(row[13], 64)
			nhiPrice, _ := strconv.ParseFloat(row[15], 64)
			purchasePrice, _ := strconv.ParseFloat(row[16], 64)
			flagPoison, _ := strconv.Atoi(row[17])
			flagDeleterious, _ := strconv.Atoi(row[18])
			flagNarcotic, _ := strconv.Atoi(row[19])
			flagPsychotropic, _ := strconv.Atoi(row[20])
			flagStimulant, _ := strconv.Atoi(row[21])
			flagStimulantRaw, _ := strconv.Atoi(row[22])
			isOrderStopped, _ := strconv.Atoi(row[23])
			productCode := strings.Trim(strings.TrimSpace(row[0]), `="`)

			input := model.ProductMasterInput{
				ProductCode:         productCode,
				YjCode:              strings.TrimSpace(row[1]),
				Gs1Code:             strings.TrimSpace(row[2]),
				ProductName:         strings.TrimSpace(row[3]),
				Specification:       strings.TrimSpace(row[4]),
				KanaName:            strings.TrimSpace(row[5]),
				MakerName:           strings.TrimSpace(row[6]),
				UsageClassification: strings.TrimSpace(row[7]),
				PackageForm:         strings.TrimSpace(row[8]),
				YjUnitName:          strings.TrimSpace(row[9]),
				YjPackUnitQty:       yjPackUnitQty,
				JanPackInnerQty:     janPackInnerQty,
				JanUnitCode:         janUnitCode,
				JanPackUnitQty:      janPackUnitQty,
				Origin:              strings.TrimSpace(row[14]),
				NhiPrice:            nhiPrice,
				PurchasePrice:       purchasePrice,
				FlagPoison:          flagPoison,
				FlagDeleterious:     flagDeleterious,
				FlagNarcotic:        flagNarcotic,
				FlagPsychotropic:    flagPsychotropic,
				FlagStimulant:       flagStimulant,
				FlagStimulantRaw:    flagStimulantRaw,
				IsOrderStopped:      isOrderStopped,
				SupplierWholesale:   strings.TrimSpace(row[24]),
				GroupCode:           strings.TrimSpace(row[25]),
				ShelfNumber:         strings.TrimSpace(row[26]),
				Category:            strings.TrimSpace(row[27]),
				UserNotes:           strings.TrimSpace(row[28]),
			}

			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				log.Printf("Failed to import product row %d: %v", i+1, err)
				http.Error(w, fmt.Sprintf("Failed to import product row %d", i+1), http.StatusInternalServerError)
				return
			}
			importedCount++
		}

		if err := db.InitializeSequenceFromMaxYjCode(tx); err != nil {
			log.Printf("Warning: failed to re-initialize YJ sequence after import: %v", err)
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("%d件の製品をインポートしました。", importedCount),
		})
	}
}
