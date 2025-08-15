// C:\Dev\WASABI\db\aggregation.go
package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// GetStockLedger generates the stock ledger report.
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	precompTotals, err := GetPreCompoundingTotals(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-compounding totals for aggregation: %w", err)
	}

	masterQuery := `SELECT ` + selectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND p.kana_name LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%")
	}
	if filters.DosageForm != "" {
		masterQuery += " AND p.usage_classification LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.DosageForm+"%")
	}
	if len(filters.DrugTypes) > 0 && filters.DrugTypes[0] != "" {
		var conditions []string
		flagMap := map[string]string{
			"poison": "p.flag_poison = 1", "deleterious": "p.flag_deleterious = 1", "narcotic": "p.flag_narcotic = 1",
			"psychotropic1": "p.flag_psychotropic = 1", "psychotropic2": "p.flag_psychotropic = 2", "psychotropic3": "p.flag_psychotropic = 3",
			"stimulant": "p.flag_stimulant = 1", "stimulant_raw": "p.flag_stimulant_raw = 1",
		}
		for _, dt := range filters.DrugTypes {
			if cond, ok := flagMap[dt]; ok {
				conditions = append(conditions, cond)
			}
		}
		if len(conditions) > 0 {
			masterQuery += " AND (" + strings.Join(conditions, " OR ") + ")"
		}
	}

	masterRows, err := conn.Query(masterQuery, masterArgs...)
	if err != nil {
		return nil, err
	}
	defer masterRows.Close()

	mastersByYjCode := make(map[string][]*model.ProductMaster)
	var productCodes []string
	for masterRows.Next() {
		m, err := scanProductMaster(masterRows)
		if err != nil {
			return nil, err
		}
		if m.YjCode != "" {
			mastersByYjCode[m.YjCode] = append(mastersByYjCode[m.YjCode], m)
		}
		productCodes = append(productCodes, m.ProductCode)
	}
	if len(productCodes) == 0 {
		return []model.StockLedgerYJGroup{}, nil
	}

	transactionsByProductCode := make(map[string][]*model.TransactionRecord)
	if len(productCodes) > 0 {
		var txArgs []interface{}
		for _, pc := range productCodes {
			txArgs = append(txArgs, pc)
		}
		txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `) AND transaction_date >= ? AND transaction_date <= ? ORDER BY transaction_date, id`
		txArgsWithDate := append(txArgs, filters.StartDate, filters.EndDate)
		txRows, err := conn.Query(txQuery, txArgsWithDate...)
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
	}

	var result []model.StockLedgerYJGroup
	for yjCode, mastersInYjGroup := range mastersByYjCode {
		if len(mastersInYjGroup) == 0 {
			continue
		}
		representativeProductName := mastersInYjGroup[0].ProductName
		yjGroup := model.StockLedgerYJGroup{
			YjCode:      yjCode,
			ProductName: representativeProductName,
			YjUnitName:  units.ResolveName(mastersInYjGroup[0].YjUnitName),
		}
		mastersByPackageKey := make(map[string][]*model.ProductMaster)
		for _, m := range mastersInYjGroup {
			key := fmt.Sprintf("%s|%g|%s", m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			mastersByPackageKey[key] = append(mastersByPackageKey[key], m)
		}

		var allPackageLedgers []model.StockLedgerPackageGroup
		for key, mastersInPackageGroup := range mastersByPackageKey {
			var pkgTxs []*model.TransactionRecord
			for _, m := range mastersInPackageGroup {
				pkgTxs = append(pkgTxs, transactionsByProductCode[m.ProductCode]...)
			}
			sort.Slice(pkgTxs, func(i, j int) bool {
				if pkgTxs[i].TransactionDate != pkgTxs[j].TransactionDate {
					return pkgTxs[i].TransactionDate < pkgTxs[j].TransactionDate
				}
				return pkgTxs[i].ID < pkgTxs[j].ID
			})
			pkg := model.StockLedgerPackageGroup{PackageKey: key}
			inventoryDayTotals := make(map[string]float64)
			hasInventoryInGroup := false
			for _, t := range pkgTxs {
				if t.Flag == 0 {
					inventoryDayTotals[t.TransactionDate] += t.YjQuantity
					hasInventoryInGroup = true
				}
			}

			var netChange, maxUsage, runningBalance float64
			var transactions []model.LedgerTransaction
			if !hasInventoryInGroup {
				pkg.StartingBalance = "期間棚卸なし"
				pkg.EndingBalance = "期間棚卸なし"
				for _, t := range pkgTxs {
					runningBalance += t.SignedYjQty()
					transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
				}
			} else {
				var startingBalanceQty float64
				isStartingBalanceSet := false
				latestInventoryDate := ""
				for date := range inventoryDayTotals {
					if date > latestInventoryDate {
						latestInventoryDate = date
					}
				}
				for _, t := range pkgTxs {
					if t.TransactionDate < latestInventoryDate {
						runningBalance += t.SignedYjQty()
					}
				}
				if !isStartingBalanceSet {
					startingBalanceQty = runningBalance + inventoryDayTotals[latestInventoryDate]
					isStartingBalanceSet = true
				}
				runningBalance = startingBalanceQty
				for _, t := range pkgTxs {
					if t.TransactionDate >= latestInventoryDate {
						if t.Flag != 0 {
							runningBalance += t.SignedYjQty()
						}
						transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
					}
				}
				pkg.StartingBalance = startingBalanceQty
				pkg.EndingBalance = runningBalance
			}
			for _, t := range pkgTxs {
				netChange += t.SignedYjQty()
				if t.Flag == 3 && t.YjQuantity > maxUsage {
					maxUsage = t.YjQuantity
				}
			}
			var precompTotalForPackage float64
			for _, master := range mastersInPackageGroup {
				if total, ok := precompTotals[master.ProductCode]; ok {
					precompTotalForPackage += total
				}
			}
			pkg.Transactions = transactions
			pkg.NetChange = netChange
			pkg.MaxUsage = maxUsage
			pkg.BaseReorderPoint = maxUsage * filters.Coefficient
			pkg.PrecompoundedTotal = precompTotalForPackage
			pkg.ReorderPoint = pkg.BaseReorderPoint + pkg.PrecompoundedTotal
			if endBalanceFloat, ok := pkg.EndingBalance.(float64); ok {
				pkg.IsReorderNeeded = endBalanceFloat < pkg.ReorderPoint && pkg.MaxUsage > 0
			}
			allPackageLedgers = append(allPackageLedgers, pkg)
		}
		if len(allPackageLedgers) > 0 {
			var yjTotalEnding, yjTotalNetChange, yjTotalReorderPoint, yjTotalBaseReorderPoint, yjTotalPrecompounded float64
			var yjTotalStarting interface{} = "期間棚卸なし"
			isYjReorderNeeded, hasAnyInventory := false, false
			for _, pkg := range allPackageLedgers {
				if start, ok := pkg.StartingBalance.(float64); ok {
					if !hasAnyInventory {
						yjTotalStarting = float64(0)
						hasAnyInventory = true
					}
					if val, ok := yjTotalStarting.(float64); ok {
						yjTotalStarting = val + start
					}
				}
				if end, ok := pkg.EndingBalance.(float64); ok {
					yjTotalEnding += end
				}
				yjTotalNetChange += pkg.NetChange
				yjTotalReorderPoint += pkg.ReorderPoint
				yjTotalBaseReorderPoint += pkg.BaseReorderPoint
				yjTotalPrecompounded += pkg.PrecompoundedTotal
				if pkg.IsReorderNeeded {
					isYjReorderNeeded = true
				}
			}
			yjGroup.StartingBalance = yjTotalStarting
			if hasAnyInventory {
				yjGroup.EndingBalance = yjTotalEnding
			} else {
				yjGroup.EndingBalance = "期間棚卸なし"
			}
			yjGroup.NetChange = yjTotalNetChange
			yjGroup.TotalReorderPoint = yjTotalReorderPoint
			yjGroup.TotalBaseReorderPoint = yjTotalBaseReorderPoint
			yjGroup.TotalPrecompounded = yjTotalPrecompounded
			yjGroup.IsReorderNeeded = isYjReorderNeeded
			yjGroup.PackageLedgers = allPackageLedgers
			result = append(result, yjGroup)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "注": 3,
			"4": 4, "歯": 4, "5": 5, "機": 5, "6": 6, "他": 6,
		}
		masterI := mastersByYjCode[result[i].YjCode][0]
		masterJ := mastersByYjCode[result[j].YjCode][0]
		prioI, okI := prio[strings.TrimSpace(masterI.UsageClassification)]
		if !okI {
			prioI = 7
		}
		prioJ, okJ := prio[strings.TrimSpace(masterJ.UsageClassification)]
		if !okJ {
			prioJ = 7
		}
		if prioI != prioJ {
			return prioI < prioJ
		}
		return masterI.KanaName < masterJ.KanaName
	})
	return result, nil
}
