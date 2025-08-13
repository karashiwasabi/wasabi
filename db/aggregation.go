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

// GetStockLedger generates the stock ledger report with the new, simplified calculation logic.
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	// 1. Get filtered product masters
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

	// 2. Fetch all transactions for the relevant products within the period
	txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `) AND transaction_date >= ? AND transaction_date <= ? ORDER BY transaction_date, id`
	txArgs := make([]interface{}, 0, len(productCodes)+2)
	for _, pc := range productCodes {
		txArgs = append(txArgs, pc)
	}
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

	// 3. Process YJ groups
	var result []model.StockLedgerYJGroup
	for yjCode, mastersInYjGroup := range mastersByYjCode {
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

			// ▼▼▼ [修正点] ご指示の最終確定ロジックを実装 ▼▼▼
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
				// PATTERN A: No inventory in group
				pkg.StartingBalance = "期間棚卸なし"
				pkg.EndingBalance = "期間棚卸なし"
				runningBalance = 0.0
				for _, t := range pkgTxs {
					runningBalance += t.SignedYjQty()
					transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
				}
			} else {
				// PATTERN B: Inventory exists in group
				runningBalance = 0.0
				var startingBalanceQty float64
				isStartingBalanceSet := false

				// Set starting balance to the quantity of the first chronological inventory
				for _, t := range pkgTxs {
					if t.Flag == 0 {
						startingBalanceQty = inventoryDayTotals[t.TransactionDate]
						isStartingBalanceSet = true
						break
					}
				}

				for i, t := range pkgTxs {
					// Determine if current day is an inventory day
					_, isInventoryDay := inventoryDayTotals[t.TransactionDate]

					if i > 0 && t.TransactionDate != pkgTxs[i-1].TransactionDate {
						// New day started, check if previous day was an inventory day and set balance
						prevDate := pkgTxs[i-1].TransactionDate
						if invTotal, wasInvDay := inventoryDayTotals[prevDate]; wasInvDay {
							runningBalance = invTotal
						}
					}

					// On inventory days, non-inventory transactions don't affect the balance.
					if isInventoryDay {
						if t.Flag != 0 {
							// For display, just show the change from the previous line, but don't alter the day's final balance
							tempBalance := runningBalance + t.SignedYjQty()
							transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: tempBalance})
							// DO NOT update runningBalance here
						} else {
							// This is an inventory record. The balance becomes the day's total inventory.
							runningBalance = inventoryDayTotals[t.TransactionDate]
							transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
						}
					} else {
						// Not an inventory day, normal calculation
						runningBalance += t.SignedYjQty()
						transactions = append(transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
					}
				}
				// Final correction for the last day in the list
				if len(pkgTxs) > 0 {
					lastDate := pkgTxs[len(pkgTxs)-1].TransactionDate
					if invTotal, wasInvDay := inventoryDayTotals[lastDate]; wasInvDay {
						runningBalance = invTotal
						// Correct the running balance of all transactions on the last day
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
					pkg.StartingBalance = "期間棚卸なし" // Should not happen in this branch, but for safety
				}
				pkg.EndingBalance = runningBalance
			}

			// Calculate NetChange and MaxUsage across all transactions regardless of inventory date
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
			// ▲▲▲ 修正ここまで ▲▲▲
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

	// 4. Final sort
	sort.Slice(result, func(i, j int) bool {
		masterI := mastersByYjCode[result[i].YjCode][0]
		masterJ := mastersByYjCode[result[j].YjCode][0]
		if masterI.UsageClassification != masterJ.UsageClassification {
			return masterI.UsageClassification < masterJ.UsageClassification
		}
		return masterI.KanaName < masterJ.KanaName
	})

	return result, nil
}
