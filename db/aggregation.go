// C:\Users\wasab\OneDrive\デスクトップ\WASABI\WASABI\db\aggregation.go

package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

/**
 * @brief 在庫元帳レポートを生成します。
 * @param conn データベース接続
 * @param filters 絞り込み条件
 * @return []model.StockLedgerYJGroup 集計結果のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * この関数はアプリケーションの在庫計算における中心的なロジックです。
 * 以下のステップで在庫元帳を生成します。
 * 1. フィルタ条件に合致する製品マスターを取得します。
 * 2. 取得した製品マスターに関連する全期間の取引履歴を一括で取得します。
 * 3. 製品をYJコードごと、さらに包装ごとにグループ化します。
 * 4. 各包装グループについて以下の計算を行います。
 * a. 期間開始前の取引履歴を遡り、最後の棚卸を基点とした「期間前在庫（繰越在庫）」を算出します。
 * b. 期間内の取引を時系列で処理し、「期間内変動」「最大使用量」「期間終了在庫」を算出します。
 * c. 発注残と予製引当数を考慮し、「有効在庫」と「発注点」を計算します。
 * 5. 全ての包装グループのデータをYJコードごとに集計し、最終的なレポートを生成します。
 * 6. 結果を剤型とカナ名でソートして返却します。
 */
