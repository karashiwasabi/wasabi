// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\valuation.go
package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// ValuationGroup は剤型ごとの在庫評価額の集計結果を保持します。
type ValuationGroup struct {
	UsageClassification string                     `json:"usageClassification"`
	DetailRows          []model.ValuationDetailRow `json:"detailRows"`
	TotalNhiValue       float64                    `json:"totalNhiValue"`
	TotalPurchaseValue  float64                    `json:"totalPurchaseValue"`
}

func GetInventoryValuation(conn *sql.DB, filters model.ValuationFilters) ([]ValuationGroup, error) {
	masterQuery := `SELECT ` + SelectColumns + ` FROM product_master WHERE 1=1`
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND (kana_name LIKE ? OR product_name LIKE ?)"
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%", "%"+filters.KanaName+"%")
	}
	if filters.UsageClassification != "" && filters.UsageClassification != "all" {
		masterQuery += " AND usage_classification = ?"
		masterArgs = append(masterArgs, filters.UsageClassification)
	}

	allMasters, err := getAllProductMastersFiltered(conn, masterQuery, masterArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtered product masters: %w", err)
	}
	if len(allMasters) == 0 {
		return []ValuationGroup{}, nil
	}

	yjHasJcshmsMaster := make(map[string]bool)
	mastersByJanCode := make(map[string]*model.ProductMaster)
	for _, master := range allMasters {
		if master.Origin == "JCSHMS" {
			yjHasJcshmsMaster[master.YjCode] = true
		}
		mastersByJanCode[master.ProductCode] = master
	}

	mastersByPackageKey := make(map[string][]*model.ProductMaster)
	for _, master := range allMasters {
		key := fmt.Sprintf("%s|%s|%g|%s", master.YjCode, master.PackageForm, master.JanPackInnerQty, master.YjUnitName)
		mastersByPackageKey[key] = append(mastersByPackageKey[key], master)
	}

	var detailRows []model.ValuationDetailRow

	for _, mastersInPackageGroup := range mastersByPackageKey {
		var totalStockForPackage float64
		for _, m := range mastersInPackageGroup {
			stock, err := CalculateStockOnDate(conn, m.ProductCode, filters.Date)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate stock on date for product %s: %w", m.ProductCode, err)
			}
			totalStockForPackage += stock
		}

		if totalStockForPackage == 0 {
			continue
		}

		var repMaster *model.ProductMaster
		if len(mastersInPackageGroup) > 0 {
			repMaster = mastersInPackageGroup[0]
			for _, m := range mastersInPackageGroup {
				if m.Origin == "JCSHMS" {
					repMaster = m
					break
				}
			}
		} else {
			continue
		}

		showAlert := false
		if repMaster.Origin != "JCSHMS" && !yjHasJcshmsMaster[repMaster.YjCode] {
			showAlert = true
		}

		tempJcshms := model.JCShms{
			JC037: repMaster.PackageForm, JC039: repMaster.YjUnitName, JC044: repMaster.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: repMaster.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: repMaster.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", repMaster.JanUnitCode), Valid: true},
		}
		spec := units.FormatSimplePackageSpec(&tempJcshms)

		unitNhiPrice := repMaster.NhiPrice
		totalNhiValue := totalStockForPackage * unitNhiPrice
		packageNhiPrice := unitNhiPrice * repMaster.YjPackUnitQty

		var totalPurchaseValue float64
		if repMaster.YjPackUnitQty > 0 {
			unitPurchasePrice := repMaster.PurchasePrice / repMaster.YjPackUnitQty
			totalPurchaseValue = totalStockForPackage * unitPurchasePrice
		}

		detailRows = append(detailRows, model.ValuationDetailRow{
			YjCode:               repMaster.YjCode,
			ProductName:          repMaster.ProductName,
			ProductCode:          repMaster.ProductCode,
			PackageSpec:          spec,
			Stock:                totalStockForPackage,
			YjUnitName:           repMaster.YjUnitName,
			PackageNhiPrice:      packageNhiPrice,
			PackagePurchasePrice: repMaster.PurchasePrice,
			TotalNhiValue:        totalNhiValue,
			TotalPurchaseValue:   totalPurchaseValue,
			ShowAlert:            showAlert,
		})
	}

	resultGroups := make(map[string]*ValuationGroup)
	for _, row := range detailRows {
		master, ok := mastersByJanCode[row.ProductCode]
		if !ok {
			continue
		}
		uc := master.UsageClassification
		group, ok := resultGroups[uc]
		if !ok {
			group = &ValuationGroup{UsageClassification: uc}
			resultGroups[uc] = group
		}
		group.DetailRows = append(group.DetailRows, row)
		group.TotalNhiValue += row.TotalNhiValue
		group.TotalPurchaseValue += row.TotalPurchaseValue
	}

	order := map[string]int{"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3, "4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6}
	var finalResult []ValuationGroup
	for _, group := range resultGroups {
		sort.Slice(group.DetailRows, func(i, j int) bool {
			// 製品名でソートするためにマスター情報を参照
			masterI, okI := mastersByJanCode[group.DetailRows[i].ProductCode]
			masterJ, okJ := mastersByJanCode[group.DetailRows[j].ProductCode]
			if !okI || !okJ {
				return group.DetailRows[i].ProductCode < group.DetailRows[j].ProductCode
			}
			return masterI.KanaName < masterJ.KanaName
		})
		finalResult = append(finalResult, *group)
	}
	sort.Slice(finalResult, func(i, j int) bool {
		prioI, okI := order[strings.TrimSpace(finalResult[i].UsageClassification)]
		if !okI {
			prioI = 7
		}
		prioJ, okJ := order[strings.TrimSpace(finalResult[j].UsageClassification)]
		if !okJ {
			prioJ = 7
		}
		return prioI < prioJ
	})

	return finalResult, nil
}

// getAllProductMastersFiltered はフィルタ条件に基づいて製品マスターを取得するヘルパー関数です。
func getAllProductMastersFiltered(conn *sql.DB, query string, args ...interface{}) ([]*model.ProductMaster, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetAllProductMastersFiltered query failed: %w", err)
	}
	defer rows.Close()

	var masters []*model.ProductMaster
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, m)
	}
	return masters, nil
}
