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

		// ▼▼▼ [修正点] ご指摘のエラー箇所を修正 ▼▼▼
		_, ok := masters[r.JanCode]
		if !ok {
			masters[r.JanCode] = r.ToProductMaster()
		}
		// ▲▲▲ 修正ここまで ▲▲▲
	}

	groups := make(map[string][]*model.ProductMaster)
	for _, m := range masters {
		if m.YjCode != "" {
			groups[m.YjCode] = append(groups[m.YjCode], m)
		}
	}

	deadStockMajorGroups := make(map[string]bool)
	for yjCode, masterList := range groups {
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

			if maxUsage*filters.Coefficient == 0 {
				isDeadStockCandidate = true
				break
			}
		}

		if isDeadStockCandidate {
			deadStockMajorGroups[yjCode] = true
		}
	}

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

		var yjTotalStock float64
		var finalPackageGroups []model.DeadStockPackageGroup

		for key, mastersInMinorGroup := range packagesByMinorGroupKey {
			if len(mastersInMinorGroup) == 0 {
				continue
			}

			var packageGroupTotalStock float64
			var productsInPackageGroup []model.DeadStockProduct

			for _, master := range mastersInMinorGroup {
				stock, err := CalculateCurrentStockForProduct(tx, master.ProductCode)
				if err != nil {
					return nil, fmt.Errorf("failed to calculate stock for dead stock list: %w", err)
				}

				savedRecords, err := getSavedDeadStock(tx, master.ProductCode)
				if err != nil {
					return nil, err
				}

				productsInPackageGroup = append(productsInPackageGroup, model.DeadStockProduct{
					ProductMaster: *master,
					CurrentStock:  stock,
					SavedRecords:  savedRecords,
				})
				packageGroupTotalStock += stock
			}

			if len(productsInPackageGroup) > 0 {
				finalPackageGroups = append(finalPackageGroups, model.DeadStockPackageGroup{
					PackageKey: key,
					TotalStock: packageGroupTotalStock,
					Products:   productsInPackageGroup,
				})
				yjTotalStock += packageGroupTotalStock
			}
		}

		dsg.TotalStock = yjTotalStock
		dsg.PackageGroups = finalPackageGroups

		if filters.ExcludeZeroStock && dsg.TotalStock <= 0 {
			continue
		}

		if len(dsg.PackageGroups) > 0 {
			result = append(result, dsg)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1,
			"2": 2, "外": 2,
			"3": 3, "歯": 3,
			"4": 4, "注": 4,
			"5": 5, "機": 5,
			"6": 6, "他": 6,
		}

		mastersI := groups[result[i].YjCode]
		mastersJ := groups[result[j].YjCode]
		if len(mastersI) == 0 || len(mastersJ) == 0 {
			return false
		}
		masterI := mastersI[0]
		masterJ := mastersJ[0]

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