func GetStockLedger(conn *sql.DB, filters model.AggregationFilters) ([]model.StockLedgerYJGroup, error) {
	backordersMap, err := GetAllBackordersMap(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get backorders for aggregation: %w", err)
	}

	precompTotals, err := GetPreCompoundingTotals(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-compounding totals for aggregation: %w", err)
	}

	// ステップ1: フィルターに合致する製品マスターを取得
	masterQuery := `SELECT ` + SelectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}

	if filters.YjCode != "" {
		masterQuery += " AND p.yj_code = ? "
		masterArgs = append(masterArgs, filters.YjCode)
	}

	if filters.KanaName != "" {
		masterQuery += " AND (p.kana_name LIKE ? OR p.product_name LIKE ?) "
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%", "%"+filters.KanaName+"%")
	}

	if filters.DosageForm != "" && filters.DosageForm != "all" {
		masterQuery += " AND p.usage_classification = ? "
		masterArgs = append(masterArgs, filters.DosageForm)
	}

	if len(filters.DrugTypes) > 0 && filters.DrugTypes[0] != "" {
		var conditions []string
		flagMap := map[string]string{
			"poison":        "p.flag_poison = 1",
			"deleterious":   "p.flag_deleterious = 1",
			"narcotic":      "p.flag_narcotic = 1",
			"psychotropic1": "p.flag_psychotropic = 1",
			"psychotropic2": "p.flag_psychotropic = 2",
			"psychotropic3": "p.flag_psychotropic = 3",
			"stimulant":     "p.flag_stimulant = 1",
			"stimulant_raw": "p.flag_stimulant_raw = 1",
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
		m, err := ScanProductMaster(masterRows)
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

	// ステップ2: 関連する全期間のトランザクションを取得
	var txArgs []interface{}
	for _, pc := range productCodes {
		txArgs = append(txArgs, pc)
	}

	transactionsByProductCode := make(map[string][]*model.TransactionRecord)
	if len(productCodes) > 0 {
		txQuery := `SELECT ` + TransactionColumns + ` FROM transaction_records WHERE jan_code IN (?` + strings.Repeat(",?", len(productCodes)-1) + `) ORDER BY transaction_date, id`
		txRows, err := conn.Query(txQuery, txArgs...)
		if err != nil {
			return nil, err
		}
		defer txRows.Close()
		for txRows.Next() {
			t, err := ScanTransactionRecord(txRows)
			if err != nil {
				return nil, err
			}
			transactionsByProductCode[t.JanCode] = append(transactionsByProductCode[t.JanCode], t)
		}
	}

	// ステップ3: YJコードごとに集計処理
	var result []model.StockLedgerYJGroup
	for yjCode, mastersInYjGroup := range mastersByYjCode {
		if len(mastersInYjGroup) == 0 {
			continue
		}

		// YJグループの代表製品名をJCSHMS由来のものから優先的に選択する
		var representativeProductName string
		var representativeYjUnitName string
		if len(mastersInYjGroup) > 0 {
			representativeProductName = mastersInYjGroup[0].ProductName
			representativeYjUnitName = mastersInYjGroup[0].YjUnitName
			for _, m := range mastersInYjGroup {
				if m.Origin == "JCSHMS" {
					representativeProductName = m.ProductName
					representativeYjUnitName = m.YjUnitName
					break
				}
			}
		}

		yjGroup := model.StockLedgerYJGroup{
			YjCode:      yjCode,
			ProductName: representativeProductName,
			YjUnitName:  units.ResolveName(representativeYjUnitName),
		}

		mastersByPackageKey := make(map[string][]*model.ProductMaster)
		for _, m := range mastersInYjGroup {
			key := fmt.Sprintf("%s|%s|%g|%s", m.YjCode, m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			mastersByPackageKey[key] = append(mastersByPackageKey[key], m)
		}

		var allPackageLedgers []model.StockLedgerPackageGroup
		for key, mastersInPackageGroup := range mastersByPackageKey {
			var allTxsForPackage []*model.TransactionRecord
			for _, m := range mastersInPackageGroup {
				allTxsForPackage = append(allTxsForPackage, transactionsByProductCode[m.ProductCode]...)
			}
			sort.Slice(allTxsForPackage, func(i, j int) bool {
				if allTxsForPackage[i].TransactionDate != allTxsForPackage[j].TransactionDate {
					return allTxsForPackage[i].TransactionDate < allTxsForPackage[j].TransactionDate
				}
				return allTxsForPackage[i].ID < allTxsForPackage[j].ID
			})

			// 期間前在庫（繰越在庫）を計算
			var startingBalance float64
			latestInventoryDateBeforePeriod := ""
			txsBeforePeriod := []*model.TransactionRecord{}
			inventorySumsByDate := make(map[string]float64)

			for _, t := range allTxsForPackage {
				if t.TransactionDate < filters.StartDate {
					txsBeforePeriod = append(txsBeforePeriod, t)
					if t.Flag == 0 { // 棚卸レコード
						inventorySumsByDate[t.TransactionDate] += t.YjQuantity
						if t.TransactionDate > latestInventoryDateBeforePeriod {
							latestInventoryDateBeforePeriod = t.TransactionDate
						}
					}
				}
			}

			if latestInventoryDateBeforePeriod != "" {
				startingBalance = inventorySumsByDate[latestInventoryDateBeforePeriod]
				for _, t := range txsBeforePeriod {
					if t.TransactionDate > latestInventoryDateBeforePeriod {
						startingBalance += t.SignedYjQty()
					}
				}
			} else {
				for _, t := range txsBeforePeriod {
					startingBalance += t.SignedYjQty()
				}
			}

			// ▼▼▼【ここから全面的に修正】▼▼▼
			// 期間内変動と最終在庫を計算するロジックを修正
			var transactionsInPeriod []model.LedgerTransaction
			var netChange, maxUsage float64
			runningBalance := startingBalance

			// 期間内の棚卸合計を日付ごとに事前計算
			periodInventorySums := make(map[string]float64)
			for _, t := range allTxsForPackage {
				if t.TransactionDate >= filters.StartDate && t.TransactionDate <= filters.EndDate && t.Flag == 0 {
					periodInventorySums[t.TransactionDate] += t.YjQuantity
				}
			}

			lastProcessedDate := ""
			for _, t := range allTxsForPackage {
				if t.TransactionDate >= filters.StartDate && t.TransactionDate <= filters.EndDate {
					// 日付が変わったタイミングで、前の日に棚卸があったなら、その日の最終在庫として残高をリセット
					if t.TransactionDate != lastProcessedDate && lastProcessedDate != "" {
						if inventorySum, ok := periodInventorySums[lastProcessedDate]; ok {
							runningBalance = inventorySum
						}
					}

					// 棚卸(flag=0)の場合はその日の棚卸合計値で残高を上書きし、それ以外は変動量を加算する
					if t.Flag == 0 {
						if inventorySum, ok := periodInventorySums[t.TransactionDate]; ok {
							runningBalance = inventorySum
						}
					} else {
						runningBalance += t.SignedYjQty()
					}

					transactionsInPeriod = append(transactionsInPeriod, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})

					// 純変動と最大使用量は全ての取引を対象に計算
					netChange += t.SignedYjQty()
					if t.Flag == 3 && t.YjQuantity > maxUsage { // 処方レコード
						maxUsage = t.YjQuantity
					}
					lastProcessedDate = t.TransactionDate
				}
			}
			// ▲▲▲【修正ここまで】▲▲▲

			backorderQty := backordersMap[key]
			effectiveEndingBalance := runningBalance + backorderQty

			pkg := model.StockLedgerPackageGroup{
				PackageKey:             key,
				StartingBalance:        startingBalance,
				EndingBalance:          runningBalance,
				EffectiveEndingBalance: effectiveEndingBalance,
				Transactions:           transactionsInPeriod,
				NetChange:              netChange,
				MaxUsage:               maxUsage,
			}

			var precompTotalForPackage float64
			for _, master := range mastersInPackageGroup {
				if total, ok := precompTotals[master.ProductCode]; ok {
					precompTotalForPackage += total
				}
			}
			pkg.BaseReorderPoint = maxUsage * filters.Coefficient
			pkg.PrecompoundedTotal = precompTotalForPackage
			pkg.ReorderPoint = pkg.BaseReorderPoint + pkg.PrecompoundedTotal
			pkg.IsReorderNeeded = effectiveEndingBalance < pkg.ReorderPoint && pkg.MaxUsage > 0
			if len(mastersInPackageGroup) > 0 {
				pkg.Masters = mastersInPackageGroup
			}
			allPackageLedgers = append(allPackageLedgers, pkg)
		}

		if len(allPackageLedgers) > 0 {
			var yjTotalEnding, yjTotalNetChange, yjTotalReorderPoint, yjTotalBaseReorderPoint, yjTotalPrecompounded float64
			var yjTotalStarting float64
			isYjReorderNeeded := false
			for _, pkg := range allPackageLedgers {
				if start, ok := pkg.StartingBalance.(float64); ok {
					yjTotalStarting += start
				}
				if end, ok := pkg.EndingBalance.(float64); ok {
					yjTotalEnding += end
				}
				yjTotalNetChange += pkg.NetChange
				yjTotalReorderPoint += pkg.ReorderPoint
				yjTotalBaseReorderPoint += pkg.BaseReorderPoint
				yjTotalPrecompounded += pkg.PrecompoundedTotal
				if pkg.IsReorderNeeded {
					isYjReorderNeeded = true
				}
			}
			yjGroup.StartingBalance = yjTotalStarting
			yjGroup.EndingBalance = yjTotalEnding
			yjGroup.NetChange = yjTotalNetChange
			yjGroup.TotalReorderPoint = yjTotalReorderPoint
			yjGroup.TotalBaseReorderPoint = yjTotalBaseReorderPoint
			yjGroup.TotalPrecompounded = yjTotalPrecompounded
			yjGroup.IsReorderNeeded = isYjReorderNeeded
			yjGroup.PackageLedgers = allPackageLedgers
			result = append(result, yjGroup)
		}
	}

	// 剤型とカナ名でソート
	sort.Slice(result, func(i, j int) bool {
		prio := map[string]int{
			"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "注": 3,
			"4": 4, "歯": 4, "5": 5, "機": 5, "6": 6, "他": 6,
		}
		masterI := mastersByYjCode[result[i].YjCode][0]
		masterJ := mastersByYjCode[result[j].YjCode][0]
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

	// 「期間内に動きがあった品目のみ」フィルタを適用
	if filters.MovementOnly {
		var filteredResult []model.StockLedgerYJGroup
		for _, yjGroup := range result {
			hasMovement := false
			for _, pkg := range yjGroup.PackageLedgers {
				for _, tx := range pkg.Transactions {
					if tx.Flag != 0 { // flagが0（棚卸）以外のトランザクション
						hasMovement = true
						break
					}
				}
				if hasMovement {
					break
				}
			}
			if hasMovement {
				filteredResult = append(filteredResult, yjGroup)
			}
		}
		return filteredResult, nil
	}
	return result, nil
}
