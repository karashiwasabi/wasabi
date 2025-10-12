// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\dead_stock.go
package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	"wasabi/model"
)

func DeleteDeadStockByProductCodesInTx(tx *sql.Tx, productCodes []string) error {
	if len(productCodes) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf("DELETE FROM dead_stock_list WHERE product_code IN (%s)", placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete dead stock records by product codes: %w", err)
	}
	return nil
}

func GetDeadStockList(conn *sql.DB, filters model.DeadStockFilters) ([]model.DeadStockGroup, error) {
	currentStockMap, err := GetAllCurrentStockMap(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get current stock for dead stock list: %w", err)
	}

	lastUsageDateMap, err := getLastTransactionDateMap(conn, 3) // flag=3 は処方
	if err != nil {
		return nil, fmt.Errorf("failed to get last usage dates: %w", err)
	}

	var deadStockProductCodes []string
	for productCode, stock := range currentStockMap {
		if stock <= 0 {
			continue
		}
		lastUsageDate, ok := lastUsageDateMap[productCode]
		if !ok || lastUsageDate < filters.StartDate {
			deadStockProductCodes = append(deadStockProductCodes, productCode)
		}
	}

	if len(deadStockProductCodes) == 0 {
		return []model.DeadStockGroup{}, nil
	}

	mastersMap, err := GetProductMastersByCodesMap(conn, deadStockProductCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get masters for dead stock candidates: %w", err)
	}

	filteredMastersMap := make(map[string]*model.ProductMaster)
	for code, master := range mastersMap {
		keep := true
		if filters.KanaName != "" {
			if !strings.Contains(master.ProductName, filters.KanaName) && !strings.Contains(master.KanaName, filters.KanaName) {
				keep = false
			}
		}
		if filters.DosageForm != "" {
			if master.UsageClassification != filters.DosageForm {
				keep = false
			}
		}
		if filters.ShelfNumber != "" {
			if !strings.Contains(master.ShelfNumber, filters.ShelfNumber) {
				keep = false
			}
		}
		if keep {
			filteredMastersMap[code] = master
		}
	}

	var filteredProductCodes []string
	for code := range filteredMastersMap {
		filteredProductCodes = append(filteredProductCodes, code)
	}

	if len(filteredProductCodes) == 0 {
		return []model.DeadStockGroup{}, nil
	}

	deadStockRecordsMap, err := getDeadStockRecordsByProductCodes(conn, filteredProductCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get dead stock records for candidates: %w", err)
	}

	groups := make(map[string]*model.DeadStockGroup)
	for _, productCode := range filteredProductCodes {
		master, ok := filteredMastersMap[productCode]
		if !ok {
			continue
		}

		group, ok := groups[master.YjCode]
		if !ok {
			group = &model.DeadStockGroup{
				YjCode:      master.YjCode,
				ProductName: master.ProductName,
			}
			groups[master.YjCode] = group
		}

		packageKey := fmt.Sprintf("%s|%g|%s", master.PackageForm, master.JanPackInnerQty, master.YjUnitName)
		var pkgGroup *model.DeadStockPackageGroup
		for i := range group.PackageGroups {
			if group.PackageGroups[i].PackageKey == packageKey {
				pkgGroup = &group.PackageGroups[i]
				break
			}
		}
		if pkgGroup == nil {
			group.PackageGroups = append(group.PackageGroups, model.DeadStockPackageGroup{PackageKey: packageKey})
			pkgGroup = &group.PackageGroups[len(group.PackageGroups)-1]
		}

		dsProduct := model.DeadStockProduct{
			ProductMaster: *master,
			CurrentStock:  currentStockMap[productCode],
			SavedRecords:  deadStockRecordsMap[productCode],
			LastUsageDate: lastUsageDateMap[productCode],
		}

		pkgGroup.Products = append(pkgGroup.Products, dsProduct)
		pkgGroup.TotalStock += dsProduct.CurrentStock
		group.TotalStock += dsProduct.CurrentStock
	}

	var result []model.DeadStockGroup
	for _, group := range groups {
		result = append(result, *group)
	}

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "注": 3,
			"4": 4, "歯": 4, "5": 5, "機": 5, "6": 6, "他": 6,
		}

		var masterI, masterJ *model.ProductMaster
		if len(result[i].PackageGroups) > 0 && len(result[i].PackageGroups[0].Products) > 0 {
			masterI = &result[i].PackageGroups[0].Products[0].ProductMaster
		}
		if len(result[j].PackageGroups) > 0 && len(result[j].PackageGroups[0].Products) > 0 {
			masterJ = &result[j].PackageGroups[0].Products[0].ProductMaster
		}

		if masterI == nil || masterJ == nil {
			return result[i].YjCode < result[j].YjCode
		}

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

func getLastTransactionDateMap(conn *sql.DB, flag int) (map[string]string, error) {
	query := `SELECT jan_code, MAX(transaction_date) FROM transaction_records WHERE flag = ? GROUP BY jan_code`
	rows, err := conn.Query(query, flag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dateMap := make(map[string]string)
	for rows.Next() {
		var janCode, date string
		if err := rows.Scan(&janCode, &date); err != nil {
			return nil, err
		}
		dateMap[janCode] = date
	}
	return dateMap, nil
}

func getDeadStockRecordsByProductCodes(conn *sql.DB, productCodes []string) (map[string][]model.DeadStockRecord, error) {
	recordsMap := make(map[string][]model.DeadStockRecord)
	if len(productCodes) == 0 {
		return recordsMap, nil
	}

	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf(`SELECT id, product_code, stock_quantity_jan, expiry_date, lot_number FROM dead_stock_list WHERE product_code IN (%s)`, placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r model.DeadStockRecord
		if err := rows.Scan(&r.ID, &r.ProductCode, &r.StockQuantityJan, &r.ExpiryDate, &r.LotNumber); err != nil {
			return nil, err
		}
		recordsMap[r.ProductCode] = append(recordsMap[r.ProductCode], r)
	}
	return recordsMap, nil
}

func SaveDeadStockListInTx(tx *sql.Tx, records []model.DeadStockRecord) error {
	const q = `
        INSERT OR REPLACE INTO dead_stock_list 
        (product_code, yj_code, package_form, jan_pack_inner_qty, yj_unit_name, 
        stock_quantity_jan, expiry_date, lot_number, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for dead_stock_list: %w", err)
	}
	defer stmt.Close()

	createdAt := time.Now().Format("2006-01-02 15:04:05")

	for _, rec := range records {
		_, err := stmt.Exec(
			rec.ProductCode, rec.YjCode, rec.PackageForm, rec.JanPackInnerQty, rec.YjUnitName,
			rec.StockQuantityJan, rec.ExpiryDate, rec.LotNumber, createdAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert/replace dead_stock_list for product %s: %w", rec.ProductCode, err)
		}
	}
	return nil
}

func GetDeadStockByYjCode(tx *sql.Tx, yjCode string) ([]model.DeadStockRecord, error) {
	const q = `
		SELECT id, product_code, stock_quantity_jan, expiry_date, lot_number 
		FROM dead_stock_list 
		WHERE yj_code = ? 
		ORDER BY product_code, expiry_date, lot_number`

	rows, err := tx.Query(q, yjCode)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead stock by yj_code: %w", err)
	}
	defer rows.Close()

	var records []model.DeadStockRecord
	for rows.Next() {
		var r model.DeadStockRecord
		if err := rows.Scan(&r.ID, &r.ProductCode, &r.StockQuantityJan, &r.ExpiryDate, &r.LotNumber); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}
