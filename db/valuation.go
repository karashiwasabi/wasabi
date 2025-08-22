// C:\Dev\WASABI\db\valuation.go
package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// ValuationGroup は剤型ごとの集計結果を保持します
type ValuationGroup struct {
	UsageClassification string                     `json:"usageClassification"`
	DetailRows          []model.ValuationDetailRow `json:"detailRows"`
	TotalNhiValue       float64                    `json:"totalNhiValue"`
	TotalPurchaseValue  float64                    `json:"totalPurchaseValue"`
}

// GetInventoryValuation は指定日の在庫評価レポートを生成します
func GetInventoryValuation(conn *sql.DB, filters model.ValuationFilters) ([]ValuationGroup, error) {
	// === ステップ1: フィルターに合致する製品マスターを取得 ===
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

	yjHasJcshmsMaster := make(map[string]bool)
	for _, master := range allMasters {
		if master.Origin == "JCSHMS" {
			yjHasJcshmsMaster[master.YjCode] = true
		}
	}

	// === ステップ2: 関連する全期間のトランザクションを一括取得 ===
	var productCodes []string
	for _, m := range allMasters {
		productCodes = append(productCodes, m.ProductCode)
	}
	if len(productCodes) == 0 {
		return []ValuationGroup{}, nil
	}

	transactionsByProductCode, err := getAllTransactionsForProducts(conn, productCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions for valuation: %w", err)
	}

	// === ステップ3: 包装グループごとに在庫を計算し、詳細行を作成 ===
	mastersByPackageKey := make(map[string][]*model.ProductMaster)
	for _, master := range allMasters {
		key := fmt.Sprintf("%s|%s|%g|%s", master.YjCode, master.PackageForm, master.JanPackInnerQty, master.YjUnitName)
		mastersByPackageKey[key] = append(mastersByPackageKey[key], master)
	}

	var detailRows []model.ValuationDetailRow

	for _, mastersInPackageGroup := range mastersByPackageKey {
		// (トランザクション集計と在庫計算ロジック)
		var allTxsForPackage []*model.TransactionRecord
		for _, m := range mastersInPackageGroup {
			if txs, ok := transactionsByProductCode[m.ProductCode]; ok {
				allTxsForPackage = append(allTxsForPackage, txs...)
			}
		}

		var txsUpToDate []*model.TransactionRecord
		for _, t := range allTxsForPackage {
			if t.TransactionDate <= filters.Date {
				txsUpToDate = append(txsUpToDate, t)
			}
		}

		sort.Slice(txsUpToDate, func(i, j int) bool {
			if txsUpToDate[i].TransactionDate != txsUpToDate[j].TransactionDate {
				return txsUpToDate[i].TransactionDate < txsUpToDate[j].TransactionDate
			}
			return txsUpToDate[i].ID < txsUpToDate[j].ID
		})

		var runningBalance float64
		latestInventoryDate := ""
		inventorySumsByDate := make(map[string]float64)

		for _, t := range txsUpToDate {
			if t.Flag == 0 {
				inventorySumsByDate[t.TransactionDate] += t.YjQuantity
				if t.TransactionDate > latestInventoryDate {
					latestInventoryDate = t.TransactionDate
				}
			}
		}

		if latestInventoryDate != "" {
			runningBalance = inventorySumsByDate[latestInventoryDate]
			for _, t := range txsUpToDate {
				if t.TransactionDate > latestInventoryDate {
					runningBalance += t.SignedYjQty()
				}
			}
		} else {
			for _, t := range txsUpToDate {
				runningBalance += t.SignedYjQty()
			}
		}

		if runningBalance == 0 {
			continue
		}

		// ▼▼▼ [修正点] 代表製品名をJCSHMS由来のものから優先的に選択するロジックを追加 ▼▼▼
		var repMaster *model.ProductMaster
		if len(mastersInPackageGroup) > 0 {
			// まずリストの最初のマスターをフォールバックとして設定
			repMaster = mastersInPackageGroup[0]
			// JCSHMS由来のマスターを探す
			for _, m := range mastersInPackageGroup {
				if m.Origin == "JCSHMS" {
					repMaster = m // JCSHMS由来のマスターが見つかれば、それを代表とする
					break
				}
			}
		} else {
			continue // マスターがなければこの包装グループはスキップ
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		showAlert := false
		if repMaster.Origin != "JCSHMS" && !yjHasJcshmsMaster[repMaster.YjCode] {
			showAlert = true
		}

		// 包装仕様を生成
		tempJcshms := model.JCShms{
			JC037: repMaster.PackageForm, JC039: repMaster.YjUnitName, JC044: repMaster.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: repMaster.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: repMaster.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", repMaster.JanUnitCode), Valid: true},
		}
		spec := units.FormatPackageSpec(&tempJcshms)

		// 薬価・納入価を計算
		packageNhiPrice := repMaster.NhiPrice * repMaster.YjPackUnitQty
		totalNhiValue := runningBalance * repMaster.NhiPrice
		var totalPurchaseValue float64
		if repMaster.YjPackUnitQty > 0 {
			unitPurchasePrice := repMaster.PurchasePrice / repMaster.YjPackUnitQty
			totalPurchaseValue = runningBalance * unitPurchasePrice
		}

		detailRows = append(detailRows, model.ValuationDetailRow{
			YjCode:               repMaster.YjCode,
			ProductName:          repMaster.ProductName,
			ProductCode:          repMaster.ProductCode,
			PackageSpec:          spec,
			Stock:                runningBalance,
			YjUnitName:           repMaster.YjUnitName,
			PackageNhiPrice:      packageNhiPrice,
			PackagePurchasePrice: repMaster.PurchasePrice,
			TotalNhiValue:        totalNhiValue,
			TotalPurchaseValue:   totalPurchaseValue,
			ShowAlert:            showAlert,
		})
	}

	// === ステップ4: 剤型ごとにグルーピング ===
	mastersByJanCode := make(map[string]*model.ProductMaster)
	for _, m := range allMasters {
		mastersByJanCode[m.ProductCode] = m
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

	// (並び替えロジックは変更なし)
	order := map[string]int{"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3, "4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6}
	var finalResult []ValuationGroup
	for _, group := range resultGroups {
		sort.Slice(group.DetailRows, func(i, j int) bool {
			return mastersByJanCode[group.DetailRows[i].ProductCode].KanaName < mastersByJanCode[group.DetailRows[j].ProductCode].KanaName
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

func getAllTransactionsForProducts(conn *sql.DB, productCodes []string) (map[string][]*model.TransactionRecord, error) {
	transactionsByProductCode := make(map[string][]*model.TransactionRecord)
	if len(productCodes) == 0 {
		return transactionsByProductCode, nil
	}

	var txArgs []interface{}
	for _, pc := range productCodes {
		txArgs = append(txArgs, pc)
	}

	txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `)`
	txRows, err := conn.Query(txQuery, txArgs...)
	if err != nil {
		return nil, err
	}
	defer txRows.Close()

	for txRows.Next() {
		t, err := ScanTransactionRecord(txRows)
		if err != nil {
			return nil, err
		}
		transactionsByProductCode[t.JanCode] = append(transactionsByProductCode[t.JanCode], t)
	}
	return transactionsByProductCode, nil
}
