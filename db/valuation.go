package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
)

// ValuationGroup は剤型ごとの集計結果を保持します
type ValuationGroup struct {
	UsageClassification string             `json:"usageClassification"`
	YjGroups            []ValuationYjGroup `json:"yjGroups"`
	TotalNhiValue       float64            `json:"totalNhiValue"`
	TotalPurchaseValue  float64            `json:"totalPurchaseValue"`
}

// ValuationYjGroup はYJコードごとの集計結果を保持します
type ValuationYjGroup struct {
	YjCode        string  `json:"yjCode"`
	ProductName   string  `json:"productName"`
	KanaName      string  `json:"kanaName"`
	TotalYjStock  float64 `json:"totalYjStock"`
	YjUnitName    string  `json:"yjUnitName"`
	NhiValue      float64 `json:"nhiValue"`
	PurchaseValue float64 `json:"purchaseValue"`
	ShowAlert     bool    `json:"showAlert"`
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

	// === ステップ3: YJコードごとに在庫を計算 ===
	mastersByYjCode := make(map[string][]*model.ProductMaster)
	for _, master := range allMasters {
		if master.YjCode != "" {
			mastersByYjCode[master.YjCode] = append(mastersByYjCode[master.YjCode], master)
		}
	}

	yjGroupMap := make(map[string]ValuationYjGroup)

	for yjCode, mastersInYjGroup := range mastersByYjCode {
		mastersByPackageKey := make(map[string][]*model.ProductMaster)
		for _, m := range mastersInYjGroup {
			key := fmt.Sprintf("%s|%s|%g|%s", m.YjCode, m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			mastersByPackageKey[key] = append(mastersByPackageKey[key], m)
		}

		var totalStockForYj float64

		for _, mastersInPackageGroup := range mastersByPackageKey {
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
			totalStockForYj += runningBalance
		}

		if totalStockForYj == 0 && filters.KanaName == "" {
			continue
		}

		var representativeMaster *model.ProductMaster
		containsJcshms := false
		for _, m := range mastersInYjGroup {
			if m.Origin == "JCSHMS" {
				representativeMaster = m
				containsJcshms = true
				break
			}
		}
		if representativeMaster == nil {
			if len(mastersInYjGroup) > 0 {
				representativeMaster = mastersInYjGroup[0]
			} else {
				continue
			}
		}

		totalNhiValue := totalStockForYj * representativeMaster.NhiPrice
		var totalPurchaseValue float64
		if representativeMaster.YjPackUnitQty > 0 {
			unitPurchasePrice := representativeMaster.PurchasePrice / representativeMaster.YjPackUnitQty
			totalPurchaseValue = totalStockForYj * unitPurchasePrice
		}

		showAlert := false
		if !containsJcshms {
			uc := strings.TrimSpace(representativeMaster.UsageClassification)
			if uc != "5" && uc != "機" && uc != "6" && uc != "他" {
				showAlert = true
			}
		}

		yjGroupMap[yjCode] = ValuationYjGroup{
			YjCode:        yjCode,
			ProductName:   representativeMaster.ProductName,
			KanaName:      representativeMaster.KanaName,
			TotalYjStock:  totalStockForYj,
			YjUnitName:    representativeMaster.YjUnitName,
			NhiValue:      totalNhiValue,
			PurchaseValue: totalPurchaseValue,
			ShowAlert:     showAlert,
		}
	}

	resultGroups := make(map[string]*ValuationGroup)
	for yjCode, yjGroupData := range yjGroupMap {
		if masterList, ok := mastersByYjCode[yjCode]; ok && len(masterList) > 0 {
			uc := masterList[0].UsageClassification
			group, ok := resultGroups[uc]
			if !ok {
				group = &ValuationGroup{UsageClassification: uc}
				resultGroups[uc] = group
			}
			group.YjGroups = append(group.YjGroups, yjGroupData)
			group.TotalNhiValue += yjGroupData.NhiValue
			group.TotalPurchaseValue += yjGroupData.PurchaseValue
		}
	}

	order := map[string]int{"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3, "4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6}
	var finalResult []ValuationGroup
	for _, group := range resultGroups {
		sort.Slice(group.YjGroups, func(i, j int) bool {
			return group.YjGroups[i].KanaName < group.YjGroups[j].KanaName
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