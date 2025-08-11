package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// GetStockLedger generates the stock ledger report, including detailed transactions.
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	// Step 1: Filter product masters based on filters.
	masterQuery := `SELECT ` + selectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND p.kana_name LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%")
	}
	// (Additional drugType filter logic would go here if implemented)

	masterRows, err := conn.Query(masterQuery, masterArgs...)
	if err != nil {
		return nil, fmt.Errorf("ledger master query failed: %w", err)
	}
	defer masterRows.Close()

	mastersByYjCode := make(map[string][]*model.ProductMaster)
	var productCodes []string
	productCodeSet := make(map[string]struct{})
	for masterRows.Next() {
		m, err := scanProductMaster(masterRows)
		if err != nil {
			return nil, err
		}
		if m.YjCode != "" {
			mastersByYjCode[m.YjCode] = append(mastersByYjCode[m.YjCode], m)
		}
		if _, ok := productCodeSet[m.ProductCode]; !ok {
			productCodeSet[m.ProductCode] = struct{}{}
			productCodes = append(productCodes, m.ProductCode)
		}
	}
	if len(productCodes) == 0 {
		return []model.StockLedgerYJGroup{}, nil
	}

	// Step 2: Get all relevant transactions for the filtered products.
	txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `)`
	var txArgs []interface{}
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
		return nil, fmt.Errorf("ledger transaction query failed: %w", err)
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

	// Step 3: Group, calculate, and build the full report structure.
	var result []model.StockLedgerYJGroup
	for yjCode, masters := range mastersByYjCode {
		yjGroup := model.StockLedgerYJGroup{
			YjCode:      yjCode,
			ProductName: masters[0].ProductName,
			YjUnitName:  units.ResolveName(masters[0].YjUnitName),
		}
		var yjTotalEnd float64

		for _, master := range masters {
			packageKey := fmt.Sprintf("%s|%g|%s", master.PackageSpec, master.JanPackInnerQty, master.YjUnitName)
			txs, hasTxs := transactionsByProductCode[master.ProductCode]
			if !hasTxs {
				continue
			}

			var currentBalance float64 = 0
			var ledgerTransactions []model.LedgerTransaction

			for _, t := range txs {
				signedYjQty := 0.0
				switch t.Flag {
				case 1, 4, 11:
					signedYjQty = t.YjQuantity // In
				case 2, 3, 5, 12:
					signedYjQty = -t.YjQuantity // Out
				}
				currentBalance += signedYjQty
				ledgerTransactions = append(ledgerTransactions, model.LedgerTransaction{
					TransactionRecord: *t,
					RunningBalance:    currentBalance,
				})
			}

			pkgLedger := model.StockLedgerPackageGroup{
				PackageKey:    packageKey,
				EndingBalance: currentBalance,
				Transactions:  ledgerTransactions,
			}
			yjGroup.PackageLedgers = append(yjGroup.PackageLedgers, pkgLedger)
			yjTotalEnd += pkgLedger.EndingBalance
		}

		if len(yjGroup.PackageLedgers) > 0 {
			yjGroup.EndingBalance = yjTotalEnd
			result = append(result, yjGroup)
		}
	}
	return result, nil
}
