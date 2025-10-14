// C:\Users\wasab\OneDrive\デスクトップ\WASABI\loader\handler.go
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
	Status      string `json:"status"` // "UPDATED", "ORPHANED", "NEW"
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

		var updatedProducts, orphanedProducts, newlyAddedProducts []UpdatedProductView

		existingMastersMap := make(map[string]*model.ProductMaster)
		for _, m := range existingMasters {
			existingMastersMap[m.ProductCode] = m
		}

		// --- 既存マスターの更新と孤立化処理 ---
		for _, master := range existingMasters {
			jcshmsRow, matchFound := newJcshmsData[master.ProductCode]
			if matchFound {
				jancodeRow := newJancodeData[master.ProductCode]
				// ProductMasterInputへの変換ロジックを createInputFromCSV に集約
				input := createInputFromCSV(jcshmsRow, jancodeRow)
				if master.Origin == "PROVISIONAL" && input.YjCode == "" {
					input.Origin = "PROVISIONAL"
					input.YjCode = master.YjCode
				}
				// 既存のユーザー設定項目を維持する
				input.PurchasePrice = master.PurchasePrice
				input.SupplierWholesale = master.SupplierWholesale
				input.GroupCode = master.GroupCode
				input.ShelfNumber = master.ShelfNumber
				input.Category = master.Category
				input.UserNotes = master.UserNotes

				if err := db.UpsertProductMasterInTx(tx, input); err != nil {
					http.Error(w, fmt.Sprintf("マスターの上書き更新に失敗 (JAN: %s): %v", master.ProductCode, err), http.StatusInternalServerError)
					return
				}
				updatedProducts = append(updatedProducts, UpdatedProductView{ProductCode: master.ProductCode, ProductName: input.ProductName, Status: "UPDATED"})
			} else if master.Origin == "JCSHMS" {
				// JCSHMS由来のマスターがCSVから消えた場合、PROVISIONAL化する
				newProductName := master.ProductName
				if !strings.HasPrefix(master.ProductName, "◆") {
					newProductName = "◆" + newProductName
				}
				master.Origin = "PROVISIONAL"
				master.ProductName = newProductName
				// 更新用のInputを作成
				input := model.ProductMasterInput{
					ProductCode:         master.ProductCode,
					YjCode:              master.YjCode,
					Gs1Code:             master.Gs1Code,
					ProductName:         master.ProductName,
					KanaName:            master.KanaName,
					MakerName:           master.MakerName,
					Specification:       master.Specification,
					UsageClassification: master.UsageClassification,
					PackageForm:         master.PackageForm,
					YjUnitName:          master.YjUnitName,
					YjPackUnitQty:       master.YjPackUnitQty,
					JanPackInnerQty:     master.JanPackInnerQty,
					JanUnitCode:         master.JanUnitCode,
					JanPackUnitQty:      master.JanPackUnitQty,
					Origin:              master.Origin,
					NhiPrice:            master.NhiPrice,
					PurchasePrice:       master.PurchasePrice,
					FlagPoison:          master.FlagPoison,
					FlagDeleterious:     master.FlagDeleterious,
					FlagNarcotic:        master.FlagNarcotic,
					FlagPsychotropic:    master.FlagPsychotropic,
					FlagStimulant:       master.FlagStimulant,
					FlagStimulantRaw:    master.FlagStimulantRaw,
					IsOrderStopped:      master.IsOrderStopped,
					SupplierWholesale:   master.SupplierWholesale,
					GroupCode:           master.GroupCode,
					ShelfNumber:         master.ShelfNumber,
					Category:            master.Category,
					UserNotes:           master.UserNotes,
				}
				if err := db.UpsertProductMasterInTx(tx, input); err != nil {
					http.Error(w, fmt.Sprintf("マスターのPROVISIONAL化に失敗 (JAN: %s): %v", master.ProductCode, err), http.StatusInternalServerError)
					return
				}
				orphanedProducts = append(orphanedProducts, UpdatedProductView{ProductCode: master.ProductCode, ProductName: newProductName, Status: "ORPHANED"})
			}
		}

		// --- 新規マスターの追加処理 ---
		// ▼▼▼【ご指示により、JCSHMSマスターにしか存在しない新規品目を自動で追加する機能を削除】▼▼▼
		/*
			for productCode, jcshmsRow := range newJcshmsData {
				if _, exists := existingMastersMap[productCode]; !exists {
					jancodeRow := newJancodeData[productCode]
					input := createInputFromCSV(jcshmsRow, jancodeRow)
					if err := db.UpsertProductMasterInTx(tx, input); err != nil {
						http.Error(w, fmt.Sprintf("新規マスターの追加に失敗 (JAN: %s): %v", productCode, err), http.StatusInternalServerError)
						return
					}
					newlyAddedProducts = append(newlyAddedProducts, UpdatedProductView{ProductCode: productCode, ProductName: input.ProductName, Status: "NEW"})
				}
			}
		*/
		// ▲▲▲【削除ここまで】▲▲▲

		if err := tx.Commit(); err != nil {
			http.Error(w, "トランザクションのコミットに失敗しました", http.StatusInternalServerError)
			return
		}

		log.Println("新しい要件に基づくJCSHMSマスター更新処理が正常に完了しました。")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":            "指定の要件で製品マスターの更新が完了しました。",
			"updatedProducts":    updatedProducts,
			"orphanedProducts":   orphanedProducts,
			"newlyAddedProducts": newlyAddedProducts,
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
	// JC122 は 123番目のカラムなので、少なくとも123列必要
	if len(jcshmsRow) < 123 {
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
	// ▼▼▼【ここを修正】▼▼▼
	// 正しいインデックス 122 を使用して JC122 (GS1コード) を読み込む
	input.Gs1Code = jcshmsRow[122]
	// ▲▲▲【修正ここまで】▲▲▲
	input.ProductName = strings.TrimSpace(jcshmsRow[18])
	input.Specification = strings.TrimSpace(jcshmsRow[20])
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
