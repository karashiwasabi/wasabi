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
	txRows, err := tx.Query(`SELECT ` + TransactionColumns + ` FROM transaction_records ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions for dead stock: %w", err)
	}
	defer txRows.Close()

	txsByProductCode := make(map[string][]*model.TransactionRecord)
	for txRows.Next() {
		r, err := ScanTransactionRecord(txRows)
		if err != nil {
			return nil, err
		}
		txsByProductCode[r.JanCode] = append(txsByProductCode[r.JanCode], r)
	}

	usedInPeriod := make(map[string]bool)
	for productCode, txs := range txsByProductCode {
		for _, t := range txs {
			if t.Flag == 3 && t.TransactionDate >= filters.StartDate && t.TransactionDate <= filters.EndDate {
				usedInPeriod[productCode] = true
				break
			}
		}
	}

	// ▼▼▼ [修正点] マスター取得時に絞り込みを行う ▼▼▼
	masterQuery := `SELECT ` + SelectColumns + ` FROM product_master WHERE 1=1`
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND (kana_name LIKE ? OR product_name LIKE ?)"
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%", "%"+filters.KanaName+"%")
	}
	if filters.DosageForm != "" {
		masterQuery += " AND usage_classification LIKE ?"
		masterArgs = append(masterArgs, "%"+filters.DosageForm+"%")
	}

	masterRows, err := tx.Query(masterQuery, masterArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtered product masters for dead stock: %w", err)
	}
	defer masterRows.Close()

	var allMasters []*model.ProductMaster
	for masterRows.Next() {
		m, err := ScanProductMaster(masterRows)
		if err != nil {
			return nil, err
		}
		allMasters = append(allMasters, m)
	}
	// ▲▲▲ 修正ここまで ▲▲▲

	groups := make(map[string][]*model.ProductMaster)
	for _, m := range allMasters {
		if m.YjCode != "" {
			groups[m.YjCode] = append(groups[m.YjCode], m)
		}
	}

	var result []model.DeadStockGroup
	for yjCode, masterList := range groups {
		if len(masterList) == 0 {
			continue
		}

		isYjGroupUsed := false
		var representativeMaster *model.ProductMaster
		for _, master := range masterList {
			if representativeMaster == nil {
				representativeMaster = master // ソート用に代表マスターを保持
			}
			if usedInPeriod[master.ProductCode] {
				isYjGroupUsed = true
				break
			}
		}
		if isYjGroupUsed {
			continue
		}

		dsg := model.DeadStockGroup{
			YjCode:      yjCode,
			ProductName: representativeMaster.ProductName,
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
					return nil, fmt.Errorf("failed to calculate stock for dead stock list (%s): %w", master.ProductCode, err)
				}

				if filters.ExcludeZeroStock && stock <= 0 {
					continue
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

		if (filters.ExcludeZeroStock && dsg.TotalStock > 0) || !filters.ExcludeZeroStock {
			if len(dsg.PackageGroups) > 0 {
				result = append(result, dsg)
			}
		}
	}

	// ▼▼▼ [修正点] 正しい並び順でソートするロジックを実装 ▼▼▼
	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3,
			"4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6,
		}

		// 各YJグループから代表マスターを取得して比較
		masterI := groups[result[i].YjCode][0]
		masterJ := groups[result[j].YjCode][0]

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
	// ▲▲▲ 修正ここまで ▲▲▲

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
