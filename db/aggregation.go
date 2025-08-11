package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// GetStockLedger generates the stock ledger report based on the finalized logic.
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	// 1. Get and group product masters by YJ code, applying new filters
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
			"poison": "p.flag_poison = 1", "deleterious": "p.flag_deleterious = 1",
			"narcotic": "p.flag_narcotic = 1", "psychotropic": "p.flag_psychotropic > 0",
			"stimulant": "p.flag_stimulant = 1", "stimulant_raw": "p.flag_stimulant_raw = 1",
		}
		for _, dt := range filters.DrugTypes {
			if condition, ok := flagMap[dt]; ok {
				conditions = append(conditions, condition)
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
	productCodeToMaster := make(map[string]*model.ProductMaster)
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
		productCodeToMaster[m.ProductCode] = m
	}
	if len(productCodes) == 0 {
		return []model.StockLedgerYJGroup{}, nil
	}

	// 2. Fetch all relevant transactions
	txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `)`
	txArgs := make([]interface{}, 0, len(productCodes)+2)
	for _, pc := range productCodes {
		txArgs = append(txArgs, pc)
	}
	if filters.StartDate != "" {
		txQuery += " AND transaction_date >= ? "
		txArgs = append(txArgs, filters.StartDate)
	}
	if filters.EndDate != "" {
		txQuery += " AND transaction_date <= ? "
		txArgs = append(txArgs, filters.EndDate)
	}
	txQuery += " ORDER BY transaction_date, id"

	txRows, err := conn.Query(txQuery, txArgs...)
	if err != nil {
		return nil, err
	}
	defer txRows.Close()

	transactionsByYjCode := make(map[string][]*model.TransactionRecord)
	for txRows.Next() {
		t, err := ScanTransactionRecord(txRows)
		if err != nil {
			return nil, err
		}
		if master, ok := productCodeToMaster[t.JanCode]; ok {
			transactionsByYjCode[master.YjCode] = append(transactionsByYjCode[master.YjCode], t)
		}
	}

	// 3. Process each YJ group
	var result []model.StockLedgerYJGroup
	for yjCode, masters := range mastersByYjCode {
		allTxsForYj, ok := transactionsByYjCode[yjCode]
		if !ok || len(allTxsForYj) == 0 {
			continue
		}

		latestInventoryDate, inventorySumOnDate := findLatestInventoryInfo(allTxsForYj)
		var allLedgerTransactions []model.LedgerTransaction
		currentYjBalance := calculateRunningBalance(allTxsForYj, latestInventoryDate, inventorySumOnDate, &allLedgerTransactions)

		packageLedgersMap := make(map[string]*model.StockLedgerPackageGroup)
		for _, m := range masters {
			key := fmt.Sprintf("%s|%g|%s", m.PackageSpec, m.JanPackInnerQty, m.YjUnitName)
			packageLedgersMap[key] = &model.StockLedgerPackageGroup{PackageKey: key, Master: m}
		}
		for _, lt := range allLedgerTransactions {
			master := productCodeToMaster[lt.JanCode]
			key := fmt.Sprintf("%s|%g|%s", master.PackageSpec, master.JanPackInnerQty, master.YjUnitName)
			if pkg, ok := packageLedgersMap[key]; ok {
				pkg.Transactions = append(pkg.Transactions, lt)
			}
		}

		var finalPackageLedgers []model.StockLedgerPackageGroup
		var yjTotalNetChange, yjTotalReorderPoint float64
		isYjReorderNeeded := false
		for _, pkg := range packageLedgersMap {
			if len(pkg.Transactions) > 0 {
				// ▼▼▼ [修正点] 未使用だったpkgStartingBalance変数を削除 ▼▼▼
				var pkgNetChange, maxUsage float64

				firstTxInPkg := pkg.Transactions[0]
				var balanceBeforePkgFirstTx float64
				for _, yjTx := range allLedgerTransactions {
					if yjTx.ID == firstTxInPkg.ID {
						balanceBeforePkgFirstTx = yjTx.RunningBalance - yjTx.SignedYjQty()
						break
					}
				}
				pkg.StartingBalance = balanceBeforePkgFirstTx

				for _, t := range pkg.Transactions {
					pkgNetChange += t.SignedYjQty()
					if t.Flag == 3 && t.YjQuantity > maxUsage {
						maxUsage = t.YjQuantity
					}
				}

				pkg.NetChange = pkgNetChange
				pkg.EndingBalance = pkg.StartingBalance + pkg.NetChange
				pkg.MaxUsage = maxUsage
				pkg.ReorderPoint = maxUsage * filters.Coefficient
				pkg.IsReorderNeeded = pkg.EndingBalance < pkg.ReorderPoint && pkg.MaxUsage > 0

				yjTotalNetChange += pkg.NetChange
				yjTotalReorderPoint += pkg.ReorderPoint
				if pkg.IsReorderNeeded {
					isYjReorderNeeded = true
				}
				finalPackageLedgers = append(finalPackageLedgers, *pkg)
			}
		}
		sort.Slice(finalPackageLedgers, func(i, j int) bool { return finalPackageLedgers[i].PackageKey < finalPackageLedgers[j].PackageKey })

		result = append(result, model.StockLedgerYJGroup{
			YjCode: yjCode, ProductName: masters[0].ProductName, YjUnitName: units.ResolveName(masters[0].YjUnitName),
			EndingBalance: currentYjBalance, NetChange: yjTotalNetChange, TotalReorderPoint: yjTotalReorderPoint,
			IsReorderNeeded: isYjReorderNeeded, PackageLedgers: finalPackageLedgers,
		})
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

// findLatestInventoryInfo finds the date and total quantity of the latest inventory record.
func findLatestInventoryInfo(transactions []*model.TransactionRecord) (date string, sum float64) {
	for i := len(transactions) - 1; i >= 0; i-- {
		if transactions[i].Flag == 0 {
			date = transactions[i].TransactionDate
			break
		}
	}
	if date != "" {
		for _, t := range transactions {
			if t.TransactionDate == date && t.Flag == 0 {
				sum += t.YjQuantity
			}
		}
	}
	return date, sum
}

// calculateRunningBalance calculates the running balance for a list of transactions, pivoting on the inventory date.
func calculateRunningBalance(transactions []*model.TransactionRecord, pivotDate string, pivotBalance float64, ledger *[]model.LedgerTransaction) float64 {
	currentBalance := 0.0
	sort.SliceStable(transactions, func(i, j int) bool { // Sort to ensure proper processing order
		if transactions[i].TransactionDate != transactions[j].TransactionDate {
			return transactions[i].TransactionDate < transactions[j].TransactionDate
		}
		return transactions[i].ID < transactions[j].ID
	})

	if pivotDate == "" {
		for _, t := range transactions {
			currentBalance += t.SignedYjQty()
			*ledger = append(*ledger, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
		}
	} else {
		for _, t := range transactions {
			if t.TransactionDate < pivotDate {
				currentBalance += t.SignedYjQty()
				*ledger = append(*ledger, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
			}
		}
		for _, t := range transactions {
			if t.TransactionDate == pivotDate && t.Flag != 0 {
				currentBalance += t.SignedYjQty()
				*ledger = append(*ledger, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
			}
		}

		currentBalance = pivotBalance // Overwrite balance

		for _, t := range transactions {
			if t.TransactionDate == pivotDate && t.Flag == 0 {
				*ledger = append(*ledger, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
			}
		}

		for _, t := range transactions {
			if t.TransactionDate > pivotDate {
				currentBalance += t.SignedYjQty()
				*ledger = append(*ledger, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: currentBalance})
			}
		}
	}

	// Re-sort the final ledger to be strictly chronological for display
	sort.SliceStable(*ledger, func(i, j int) bool {
		if (*ledger)[i].TransactionDate != (*ledger)[j].TransactionDate {
			return (*ledger)[i].TransactionDate < (*ledger)[j].TransactionDate
		}
		return (*ledger)[i].ID < (*ledger)[j].ID
	})

	return currentBalance
}
