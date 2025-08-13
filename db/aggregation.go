package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// getStartingBalanceForProduct calculates the starting balance for a single product code before a given start date.
func getStartingBalanceForProduct(conn *sql.DB, productCode string, startDate string) (float64, error) {
	var startingBalance float64
	var lastInvDate sql.NullString

	// 1. Find the latest inventory date for this product before the start date.
	invDateQuery := `SELECT MAX(transaction_date) FROM transaction_records WHERE jan_code = ? AND flag = 0 AND transaction_date < ?`
	err := conn.QueryRow(invDateQuery, productCode, startDate).Scan(&lastInvDate)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get latest inventory date for %s: %w", productCode, err)
	}

	if lastInvDate.Valid && lastInvDate.String != "" {
		// 2a. If inventory exists, get the balance at that time.
		var invBalance sql.NullFloat64
		invQuery := `SELECT SUM(yj_quantity) FROM transaction_records WHERE jan_code = ? AND flag = 0 AND transaction_date = ?`
		err := conn.QueryRow(invQuery, productCode, lastInvDate.String).Scan(&invBalance)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to get inventory balance for %s: %w", productCode, err)
		}
		startingBalance = invBalance.Float64

		// 2b. Add the net change from that inventory date to the start date.
		var netChange sql.NullFloat64
		changeQuery := `
			SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
			FROM transaction_records WHERE jan_code = ? AND flag != 0 AND transaction_date > ? AND transaction_date < ?`
		err = conn.QueryRow(changeQuery, productCode, lastInvDate.String, startDate).Scan(&netChange)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to calculate net change post-inventory for %s: %w", productCode, err)
		}
		startingBalance += netChange.Float64
	} else {
		// 3. If no prior inventory exists, calculate net change from the beginning of time.
		var netChange sql.NullFloat64
		changeQuery := `
			SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
			FROM transaction_records WHERE jan_code = ? AND transaction_date < ?`
		err = conn.QueryRow(changeQuery, productCode, startDate).Scan(&netChange)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to calculate total net change for %s: %w", productCode, err)
		}
		startingBalance = netChange.Float64
	}

	return startingBalance, nil
}

// GetStockLedger generates the stock ledger report with correct, bottom-up calculations.
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	// 1. Get filtered product masters
	masterQuery := `SELECT ` + selectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}
	// (Filter logic remains the same)
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

	// 2. Fetch transactions for the period
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
	for yjCode, masters := range mastersByYjCode {
		yjGroup := model.StockLedgerYJGroup{YjCode: yjCode, ProductName: masters[0].ProductName, YjUnitName: units.ResolveName(masters[0].YjUnitName)}
		var allPackageLedgers []model.StockLedgerPackageGroup

		// Calculate each package group independently
		for _, master := range masters {
			pkgTxs := transactionsByProductCode[master.ProductCode]

			startingBalance, err := getStartingBalanceForProduct(conn, master.ProductCode, filters.StartDate)
			if err != nil {
				return nil, err
			}

			pkg := model.StockLedgerPackageGroup{
				PackageKey:      fmt.Sprintf("%s|%g|%s", master.PackageSpec, master.JanPackInnerQty, master.YjUnitName),
				StartingBalance: startingBalance,
			}

			// Calculate running balance, net change, and other metrics for this package
			currentBalance := startingBalance
			var netChange, maxUsage float64
			inventoryAppliedForDate := make(map[string]bool)

			for _, t := range pkgTxs {
				if t.Flag == 0 { // Inventory record
					if !inventoryAppliedForDate[t.TransactionDate] {
						var dailyNetChange, dailyInventorySum float64
						for _, t2 := range pkgTxs {
							if t2.TransactionDate == t.TransactionDate {
								if t2.Flag != 0 {
									dailyNetChange += t2.SignedYjQty()
								} else {
									dailyInventorySum += t2.YjQuantity
								}
							}
						}
						// Undo the daily changes and apply the definitive inventory count
						currentBalance = (currentBalance - dailyNetChange) + dailyInventorySum
						inventoryAppliedForDate[t.TransactionDate] = true
					}
				} else { // Operational transaction
					currentBalance += t.SignedYjQty()
					netChange += t.SignedYjQty()
				}

				if t.Flag == 3 && t.YjQuantity > maxUsage {
					maxUsage = t.YjQuantity
				}
				pkg.Transactions = append(pkg.Transactions, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
			}

			pkg.EndingBalance = currentBalance
			pkg.NetChange = netChange
			pkg.MaxUsage = maxUsage
			pkg.ReorderPoint = maxUsage * filters.Coefficient
			pkg.IsReorderNeeded = pkg.EndingBalance < pkg.ReorderPoint && pkg.MaxUsage > 0

			allPackageLedgers = append(allPackageLedgers, pkg)
		}

		// Sum up the package group totals for the YJ group summary
		if len(allPackageLedgers) > 0 {
			var yjTotalStarting, yjTotalEnding, yjTotalNetChange, yjTotalReorderPoint float64
			isYjReorderNeeded := false
			for _, pkg := range allPackageLedgers {
				yjTotalStarting += pkg.StartingBalance
				yjTotalEnding += pkg.EndingBalance
				yjTotalNetChange += pkg.NetChange
				yjTotalReorderPoint += pkg.ReorderPoint
				if pkg.IsReorderNeeded {
					isYjReorderNeeded = true
				}
			}
			yjGroup.StartingBalance = yjTotalStarting
			yjGroup.EndingBalance = yjTotalEnding
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
