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
)

/**
 * @brief 得意先マスターをCSV形式でエクスポートします。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * 得意先コードはExcelで開いた際に先頭のゼロが消えないよう `="<CODE>"` の形式で出力します。
 */
func ExportClientsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

/**
 * @brief 得意先マスターをCSVファイルからインポートします。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 */
func ImportClientsHandler(conn *sql.DB) http.HandlerFunc {
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

/**
 * @brief 製品マスター（編集可能データ）をCSV形式でエクスポートします。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * データベースから編集可能な製品マスター（JCSHMS由来でないもの）を取得します。
 * JANコードはExcelで開いた際に先頭のゼロが消えないよう `="<JAN>"` の形式で出力します。
 */
func ExportProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetEditableProductMasters(conn)
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
			"product_code", "yj_code", "product_name", "origin", "kana_name", "maker_name",
			"usage_classification", "package_form",
			"yj_unit_name", "yj_pack_unit_qty", "flag_poison", "flag_deleterious", "flag_narcotic",
			"flag_psychotropic", "flag_stimulant", "flag_stimulant_raw", "jan_pack_inner_qty",
			"jan_unit_code", "jan_pack_unit_qty", "nhi_price", "purchase_price", "supplier_wholesale",
		}
		if err := csvWriter.Write(header); err != nil {
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, p := range products {
			record := []string{
				fmt.Sprintf("=%q", p.ProductCode),
				p.YjCode,
				p.ProductName,
				p.Origin,
				p.KanaName,
				p.MakerName,
				p.UsageClassification,
				p.PackageForm,
				p.YjUnitName,
				strconv.FormatFloat(p.YjPackUnitQty, 'f', -1, 64),
				strconv.Itoa(p.FlagPoison),
				strconv.Itoa(p.FlagDeleterious),
				strconv.Itoa(p.FlagNarcotic),
				strconv.Itoa(p.FlagPsychotropic),
				strconv.Itoa(p.FlagStimulant),
				strconv.Itoa(p.FlagStimulantRaw),
				strconv.FormatFloat(p.JanPackInnerQty, 'f', -1, 64),
				strconv.Itoa(p.JanUnitCode),
				strconv.FormatFloat(p.JanPackUnitQty, 'f', -1, 64),
				strconv.FormatFloat(p.NhiPrice, 'f', -1, 64),
				strconv.FormatFloat(p.PurchasePrice, 'f', -1, 64),
				p.SupplierWholesale,
			}
			if err := csvWriter.Write(record); err != nil {
				log.Printf("Failed to write product row to CSV (JAN: %s): %v", p.ProductCode, err)
			}
		}
	}
}

/**
 * @brief 製品マスターをCSVファイルからインポートします。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 */
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
			if i == 0 || len(row) < 22 {
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
			productCode := strings.Trim(strings.TrimSpace(row[0]), `="`)

			input := model.ProductMasterInput{
				ProductCode:         productCode,
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
