// C:\Dev\WASABI\db\search.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"wasabi/model"
	"wasabi/units"
)

// SearchJcshmsByName は製品名またはカナ名（部分一致）でJCSHMSマスタを検索し、表示用のモデルを返します。
func SearchJcshmsByName(conn *sql.DB, nameQuery string) ([]model.ProductMasterView, error) {
	const q = `
		SELECT
			j.JC000, j.JC009, j.JC018, j.JC022, j.JC030, j.JC013, j.JC037, j.JC039,
			j.JC044, j.JC050,
			ja.JA006, ja.JA008, ja.JA007
		FROM jcshms AS j
		LEFT JOIN jancode AS ja ON j.JC000 = ja.JA001
		WHERE j.JC018 LIKE ? OR j.JC022 LIKE ? ORDER BY j.JC022
		LIMIT 500`

	rows, err := conn.Query(q, "%"+nameQuery+"%", "%"+nameQuery+"%")
	if err != nil {
		return nil, fmt.Errorf("SearchJcshmsByName failed: %w", err)
	}
	defer rows.Close()

	var results []model.ProductMasterView
	for rows.Next() {
		var tempJcshms model.JCShms
		var jc000, jc009, jc018, jc022, jc030, jc013, jc037, jc039, jc050 sql.NullString
		var jc044 sql.NullFloat64

		if err := rows.Scan(
			&jc000, &jc009, &jc018, &jc022, &jc030, &jc013, &jc037, &jc039,
			&jc044, &jc050,
			&tempJcshms.JA006, &tempJcshms.JA008, &tempJcshms.JA007,
		); err != nil {
			return nil, err
		}

		tempJcshms.JC013 = jc013.String
		tempJcshms.JC037 = jc037.String
		tempJcshms.JC039 = jc039.String
		tempJcshms.JC044 = jc044.Float64

		nhiPriceVal, err := strconv.ParseFloat(jc050.String, 64)
		if err != nil {
			nhiPriceVal = 0
			if jc050.String != "" {
				log.Printf("[WARN] Invalid JC050 data during search: '%s'", jc050.String)
			}
		}
		tempJcshms.JC050 = nhiPriceVal

		var unitNhiPrice float64
		if tempJcshms.JC044 > 0 {
			unitNhiPrice = tempJcshms.JC050 / tempJcshms.JC044
		}

		janUnitCodeInt, _ := strconv.Atoi(tempJcshms.JA007.String)

		// ▼▼▼ [修正点] ProductMaster構造体からJanUnitNameの設定を削除 ▼▼▼
		view := model.ProductMasterView{
			ProductMaster: model.ProductMaster{
				ProductCode:         jc000.String,
				YjCode:              jc009.String,
				ProductName:         jc018.String,
				KanaName:            jc022.String,
				MakerName:           jc030.String,
				UsageClassification: jc013.String,
				PackageForm:         jc037.String,
				YjUnitName:          units.ResolveName(jc039.String),
				YjPackUnitQty:       jc044.Float64,
				JanPackInnerQty:     tempJcshms.JA006.Float64,
				JanPackUnitQty:      tempJcshms.JA008.Float64,
				JanUnitCode:         janUnitCodeInt,
				NhiPrice:            unitNhiPrice,
			},
			FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
		}

		// ▼▼▼ [修正点] ProductMasterViewのJanUnitNameフィールドに値を設定するロジックを追加 ▼▼▼
		if view.ProductMaster.JanUnitCode == 0 {
			view.JanUnitName = view.ProductMaster.YjUnitName
		} else {
			view.JanUnitName = units.ResolveName(tempJcshms.JA007.String)
		}
		// ▲▲▲ 修正ここまで ▲▲▲
		// ▲▲▲ 修正ここまで ▲▲▲
		results = append(results, view)
	}
	return results, nil
}

// SearchAllProductMastersByName は製品名またはカナ名（部分一致）でproduct_masterテーブル全体を検索します。
func SearchAllProductMastersByName(conn *sql.DB, nameQuery string) ([]model.ProductMasterView, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master 

		  WHERE kana_name LIKE ? OR product_name LIKE ? ORDER BY kana_name LIMIT 500`

	rows, err := conn.Query(q, "%"+nameQuery+"%", "%"+nameQuery+"%")
	if err != nil {
		return nil, fmt.Errorf("SearchAllProductMastersByName failed: %w", err)
	}
	defer rows.Close()

	var mastersView []model.ProductMasterView
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}

		tempJcshms := model.JCShms{
			JC037: m.PackageForm,
			JC039: m.YjUnitName,
			JC044: m.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
		}
		formattedSpec := units.FormatPackageSpec(&tempJcshms)

		mastersView = append(mastersView, model.ProductMasterView{
			ProductMaster:        *m,
			FormattedPackageSpec: formattedSpec,
		})
	}
	return mastersView, nil
}
