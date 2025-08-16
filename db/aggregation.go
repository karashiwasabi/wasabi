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
	// ▼▼▼ [修正点] 発注残マップ取得関数を変更 ▼▼▼
	backordersMap, err := GetAllBackordersMap(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get backorders for aggregation: %w", err)
	}
	// ▲▲▲ 修正ここまで ▲▲▲

	precompTotals, err := GetPreCompoundingTotals(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-compounding totals for aggregation: %w", err)
	}

	// === ステップ1: フィルターに合致する製品マスターを取得 ===
	masterQuery := `SELECT ` + selectColumns + ` FROM product_master p WHERE 1=1 `
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND p.kana_name LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%")
	}
	if filters.DosageForm != "" {
		masterQuery += " AND p.usage_classification LIKE ? "
		masterArgs = append(masterArgs, "%"+filters.DosageForm+"%")
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

	// === ステップ2: 関連する全期間のトランザクションを取得 ===
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

	// === ステップ3: YJコードごとに集計処理 ===
	var result []model.StockLedgerYJGroup
	for yjCode, mastersInYjGroup := range mastersByYjCode {
		if len(mastersInYjGroup) == 0 {
			continue
		}

		representativeProductName := mastersInYjGroup[0].ProductName
		yjGroup := model.StockLedgerYJGroup{
			YjCode:      yjCode,
			ProductName: representativeProductName,
			YjUnitName:  units.ResolveName(mastersInYjGroup[0].YjUnitName),
		}

		// ▼▼▼ [修正点] mastersByPackageKeyのキー生成方法を変更 ▼▼▼
		mastersByPackageKey := make(map[string][]*model.ProductMaster)
		for _, m := range mastersInYjGroup {
			// YJコードを含めたユニークなキーを生成
			key := fmt.Sprintf("%s|%s|%g|%s", m.YjCode, m.PackageForm, m.JanPackInnerQty, m.YjUnitName)
			mastersByPackageKey[key] = append(mastersByPackageKey[key], m)
		}
		// ▲▲▲ 修正ここまで ▲▲▲

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

			// --- 新しい計算ロジック ---
			// 1. 期首在庫を計算
			var startingBalance float64
			latestInventoryDateBeforePeriod := ""
			var lastInventoryQty float64
			for _, t := range allTxsForPackage {
				if t.TransactionDate < filters.StartDate && t.Flag == 0 {
					if t.TransactionDate > latestInventoryDateBeforePeriod {
						latestInventoryDateBeforePeriod = t.TransactionDate
						lastInventoryQty = t.YjQuantity
					}
				}
			}

			if latestInventoryDateBeforePeriod != "" {
				startingBalance = lastInventoryQty
				for _, t := range allTxsForPackage {
					if t.TransactionDate > latestInventoryDateBeforePeriod && t.TransactionDate < filters.StartDate {
						startingBalance += t.SignedYjQty()
					}
				}
			} else {
				for _, t := range allTxsForPackage {
					if t.TransactionDate < filters.StartDate {
						startingBalance += t.SignedYjQty()
					}
				}
			}

			// 2. 期間内のトランザクションを処理して残高を計算
			var transactionsInPeriod []model.LedgerTransaction
			var netChange, maxUsage float64
			runningBalance := startingBalance

			for _, t := range allTxsForPackage {
				if t.TransactionDate >= filters.StartDate && t.TransactionDate <= filters.EndDate {
					if t.Flag == 0 { // 棚卸の場合、残高を強制補正
						runningBalance = t.YjQuantity
					} else { // それ以外の取引
						runningBalance += t.SignedYjQty()
					}
					transactionsInPeriod = append(transactionsInPeriod, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})

					netChange += t.SignedYjQty()
					if t.Flag == 3 && t.YjQuantity > maxUsage {
						maxUsage = t.YjQuantity
					}
				}
			}

			// ▼▼▼ [修正点] 物理在庫に発注残を加えた有効在庫を計算 ▼▼▼
			backorderQty := backordersMap[key]
			effectiveEndingBalance := runningBalance + backorderQty
			// ▲▲▲ 修正ここまで ▲▲▲

			pkg := model.StockLedgerPackageGroup{
				PackageKey:      key,
				StartingBalance: startingBalance,
				EndingBalance:   runningBalance,
				// ▼▼▼ [修正点] 以下の1行を追加 ▼▼▼
				EffectiveEndingBalance: effectiveEndingBalance,
				// ▲▲▲ 修正ここまで ▲▲▲
				Transactions: transactionsInPeriod,
				NetChange:    netChange,
				MaxUsage:     maxUsage,
			}

			// 発注点計算
			var precompTotalForPackage float64
			for _, master := range mastersInPackageGroup {
				if total, ok := precompTotals[master.ProductCode]; ok {
					precompTotalForPackage += total
				}
			}
			pkg.BaseReorderPoint = maxUsage * filters.Coefficient
			pkg.PrecompoundedTotal = precompTotalForPackage
			pkg.ReorderPoint = pkg.BaseReorderPoint + pkg.PrecompoundedTotal
			// ▼▼▼ [修正点] 発注要否の判定を effectiveEndingBalance で行う ▼▼▼
			pkg.IsReorderNeeded = effectiveEndingBalance < pkg.ReorderPoint && pkg.MaxUsage > 0
			// ▲▲▲ 修正ここまで ▲▲▲
			if len(mastersInPackageGroup) > 0 {
				pkg.Masters = mastersInPackageGroup // スライス全体を新しいフィールドに設定する
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

	return result, nil
}
