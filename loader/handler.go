// C:\Dev\WASABI\loader\handler.go

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

// (UpdatedProductView 構造体は変更なし)
type UpdatedProductView struct {
	ProductCode string `json:"productCode"`
	ProductName string `json:"productName"`
}

// ▼▼▼ [修正点] ReloadJcshmsHandlerのロジックを全面的に改善 ▼▼▼
// ReloadJcshmsHandler handles the request to reload JCSHMS and JANCODE master files.
func ReloadJcshmsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Attempting to reload JCSHMS and JANCODE master files...")

		// 1. 新しいCSVマスターをメモリにロード
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

		// 2. 既存の製品マスターを全て取得し、JANコードをキーにしたマップを作成
		existingMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get product masters: "+err.Error(), http.StatusInternalServerError)
			return
		}
		existingMastersMap := make(map[string]*model.ProductMaster)
		for _, master := range existingMasters {
			existingMastersMap[master.ProductCode] = master
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var updatedProducts, orphanedProducts, newProducts []UpdatedProductView

		// 3. 新しいJCSHMSデータをループし、既存マスターを更新または新規マスターを挿入
		for janCode, jcshmsRow := range newJcshmsData {
			input := createInputFromCSV(jcshmsRow, newJancodeData[janCode])
			isNew := false

			// 既存マスターがあれば、納入価と卸情報を引き継ぐ
			if existingMaster, ok := existingMastersMap[janCode]; ok {
				input.PurchasePrice = existingMaster.PurchasePrice
				input.SupplierWholesale = existingMaster.SupplierWholesale
				delete(existingMastersMap, janCode) // 処理済みとしてマップから削除
			} else {
				isNew = true
			}

			// YJコードがない場合は仮マスターとして登録
			if input.YjCode == "" {
				input.Origin = "PROVISIONAL"
				input.UsageClassification = "他"
				newYj, err := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
				if err != nil {
					http.Error(w, "Failed to get next sequence for YJ-less jcshms master: "+err.Error(), http.StatusInternalServerError)
					return
				}
				input.YjCode = newYj
			}

			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				http.Error(w, fmt.Sprintf("Failed to upsert master for %s: %v", janCode, err), http.StatusInternalServerError)
				return
			}

			if isNew {
				newProducts = append(newProducts, UpdatedProductView{ProductCode: janCode, ProductName: input.ProductName})
			} else {
				updatedProducts = append(updatedProducts, UpdatedProductView{ProductCode: janCode, ProductName: input.ProductName})
			}
		}

		// 4. 新しいJCSHMSデータに存在しなかった既存マスターを「孤立」させる
		for janCode, master := range existingMastersMap {
			if master.Origin == "JCSHMS" {
				newProductName := master.ProductName
				if !strings.HasPrefix(master.ProductName, "◆") {
					newProductName = "◆" + master.ProductName
				}
				_, err := tx.Exec(`UPDATE product_master SET origin = ?, product_name = ? WHERE product_code = ?`, "PROVISIONAL", newProductName, janCode)
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to orphan master for %s: %v", janCode, err), http.StatusInternalServerError)
					return
				}
				orphanedProducts = append(orphanedProducts, UpdatedProductView{ProductCode: janCode, ProductName: newProductName})
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
			"newProducts":      newProducts,
			"updatedProducts":  updatedProducts,
			"orphanedProducts": orphanedProducts,
		})
	}
}

// ▲▲▲ 修正ここまで ▲▲▲

// (loadCSVToMap, createInputFromCSV は変更なし)
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
func createInputFromCSV(jcshmsRow, jancodeRow []string) model.ProductMasterInput {
	var input model.ProductMasterInput
	if len(jcshmsRow) < 67 {
		return input
	}

	yjPackUnitQty, _ := strconv.ParseFloat(jcshmsRow[44], 64)
	packagePrice, _ := strconv.ParseFloat(jcshmsRow[50], 64)

	var unitNhiPrice float64
	if yjPackUnitQty > 0 {
		unitNhiPrice = packagePrice / yjPackUnitQty
	}

	input.ProductCode = jcshmsRow[0]
	input.YjCode = jcshmsRow[9]
	input.ProductName = jcshmsRow[18]
	input.Origin = "JCSHMS"
	input.KanaName = jcshmsRow[22]
	input.MakerName = jcshmsRow[30]
	input.UsageClassification = jcshmsRow[13]
	input.PackageForm = jcshmsRow[37]
	input.YjUnitName = jcshmsRow[39]
	input.YjPackUnitQty = yjPackUnitQty
	input.NhiPrice = unitNhiPrice
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