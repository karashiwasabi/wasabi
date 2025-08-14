// C:\Dev\WASABI\db\aggregation.go

package db

import (
	"database/sql"
	"fmt"
	"log" // ▼▼▼ [修正点] logパッケージをインポート ▼▼▼
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// (inventoryInfo struct and GetStockLedger function up to the sort are unchanged)
type inventoryInfo struct {
	Date     string
	Quantity float64
}

func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	// (Function body from line 17 to 337 is unchanged)
	masterQuery := `SELECT ` + selectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND p.kana_name LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%")
	}
	if filters.DosageForm != "" {
		masterQuery += " AND p.package_form LIKE ? "
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

	var txArgs []interface{}
	for _, pc := range productCodes {
		txArgs = append(txArgs, pc)
	}
	txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `) AND transaction_date >= ? AND transaction_date <= ? ORDER BY transaction_date, id`
	txArgs = append(txArgs, filters.StartDate, filters.EndDate)

	txRows, err := conn.Query(txQuery, txArgs...)
	if err != nil {
		return nil, err
	}
	defer txRows.Close()

	transactionsByProductCode := make(map[string][]*model.TransactionRecord)
	for txRows.Next() {
		t, err := ScanTransactionRecord(txRows)
		if err != nil {
			return nil, err
		}
		transactionsByProductCode[t.JanCode] = append(transactionsByProductCode[t.JanCode], t)
	}

	latestInventoryMap := make(map[string]inventoryInfo)
	if len(productCodes) > 0 {
		invQueryArgs := make([]interface{}, len(productCodes))
		copy(invQueryArgs, txArgs[:len(productCodes)])
		invQueryArgs = append(invQueryArgs, filters.StartDate, filters.EndDate)
		invQuery := `
			SELECT product_code, transaction_date, yj_quantity
			FROM (
				SELECT
					jan_code AS product_code,
					transaction_date,
					SUM(yj_quantity) AS yj_quantity,
					ROW_NUMBER() OVER(PARTITION BY jan_code ORDER BY transaction_date DESC) as rn
				FROM transaction_records
				WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `) AND flag = 0 AND transaction_date BETWEEN ? AND ?
				GROUP BY jan_code, transaction_date
			)
			WHERE rn = 1
		`
		invRows, err := conn.Query(invQuery, invQueryArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to bulk query inventory: %w", err)
		}
		defer invRows.Close()

		for invRows.Next() {
			var pCode, date string
			var qty float64
			if err := invRows.Scan(&pCode, &date, &qty); err != nil {
				return nil, err
			}
			latestInventoryMap[pCode] = inventoryInfo{Date: date, Quantity: qty}
		}
	}

	var result []model.StockLedgerYJGroup
	for yjCode, mastersInYjGroup := range mastersByYjCode {
		if len(mastersInYjGroup) == 0 {
			continue
		}

		representativeProductName := mastersInYjGroup[0].ProductName
		for _, master := range mastersInYjGroup {
			isComplete := false
			if txs, ok := transactionsByProductCode[master.ProductCode]; ok {
				for _, t := range txs {
					if t.ProcessFlagMA == "COMPLETE" {
						isComplete = true
						break
					}
				}
			}
			if isComplete {
				representativeProductName = master.ProductName
				break
			}
		}

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
			pkgTxs := []*model.TransactionRecord{}
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

			var netChange, maxUsage float64
			var runningBalance float64
			var transactions []model.LedgerTransaction

			if !hasInventoryInGroup {
				pkg.StartingBalance = "期間棚卸なし"
				pkg.EndingBalance = "期間棚卸なし"
				runningBalance = 0.0
				for _, t := range pkgTxs {
					runningBalance += t.SignedYjQty()
					transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
				}
			} else {
				runningBalance = 0.0
				var startingBalanceQty float64
				isStartingBalanceSet := false

				for _, t := range pkgTxs {
					if t.Flag == 0 {
						if !isStartingBalanceSet {
							startingBalanceQty = inventoryDayTotals[t.TransactionDate]
							isStartingBalanceSet = true
						}
						break
					}
				}

				for i, t := range pkgTxs {
					_, isInventoryDay := inventoryDayTotals[t.TransactionDate]

					if i > 0 && t.TransactionDate != pkgTxs[i-1].TransactionDate {
						prevDate := pkgTxs[i-1].TransactionDate
						if invTotal, wasInvDay := inventoryDayTotals[prevDate]; wasInvDay {
							runningBalance = invTotal
						}
					}

					if isInventoryDay {
						if t.Flag != 0 {
							tempBalance := runningBalance + t.SignedYjQty()
							transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: tempBalance})
						} else {
							runningBalance = inventoryDayTotals[t.TransactionDate]
							transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
						}
					} else {
						runningBalance += t.SignedYjQty()
						transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
					}
				}

				if len(pkgTxs) > 0 {
					lastDate := pkgTxs[len(pkgTxs)-1].TransactionDate
					if invTotal, wasInvDay := inventoryDayTotals[lastDate]; wasInvDay {
						runningBalance = invTotal
						for i := len(transactions) - 1; i >= 0; i-- {
							if transactions[i].TransactionDate == lastDate {
								transactions[i].RunningBalance = runningBalance
							} else {
								break
							}
						}
					}
				}

				if isStartingBalanceSet {
					pkg.StartingBalance = startingBalanceQty
				} else {
					pkg.StartingBalance = "期間棚卸なし"
				}
				pkg.EndingBalance = runningBalance
			}

			for _, t := range pkgTxs {
				netChange += t.SignedYjQty()
				if t.Flag == 3 && t.YjQuantity > maxUsage {
					maxUsage = t.YjQuantity
				}
			}

			pkg.Transactions = transactions
			pkg.NetChange = netChange
			pkg.MaxUsage = maxUsage
			pkg.ReorderPoint = maxUsage * filters.Coefficient

			if endBalanceFloat, ok := pkg.EndingBalance.(float64); ok {
				pkg.IsReorderNeeded = endBalanceFloat < pkg.ReorderPoint && pkg.MaxUsage > 0
			}
			allPackageLedgers = append(allPackageLedgers, pkg)
		}

		if len(allPackageLedgers) > 0 {
			var yjTotalEnding, yjTotalNetChange, yjTotalReorderPoint float64
			var yjTotalStarting interface{}
			isYjReorderNeeded := false
			hasAnyInventory := false

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
				if pkg.IsReorderNeeded {
					isYjReorderNeeded = true
				}
			}

			if !hasAnyInventory {
				yjGroup.StartingBalance = "期間棚卸なし"
				yjGroup.EndingBalance = "期間棚卸なし"
			} else {
				yjGroup.StartingBalance = yjTotalStarting
				yjGroup.EndingBalance = yjTotalEnding
			}

			yjGroup.NetChange = yjTotalNetChange
			yjGroup.TotalReorderPoint = yjTotalReorderPoint
			yjGroup.IsReorderNeeded = isYjReorderNeeded
			yjGroup.PackageLedgers = allPackageLedgers
			result = append(result, yjGroup)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{"内": 1, "外": 2, "注": 3}

		masterI := mastersByYjCode[result[i].YjCode][0]
		masterJ := mastersByYjCode[result[j].YjCode][0]

		// ▼▼▼ [修正点] 診断用のログ出力を追加 ▼▼▼
		log.Printf("AGGREGATION SORT: Comparing A: '%s' (prio: %d) with B: '%s' (prio: %d)",
			masterI.UsageClassification, prio[masterI.UsageClassification],
			masterJ.UsageClassification, prio[masterJ.UsageClassification])
		// ▲▲▲ 修正ここまで ▲▲▲

		prioI, okI := prio[masterI.UsageClassification]
		if !okI {
			prioI = 4
		}

		prioJ, okJ := prio[masterJ.UsageClassification]
		if !okJ {
			prioJ = 4
		}

		if prioI != prioJ {
			return prioI < prioJ
		}
		return masterI.KanaName < masterJ.KanaName
	})

	return result, nil
}
