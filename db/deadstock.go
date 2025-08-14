// C:\Dev\WASABI\db\deadstock.go

package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	"wasabi/model"
)

func GetDeadStockList(tx *sql.Tx, filters model.DeadStockFilters) ([]model.DeadStockGroup, error) {
	// 1. Get all transactions to build a master list and for stock calculation
	rows, err := tx.Query(`SELECT ` + TransactionColumns + ` FROM transaction_records ORDER BY transaction_date, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions: %w", err)
	}
	defer rows.Close()

	txsByProductCode := make(map[string][]*model.TransactionRecord)
	masters := make(map[string]*model.ProductMaster)
	for rows.Next() {
		r, err := ScanTransactionRecord(rows)
		if err != nil {
			return nil, err
		}
		txsByProductCode[r.JanCode] = append(txsByProductCode[r.JanCode], r)
		if _, ok := masters[r.JanCode]; !ok {
			masters[r.JanCode] = r.ToProductMaster()
		}
	}

	// 2. Group masters by YjCode (Major Group)
	groups := make(map[string][]*model.ProductMaster)
	for _, m := range masters {
		if m.YjCode != "" {
			groups[m.YjCode] = append(groups[m.YjCode], m)
		}
	}

	// ▼▼▼ [修正点] ご指示の最終ロジックに書き換え ▼▼▼
	// 3. Determine which major groups are dead stock candidates
	deadStockMajorGroups := make(map[string]bool)
	for yjCode, masterList := range groups {
		// Group masters by minor key (package) to calculate maxUsage per package
		packagesByMinorGroupKey := make(map[string][]*model.ProductMaster)
		for _, m := range masterList {
			key := fmt.Sprintf("%s|%g|%s", m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			packagesByMinorGroupKey[key] = append(packagesByMinorGroupKey[key], m)
		}

		isDeadStockCandidate := false
		for _, mastersInMinorGroup := range packagesByMinorGroupKey {
			var maxUsage float64
			for _, master := range mastersInMinorGroup {
				for _, t := range txsByProductCode[master.ProductCode] {
					if t.Flag == 3 && t.TransactionDate >= filters.StartDate && t.TransactionDate <= filters.EndDate {
						if t.YjQuantity > maxUsage {
							maxUsage = t.YjQuantity
						}
					}
				}
			}

			// If reorder point is 0 for any minor group, the whole major group is a candidate
			if maxUsage*filters.Coefficient == 0 {
				isDeadStockCandidate = true
				break
			}
		}

		if isDeadStockCandidate {
			deadStockMajorGroups[yjCode] = true
		}
	}

	// 4. Build the final result for the candidate groups
	var result []model.DeadStockGroup
	for yjCode, masterList := range groups {
		if !deadStockMajorGroups[yjCode] {
			continue
		}
		if len(masterList) == 0 {
			continue
		}

		dsg := model.DeadStockGroup{
			YjCode:      yjCode,
			ProductName: masterList[0].ProductName,
		}

		packagesByMinorGroupKey := make(map[string][]*model.ProductMaster)
		for _, m := range masterList {
			key := fmt.Sprintf("%s|%g|%s", m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			packagesByMinorGroupKey[key] = append(packagesByMinorGroupKey[key], m)
		}

		var totalStock float64
		var finalPackages []model.DeadStockPackage

		for _, mastersInMinorGroup := range packagesByMinorGroupKey {
			if len(mastersInMinorGroup) == 0 {
				continue
			}

			var aggregatedStock float64
			var aggregatedSavedRecords []model.DeadStockRecord

			for _, master := range mastersInMinorGroup {
				stock, _ := calculateCurrentStock(txsByProductCode[master.ProductCode])
				aggregatedStock += stock

				savedRecords, err := getSavedDeadStock(tx, master.ProductCode)
				if err != nil {
					return nil, err
				}
				aggregatedSavedRecords = append(aggregatedSavedRecords, savedRecords...)
			}

			repMaster := mastersInMinorGroup[0]
			finalPackages = append(finalPackages, model.DeadStockPackage{
				ProductMaster: *repMaster,
				CurrentStock:  aggregatedStock,
				SavedRecords:  aggregatedSavedRecords,
			})
			totalStock += aggregatedStock
		}

		dsg.TotalStock = totalStock
		dsg.Packages = finalPackages

		if filters.ExcludeZeroStock && dsg.TotalStock <= 0 {
			continue
		}

		if len(dsg.Packages) > 0 {
			result = append(result, dsg)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{"内": 1, "外": 2, "注": 3}

		if len(result[i].Packages) == 0 || len(result[j].Packages) == 0 {
			return false
		}
		masterI := result[i].Packages[0].ProductMaster
		masterJ := result[j].Packages[0].ProductMaster

		// Trim space before lookup for robustness
		prioI, okI := prio[strings.TrimSpace(masterI.UsageClassification)]
		if !okI {
			prioI = 4
		}

		prioJ, okJ := prio[strings.TrimSpace(masterJ.UsageClassification)]
		if !okJ {
			prioJ = 4
		}

		if prioI != prioJ {
			return prioI < prioJ
		}
		return masterI.KanaName < masterJ.KanaName
	})
	// ▲▲▲ 修正ここまで ▲▲▲

	return result, nil
}

// (calculateCurrentStock, getSavedDeadStock, UpsertDeadStockRecordsInTx functions are unchanged)
func calculateCurrentStock(txs []*model.TransactionRecord) (float64, error) {
	inventoryDayTotals := make(map[string]float64)
	hasInventory := false
	for _, t := range txs {
		if t.Flag == 0 {
			inventoryDayTotals[t.TransactionDate] += t.YjQuantity
			hasInventory = true
		}
	}
	var runningBalance float64
	if !hasInventory {
		for _, t := range txs {
			runningBalance += t.SignedYjQty()
		}
	} else {
		for i, t := range txs {
			if i > 0 && t.TransactionDate != txs[i-1].TransactionDate {
				prevDate := txs[i-1].TransactionDate
				if invTotal, wasInvDay := inventoryDayTotals[prevDate]; wasInvDay {
					runningBalance = invTotal
				}
			}
			if invTotal, isInvDay := inventoryDayTotals[t.TransactionDate]; isInvDay {
				if t.Flag == 0 {
					runningBalance = invTotal
				}
			} else {
				runningBalance += t.SignedYjQty()
			}
		}
		if len(txs) > 0 {
			lastDate := txs[len(txs)-1].TransactionDate
			if invTotal, wasInvDay := inventoryDayTotals[lastDate]; wasInvDay {
				runningBalance = invTotal
			}
		}
	}
	return runningBalance, nil
}
func getSavedDeadStock(tx *sql.Tx, productCode string) ([]model.DeadStockRecord, error) {
	const q = `SELECT id, stock_quantity_jan, expiry_date, lot_number FROM dead_stock_list WHERE product_code = ? ORDER BY id`
	rows, err := tx.Query(q, productCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.DeadStockRecord
	for rows.Next() {
		var r model.DeadStockRecord
		if err := rows.Scan(&r.ID, &r.StockQuantityJan, &r.ExpiryDate, &r.LotNumber); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}
func UpsertDeadStockRecordsInTx(tx *sql.Tx, records []model.DeadStockRecord) error {
	productCodes := make(map[string]struct{})
	for _, r := range records {
		productCodes[r.ProductCode] = struct{}{}
	}

	if len(productCodes) > 0 {
		var args []interface{}
		var placeholders []string
		for pc := range productCodes {
			args = append(args, pc)
			placeholders = append(placeholders, "?")
		}
		deleteQuery := `DELETE FROM dead_stock_list WHERE product_code IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := tx.Exec(deleteQuery, args...); err != nil {
			return fmt.Errorf("failed to delete old dead stock records: %w", err)
		}
	}

	const q = `INSERT INTO dead_stock_list (
		product_code, yj_code, package_form, jan_pack_inner_qty, yj_unit_name,
		stock_quantity_jan, expiry_date, lot_number, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare dead_stock_list statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range records {
		if r.StockQuantityJan <= 0 && r.ExpiryDate == "" && r.LotNumber == "" {
			continue
		}
		_, err := stmt.Exec(
			r.ProductCode, r.YjCode, r.PackageForm, r.JanPackInnerQty, r.YjUnitName,
			r.StockQuantityJan, r.ExpiryDate, r.LotNumber, time.Now().Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			return fmt.Errorf("failed to insert into dead_stock_list: %w", err)
		}
	}
	return nil
}
