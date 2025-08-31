// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\deadstock.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"wasabi/model"
)

/**
 * @brief 不動在庫リストを生成します。
 * @param tx SQLトランザクションオブジェクト
 * @param filters 絞り込み条件
 * @return []model.DeadStockGroup 不動在庫リストの集計結果
 * @return error 処理中にエラーが発生した場合
 * @details
 * 以下のステップで不動在庫リストを生成します。
 * 1. フィルタ条件に合致する製品マスターの候補を全て取得します。
 * 2. 指定された期間内に処方された（flag=3）製品のリストを取得します。
 * 3. 1の候補リストから2のリストに含まれる製品を除外し、「不動在庫」の製品リストを確定します。
 * 4. 不動在庫品目の現在庫と、保存済みのロット・期限情報を取得します。
 * 5. 最終的な表示形式（YJコード > 包装単位 > 個別JAN）に整形して返却します。
 */
func GetDeadStockList(tx *sql.Tx, filters model.DeadStockFilters) ([]model.DeadStockGroup, error) {
	log.Println("--- Dead Stock List Generation Start ---")

	// ステップ1: フィルターに合致する製品マスター候補を全て取得
	masterQuery := `SELECT ` + SelectColumns + ` FROM product_master WHERE 1=1`
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND (kana_name LIKE ? OR product_name LIKE ?)"
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%", "%"+filters.KanaName+"%")
	}
	if filters.DosageForm != "" {
		masterQuery += " AND usage_classification = ?"
		masterArgs = append(masterArgs, filters.DosageForm)
	}

	masterRows, err := tx.Query(masterQuery, masterArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtered product masters for dead stock: %w", err)
	}
	defer masterRows.Close()

	var candidateMasters []*model.ProductMaster
	for masterRows.Next() {
		m, err := ScanProductMaster(masterRows)
		if err != nil {
			return nil, err
		}
		candidateMasters = append(candidateMasters, m)
	}

	if len(candidateMasters) == 0 {
		return []model.DeadStockGroup{}, nil
	}

	// ステップ2: "期間に処方無し"の製品を特定
	usedInPeriod := make(map[string]bool)
	usageQuery := `SELECT DISTINCT jan_code FROM transaction_records WHERE flag = 3 AND transaction_date BETWEEN ? AND ?`
	usageRows, err := tx.Query(usageQuery, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get used products in period: %w", err)
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var janCode string
		if err := usageRows.Scan(&janCode); err == nil {
			usedInPeriod[janCode] = true
		}
	}

	var deadStockMasters []*model.ProductMaster
	var deadStockProductCodes []string
	for _, master := range candidateMasters {
		if !usedInPeriod[master.ProductCode] {
			deadStockMasters = append(deadStockMasters, master)
			deadStockProductCodes = append(deadStockProductCodes, master.ProductCode)
		}
	}

	if len(deadStockMasters) == 0 {
		return []model.DeadStockGroup{}, nil
	}

	// ステップ3: 不動在庫品目の期間前取引履歴を取得
	historyTxsByProductCode := make(map[string][]*model.TransactionRecord)
	if len(deadStockProductCodes) > 0 && filters.StartDate != "" {
		startDate, err := time.Parse("20060102", filters.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid start date format: %w", err)
		}

		historyEndDate := startDate.AddDate(0, 0, -1)
		tempDate := startDate.AddDate(0, -2, 0)
		historyStartDate := time.Date(tempDate.Year(), tempDate.Month(), 1, 0, 0, 0, 0, time.UTC)

		historyQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE flag != 0 AND transaction_date BETWEEN ? AND ? AND jan_code IN (?` + strings.Repeat(",?", len(deadStockProductCodes)-1) + `)`
		args := []interface{}{historyStartDate.Format("20060102"), historyEndDate.Format("20060102")}
		for _, pc := range deadStockProductCodes {
			args = append(args, pc)
		}

		historyRows, err := tx.Query(historyQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to get recent transaction history: %w", err)
		}
		defer historyRows.Close()

		for historyRows.Next() {
			rec, err := ScanTransactionRecord(historyRows)
			if err != nil {
				return nil, err
			}
			historyTxsByProductCode[rec.JanCode] = append(historyTxsByProductCode[rec.JanCode], rec)
		}
	}

	// ステップ4: 最終整形
	groups := make(map[string][]*model.ProductMaster)
	for _, m := range deadStockMasters {
		if m.YjCode != "" {
			groups[m.YjCode] = append(groups[m.YjCode], m)
		}
	}

	var result []model.DeadStockGroup
	for yjCode, masterList := range groups {
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
			var packageGroupTotalStock float64
			var productsInPackageGroup []model.DeadStockProduct
			var recentTxsForPackage []model.TransactionRecord

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

				if txs, ok := historyTxsByProductCode[master.ProductCode]; ok {
					for _, txPtr := range txs {
						recentTxsForPackage = append(recentTxsForPackage, *txPtr)
					}
				}
			}

			if len(productsInPackageGroup) > 0 {
				sort.Slice(recentTxsForPackage, func(i, j int) bool {
					if recentTxsForPackage[i].TransactionDate != recentTxsForPackage[j].TransactionDate {
						return recentTxsForPackage[i].TransactionDate < recentTxsForPackage[j].TransactionDate
					}
					return recentTxsForPackage[i].ID < recentTxsForPackage[j].ID
				})

				finalPackageGroups = append(finalPackageGroups, model.DeadStockPackageGroup{
					PackageKey:         key,
					TotalStock:         packageGroupTotalStock,
					Products:           productsInPackageGroup,
					RecentTransactions: recentTxsForPackage,
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

	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3,
			"4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6,
		}

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

	return result, nil
}

// getSavedDeadStock は特定の製品コードに紐づくロット・期限情報をDBから取得するヘルパー関数です。
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

/**
 * @brief 手動で入力されたロット・期限情報をデッドストックテーブルに保存（UPSERT）します。
 * @param tx SQLトランザクションオブジェクト
 * @param records 保存するデッドストック情報のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * この関数はまず、渡されたレコードに対応する製品の既存データを全て削除します。
 * その後、新しいレコードを挿入します。これにより、常に最新の状態でデータが保たれます。
 */
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

/**
 * @brief 複数の製品コードに紐づく有効なデッドストックレコードを全て取得します。
 * @param conn データベース接続
 * @param productCodes 取得対象の製品コードのスライス
 * @return []model.DeadStockRecord デッドストック情報のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 「棚卸調整」画面などで、特定の製品群のロット・期限情報を参照するために使用されます。
 */
func GetDeadStockByProductCodes(conn *sql.DB, productCodes []string) ([]model.DeadStockRecord, error) {
	if len(productCodes) == 0 {
		return []model.DeadStockRecord{}, nil
	}

	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf(`
		SELECT id, product_code, stock_quantity_jan, expiry_date, lot_number 
		FROM dead_stock_list 
		WHERE product_code IN (%s)
		ORDER BY product_code, expiry_date, lot_number`, placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead stock records by product codes: %w", err)
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
