package loader

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

type UpdatedProductView struct {
	ProductCode string `json:"productCode"`
	ProductName string `json:"productName"`
}

func CreateMasterUpdateHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("新しい要件に基づくJCSHMSマスター更新処理を開始します...")

		// === ステップ1: 必要なデータを全てメモリにロード ===
		newJcshmsData, err := loadCSVToMap("SOU/JCSHMS.CSV", false, 0)
		if err != nil {
			http.Error(w, "JCSHMS.CSVの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		newJancodeData, err := loadCSVToMap("SOU/JANCODE.CSV", true, 1)
		if err != nil {
			http.Error(w, "JANCODE.CSVの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		existingMasters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "既存の製品マスターの取得に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "トランザクションの開始に失敗しました", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		const updateQuery = `UPDATE product_master SET
			yj_code=?, product_name=?, origin=?, kana_name=?, maker_name=?,
			usage_classification=?, package_form=?, 
			yj_unit_name=?, yj_pack_unit_qty=?,
			flag_poison=?, flag_deleterious=?, flag_narcotic=?, flag_psychotropic=?,
			flag_stimulant=?, flag_stimulant_raw=?, jan_pack_inner_qty=?,
			jan_unit_code=?, jan_pack_unit_qty=?, nhi_price=?, specification=?
		WHERE product_code = ?`
		updateStmt, err := tx.Prepare(updateQuery)
		if err != nil {
			http.Error(w, "更新用SQLステートメントの準備に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer updateStmt.Close()

		const orphanQuery = `UPDATE product_master SET origin = ?, product_name = ?
		WHERE product_code = ?`
		orphanStmt, err := tx.Prepare(orphanQuery)
		if err != nil {
			http.Error(w, "PROVISIONAL化用SQLステートメントの準備に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer orphanStmt.Close()

		var updatedProducts, orphanedProducts []UpdatedProductView

		for _, master := range existingMasters {
			jcshmsRow, matchFound := newJcshmsData[master.ProductCode]

			if matchFound {
				jancodeRow := newJancodeData[master.ProductCode]
				input := createInputFromCSV(jcshmsRow, jancodeRow)

				if master.Origin == "PROVISIONAL" && input.YjCode == "" {
					input.Origin = "PROVISIONAL"
					input.YjCode = master.YjCode // 既存のYJコードを維持する
				}

				_, err := updateStmt.Exec(
					input.YjCode, input.ProductName, input.Origin, input.KanaName, input.MakerName,
					input.UsageClassification, input.PackageForm, input.YjUnitName, input.YjPackUnitQty,
					input.FlagPoison, input.FlagDeleterious, input.FlagNarcotic, input.FlagPsychotropic,
					input.FlagStimulant, input.FlagStimulantRaw, input.JanPackInnerQty,
					input.JanUnitCode, input.JanPackUnitQty, input.NhiPrice, input.Specification,
					master.ProductCode,
				)
				if err != nil {
					http.Error(w, fmt.Sprintf("マスターの上書き更新に失敗 (JAN: %s): %v", master.ProductCode, err), http.StatusInternalServerError)
					return
				}
				updatedProducts = append(updatedProducts, UpdatedProductView{ProductCode: master.ProductCode, ProductName: input.ProductName})

			} else {
				newProductName := master.ProductName
				if !strings.HasPrefix(master.ProductName, "◆") {
					newProductName = "◆" + newProductName
				}
				_, err := orphanStmt.Exec("PROVISIONAL", newProductName, master.ProductCode)
				if err != nil {
					http.Error(w, fmt.Sprintf("マスターのPROVISIONAL化に失敗 (JAN: %s): %v", master.ProductCode, err), http.StatusInternalServerError)
					return
				}
				orphanedProducts = append(orphanedProducts, UpdatedProductView{ProductCode: master.ProductCode, ProductName: newProductName})
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "トランザクションのコミットに失敗しました", http.StatusInternalServerError)
			return
		}

		log.Println("新しい要件に基づくJCSHMSマスター更新処理が正常に完了しました。")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":          "指定の要件で製品マスターの更新が完了しました。",
			"updatedProducts":  updatedProducts,
			"orphanedProducts": orphanedProducts,
		})
	}
}

func loadCSVToMap(filepath string, skipHeader bool, keyIndex int) (map[string][]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(transform.NewReader(f, japanese.ShiftJIS.NewDecoder()))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	if skipHeader {
		if _, err := r.Read(); err != nil && err != io.EOF {
			return nil, err
		}
	}

	dataMap := make(map[string][]string)
	for {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(row) > keyIndex {
			dataMap[row[keyIndex]] = row
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
	// ▼▼▼【ここが修正箇所】製品名と規格を分離して格納します ▼▼▼
	input.ProductName = strings.TrimSpace(jcshmsRow[18])
	input.Specification = strings.TrimSpace(jcshmsRow[20])
	// ▲▲▲【修正ここまで】▲▲▲
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

	if input.YjCode == "" {
		input.UsageClassification = "他"
	}

	if len(jancodeRow) > 6 {
		input.JanPackInnerQty, _ = strconv.ParseFloat(jancodeRow[6], 64)
	}
	if len(jancodeRow) > 7 {
		input.JanUnitCode, _ = strconv.Atoi(jancodeRow[7])
	}
	if len(jancodeRow) > 8 {
		input.JanPackUnitQty, _ = strconv.ParseFloat(jancodeRow[8], 64)
	}
	return input
}
