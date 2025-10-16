// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\search.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings" // stringsパッケージをインポート
	"wasabi/model"
	"wasabi/units"
)

/**
 * @brief 製品名またはカナ名でJCSHMSマスタを検索し、表示用のモデルを返します。
 * @param conn データベース接続
 * @param nameQuery 検索キーワード
 * @return []model.ProductMasterView 検索結果のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * この関数は `product_master` テーブルではなく、`jcshms` と `jancode` テーブルを直接検索します。
 * アプリ内にまだ存在しない公式の医薬品マスターを探すために使用されます。
 */
func SearchJcshmsByName(conn *sql.DB, nameQuery string) ([]model.ProductMasterView, error) {
	// ▼▼▼【ここから修正】▼▼▼
	const q = `
		SELECT
			j.JC000, j.JC009, j.JC018, j.JC022, j.JC030, j.JC013, j.JC037, j.JC039,
			j.JC044, j.JC050,
			ja.JA006, ja.JA008, ja.JA007
		FROM jcshms AS j
		LEFT JOIN jancode AS ja ON j.JC000 = ja.JA001
		WHERE j.JC018 LIKE ? OR j.JC022 LIKE ?
		ORDER BY
			CASE
				WHEN TRIM(j.JC013) = '内' OR TRIM(j.JC013) = '1' THEN 1
				WHEN TRIM(j.JC013) = '外' OR TRIM(j.JC013) = '2' THEN 2
				WHEN TRIM(j.JC013) = '注' OR TRIM(j.JC013) = '3' THEN 3
				WHEN TRIM(j.JC013) = '歯' OR TRIM(j.JC013) = '4' THEN 4
				WHEN TRIM(j.JC013) = '機' OR TRIM(j.JC013) = '5' THEN 5
				WHEN TRIM(j.JC013) = '他' OR TRIM(j.JC013) = '6' THEN 6
				ELSE 7
			END,
			j.JC022
		LIMIT 500`

	rows, err := conn.Query(q, "%"+nameQuery+"%", "%"+nameQuery+"%")
	if err != nil {
		return nil, fmt.Errorf("SearchJcshmsByName failed: %w", err)
	}
	defer rows.Close()

	type tempResult struct {
		jcshms   model.JCShms
		jc000    string
		jc009    string
		jc018    string
		jc022    string
		jc030    string
		nhiPrice float64
	}

	var tempResults []tempResult
	var janCodes []interface{}

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

		tempResults = append(tempResults, tempResult{
			jcshms:   tempJcshms,
			jc000:    jc000.String,
			jc009:    jc009.String,
			jc018:    jc018.String,
			jc022:    jc022.String,
			jc030:    jc030.String,
			nhiPrice: nhiPriceVal,
		})
		janCodes = append(janCodes, jc000.String)
	}

	adoptedMap := make(map[string]bool)
	if len(janCodes) > 0 {
		query := "SELECT product_code FROM product_master WHERE product_code IN (?" + strings.Repeat(",?", len(janCodes)-1) + ")"
		adoptedRows, err := conn.Query(query, janCodes...)
		if err != nil {
			return nil, fmt.Errorf("failed to check adopted products: %w", err)
		}
		defer adoptedRows.Close()
		for adoptedRows.Next() {
			var productCode string
			if err := adoptedRows.Scan(&productCode); err != nil {
				return nil, err
			}
			adoptedMap[productCode] = true
		}
	}

	var results []model.ProductMasterView
	for _, temp := range tempResults {
		tempJcshms := temp.jcshms
		tempJcshms.JC050 = temp.nhiPrice

		var unitNhiPrice float64
		if tempJcshms.JC044 > 0 {
			unitNhiPrice = tempJcshms.JC050 / tempJcshms.JC044
		}

		janUnitCodeInt, _ := strconv.Atoi(tempJcshms.JA007.String)

		view := model.ProductMasterView{
			ProductMaster: model.ProductMaster{
				ProductCode:         temp.jc000,
				YjCode:              temp.jc009,
				ProductName:         temp.jc018,
				KanaName:            temp.jc022,
				MakerName:           temp.jc030,
				UsageClassification: tempJcshms.JC013,
				PackageForm:         tempJcshms.JC037,
				YjUnitName:          units.ResolveName(tempJcshms.JC039),
				YjPackUnitQty:       tempJcshms.JC044,
				JanPackInnerQty:     tempJcshms.JA006.Float64,
				JanPackUnitQty:      tempJcshms.JA008.Float64,
				JanUnitCode:         janUnitCodeInt,
				NhiPrice:            unitNhiPrice,
			},
			FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
			IsAdopted:            adoptedMap[temp.jc000],
		}

		if view.ProductMaster.JanUnitCode == 0 {
			view.JanUnitName = view.ProductMaster.YjUnitName
		} else {
			view.JanUnitName = units.ResolveName(tempJcshms.JA007.String)
		}
		results = append(results, view)
	}
	return results, nil
	// ▲▲▲【修正ここまで】▲▲▲
}

/**
 * @brief 製品名またはカナ名で `product_master` テーブル全体を検索します。
 * @param conn データベース接続
 * @param nameQuery 検索キーワード
 * @return []model.ProductMasterView 検索結果のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * JCSHMS由来のマスターと、手動で登録されたPROVISIONALマスターの両方が検索対象になります。
 */
func SearchAllProductMastersByName(conn *sql.DB, nameQuery string) ([]model.ProductMasterView, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master 
		  WHERE kana_name LIKE ? OR product_name LIKE ? 
		  ORDER BY
			CASE
				WHEN TRIM(usage_classification) = '内' OR TRIM(usage_classification) = '1' THEN 1
				WHEN TRIM(usage_classification) = '外' OR TRIM(usage_classification) = '2' THEN 2
				WHEN TRIM(usage_classification) = '注' OR TRIM(usage_classification) = '3' THEN 3
				WHEN TRIM(usage_classification) = '歯' OR TRIM(usage_classification) = '4' THEN 4
				WHEN TRIM(usage_classification) = '機' OR TRIM(usage_classification) = '5' THEN 5
				WHEN TRIM(usage_classification) = '他' OR TRIM(usage_classification) = '6' THEN 6
				ELSE 7
			END,
			kana_name
		  LIMIT 500`

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
