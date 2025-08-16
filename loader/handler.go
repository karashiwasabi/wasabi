package loader

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// UpdatedProductView は更新結果をフロントエンドに返すための構造体です
type UpdatedProductView struct {
	ProductCode string `json:"productCode"`
	ProductName string `json:"productName"`
}

// ReloadJcshmsHandler handles the request to reload JCSHMS and JANCODE master files.
func ReloadJcshmsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Attempting to reload JCSHMS and JANCODE master files...")

		// 1. 新しいJCSHMS/JANCODEをメモリに読み込む
		newJcshmsData, err := loadCSVToMap("SOU/JCSHMS.CSV", false)
		if err != nil {
			http.Error(w, "Failed to load new JCSHMS.CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}
		newJancodeData, err := loadCSVToMap("SOU/JANCODE.CSV", true)
		if err != nil {
			http.Error(w, "Failed to load new JANCODE.CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. DBから現在の product_master を全件取得
		productMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get product masters: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var updatedProducts, orphanedProducts []UpdatedProductView

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// 3. product_masterをループし、新しいデータと照合・更新
		for _, master := range productMasters {
			janCode := master.ProductCode
			newJcshmsRow, jcshmsExists := newJcshmsData[janCode]

			if jcshmsExists {
				// --- ケースA: 新しいJCSHMSにもJANが存在する場合 → 上書き更新 ---
				input := createInputFromCSV(newJcshmsRow, newJancodeData[janCode])
				if err := db.UpsertProductMasterInTx(tx, input); err != nil {
					http.Error(w, fmt.Sprintf("Failed to update master for %s: %v", janCode, err), http.StatusInternalServerError)
					return
				}
				updatedProducts = append(updatedProducts, UpdatedProductView{
					ProductCode: janCode,
					ProductName: input.ProductName,
				})
			} else if master.Origin == "JCSHMS" {
				// --- ケースB: 新しいJCSHMSにJANが存在しない場合 → 手動管理へ移行 ---
				newProductName := master.ProductName
				if !strings.HasPrefix(master.ProductName, "◆") {
					newProductName = "◆" + master.ProductName
				}

				_, err := tx.Exec(`
					UPDATE product_master SET origin = ?, product_name = ?
					WHERE product_code = ?`, "MANUAL", newProductName, janCode)

				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to orphan master for %s: %v", janCode, err), http.StatusInternalServerError)
					return
				}
				orphanedProducts = append(orphanedProducts, UpdatedProductView{
					ProductCode: janCode,
					ProductName: newProductName,
				})
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		log.Println("JCSHMS and JANCODE master files reloaded successfully.")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":          "JCSHMSマスターの更新が完了しました。",
			"updatedProducts":  updatedProducts,
			"orphanedProducts": orphanedProducts,
		})
	}
}

// loadCSVToMapはCSVファイルを読み込み、最初の列をキーとしたマップを返します
func loadCSVToMap(filepath string, skipHeader bool) (map[string][]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(transform.NewReader(f, japanese.ShiftJIS.NewDecoder()))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	if skipHeader {
		if _, err := r.Read(); err != nil && err.Error() != "EOF" {
			return nil, err
		}
	}

	dataMap := make(map[string][]string)
	for {
		row, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		if len(row) > 0 {
			dataMap[row[0]] = row
		}
	}
	return dataMap, nil
}

// createInputFromCSVはCSVの行データからProductMasterInputを生成します (WASABIの構造に適合)
func createInputFromCSV(jcshmsRow, jancodeRow []string) model.ProductMasterInput {
	var input model.ProductMasterInput
	if len(jcshmsRow) < 67 {
		return input
	}

	input.ProductCode = jcshmsRow[0]
	input.YjCode = jcshmsRow[9]
	input.ProductName = jcshmsRow[18]
	input.Origin = "JCSHMS"
	input.KanaName = jcshmsRow[22]
	input.MakerName = jcshmsRow[30]
	input.UsageClassification = jcshmsRow[13]
	input.PackageForm = jcshmsRow[37]
	input.PackageSpec = jcshmsRow[37]
	input.YjUnitName = jcshmsRow[39]
	input.YjPackUnitQty, _ = strconv.ParseFloat(jcshmsRow[44], 64)
	input.NhiPrice, _ = strconv.ParseFloat(jcshmsRow[50], 64)
	input.FlagPoison, _ = strconv.Atoi(jcshmsRow[61])
	input.FlagDeleterious, _ = strconv.Atoi(jcshmsRow[62])
	input.FlagNarcotic, _ = strconv.Atoi(jcshmsRow[63])
	input.FlagPsychotropic, _ = strconv.Atoi(jcshmsRow[64])
	input.FlagStimulant, _ = strconv.Atoi(jcshmsRow[65])
	input.FlagStimulantRaw, _ = strconv.Atoi(jcshmsRow[66])

	if len(jancodeRow) >= 9 {
		input.JanPackInnerQty, _ = strconv.ParseFloat(jancodeRow[6], 64)
		input.JanUnitCode, _ = strconv.Atoi(jancodeRow[7])
		input.JanPackUnitQty, _ = strconv.ParseFloat(jancodeRow[8], 64)
	}
	return input
}
