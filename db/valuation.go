package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"wasabi/model"
)

// ValuationGroup は剤型ごとの集計結果を保持します
type ValuationGroup struct {
	UsageClassification string             `json:"usageClassification"`
	YjGroups            []ValuationYjGroup `json:"yjGroups"`
	TotalNhiValue       float64            `json:"totalNhiValue"`
	TotalPurchaseValue  float64            `json:"totalPurchaseValue"`
}

// ValuationYjGroup はYJコードごとの集計結果を保持します
type ValuationYjGroup struct {
	YjCode                string  `json:"yjCode"`
	ProductName           string  `json:"productName"`
	TotalYjStock          float64 `json:"totalYjStock"`
	YjUnitName            string  `json:"yjUnitName"`
	NhiValue              float64 `json:"nhiValue"`
	PurchaseValue         float64 `json:"purchaseValue"`
	ContainsOnlyNonJcshms bool    `json:"containsOnlyNonJcshms"`
}

// GetInventoryValuation は指定日の在庫評価レポートを生成します
func GetInventoryValuation(conn *sql.DB, date string) ([]ValuationGroup, error) {
	// 1. 全ての製品マスターを取得
	masters, err := GetAllProductMasters(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get all product masters: %w", err)
	}

	// 2. 各製品の指定日時点の理論在庫を計算
	stockMap := make(map[string]float64)
	for _, master := range masters {
		stock, err := calculateStockForDate(conn, master.ProductCode, date)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate stock for %s: %w", master.ProductCode, err)
		}
		stockMap[master.ProductCode] = stock
	}

	// 3. YJコード単位で集計
	yjGroupMap := make(map[string]ValuationYjGroup)
	mastersInYjGroup := make(map[string][]*model.ProductMaster)

	for _, master := range masters {
		stock := stockMap[master.ProductCode]
		if stock == 0 {
			continue // 在庫が0の品目は集計から除外
		}

		group, ok := yjGroupMap[master.YjCode]
		if !ok {
			group = ValuationYjGroup{
				YjCode:                master.YjCode,
				ProductName:           master.ProductName,
				YjUnitName:            master.YjUnitName,
				ContainsOnlyNonJcshms: true, // 初期値をtrueに設定
			}
		}

		group.TotalYjStock += stock
		group.NhiValue += stock * master.NhiPrice
		group.PurchaseValue += stock * master.PurchasePrice
		if master.Origin == "JCSHMS" {
			group.ContainsOnlyNonJcshms = false // JCSHMS由来の品目が一つでもあればfalseに
		}

		yjGroupMap[master.YjCode] = group
		mastersInYjGroup[master.YjCode] = append(mastersInYjGroup[master.YjCode], master)
	}

	// 4. 剤型(usage_classification)ごとに最終的なグループ分け
	resultGroups := make(map[string]ValuationGroup)
	for yjCode, yjGroup := range yjGroupMap {
		if len(mastersInYjGroup[yjCode]) == 0 {
			continue
		}
		uc := mastersInYjGroup[yjCode][0].UsageClassification

		group, ok := resultGroups[uc]
		if !ok {
			group = ValuationGroup{UsageClassification: uc}
		}

		group.YjGroups = append(group.YjGroups, yjGroup)
		group.TotalNhiValue += yjGroup.NhiValue
		group.TotalPurchaseValue += yjGroup.PurchaseValue
		resultGroups[uc] = group
	}

	// 5. 指示通りの順序でソート
	order := map[string]int{"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3, "4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6}
	var finalResult []ValuationGroup
	for _, group := range resultGroups {
		finalResult = append(finalResult, group)
	}
	sort.Slice(finalResult, func(i, j int) bool {
		prioI, okI := order[strings.TrimSpace(finalResult[i].UsageClassification)]
		if !okI {
			prioI = 7
		}
		prioJ, okJ := order[strings.TrimSpace(finalResult[j].UsageClassification)]
		if !okJ {
			prioJ = 7
		}
		return prioI < prioJ
	})

	return finalResult, nil
}

// calculateStockForDate は特定の日付の理論在庫を計算します
func calculateStockForDate(conn *sql.DB, productCode string, date string) (float64, error) {
	var lastInventory model.TransactionRecord
	var hasInventory bool

	row := conn.QueryRow(`
		SELECT `+TransactionColumns+` FROM transaction_records
		WHERE jan_code = ? AND flag = 0 AND transaction_date <= ?
		ORDER BY transaction_date DESC, id DESC LIMIT 1`, productCode, date)

	rec, err := ScanTransactionRecord(row)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if err == nil {
		hasInventory = true
		lastInventory = *rec
	}

	var query string
	var args []interface{}
	if hasInventory {
		query = `SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
				 FROM transaction_records
				 WHERE jan_code = ? AND (transaction_date > ? OR (transaction_date = ? AND id > ?)) AND transaction_date <= ?`
		args = []interface{}{productCode, lastInventory.TransactionDate, lastInventory.TransactionDate, lastInventory.ID, date}
	} else {
		query = `SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
				 FROM transaction_records WHERE jan_code = ? AND transaction_date <= ?`
		args = []interface{}{productCode, date}
	}

	var nullNetChange sql.NullFloat64
	err = conn.QueryRow(query, args...).Scan(&nullNetChange)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	netChange := nullNetChange.Float64

	if hasInventory {
		return lastInventory.YjQuantity + netChange, nil
	}
	return netChange, nil
}
