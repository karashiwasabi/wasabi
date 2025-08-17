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
	YjCode        string  `json:"yjCode"`
	ProductName   string  `json:"productName"`
	KanaName      string  `json:"kanaName"`
	TotalYjStock  float64 `json:"totalYjStock"`
	YjUnitName    string  `json:"yjUnitName"`
	NhiValue      float64 `json:"nhiValue"`
	PurchaseValue float64 `json:"purchaseValue"`
	ShowAlert     bool    `json:"showAlert"`
}

// GetInventoryValuation は指定日の在庫評価レポートを生成します
func GetInventoryValuation(conn *sql.DB, filters model.ValuationFilters) ([]ValuationGroup, error) {
	masterQuery := `SELECT ` + selectColumns + ` FROM product_master WHERE 1=1`
	var masterArgs []interface{}
	if filters.KanaName != "" {
		masterQuery += " AND (kana_name LIKE ? OR product_name LIKE ?)"
		masterArgs = append(masterArgs, "%"+filters.KanaName+"%", "%"+filters.KanaName+"%")
	}
	if filters.UsageClassification != "" && filters.UsageClassification != "all" {
		masterQuery += " AND usage_classification = ?"
		masterArgs = append(masterArgs, filters.UsageClassification)
	}

	allMasters, err := getAllProductMastersFiltered(conn, masterQuery, masterArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtered product masters: %w", err)
	}

	mastersInYjGroup := make(map[string][]*model.ProductMaster)
	for _, master := range allMasters {
		if master.YjCode != "" {
			mastersInYjGroup[master.YjCode] = append(mastersInYjGroup[master.YjCode], master)
		}
	}

	yjGroupMap := make(map[string]ValuationYjGroup)

	for yjCode, masters := range mastersInYjGroup {
		var representativeMaster *model.ProductMaster
		containsJcshms := false

		for _, m := range masters {
			if m.Origin == "JCSHMS" {
				representativeMaster = m
				containsJcshms = true
				break
			}
		}
		if representativeMaster == nil {
			if len(masters) > 0 {
				representativeMaster = masters[0]
			} else {
				continue
			}
		}

		var totalStock float64
		for _, m := range masters {
			stock, err := calculateStockForDate(conn, m.ProductCode, filters.Date)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate stock for %s: %w", m.ProductCode, err)
			}
			totalStock += stock
		}

		if totalStock == 0 {
			continue
		}

		// ▼▼▼ [修正点] 納入価金額の計算ロジックを全面的に変更 ▼▼▼
		totalNhiValue := totalStock * representativeMaster.NhiPrice

		var totalPurchaseValue float64
		packagePurchasePrice := representativeMaster.PurchasePrice // これは包装価格
		yjPackQty := representativeMaster.YjPackUnitQty            // これがYJ包装数量

		if yjPackQty > 0 {
			unitPurchasePrice := packagePurchasePrice / yjPackQty // YJ単位あたりの納入単価を計算
			totalPurchaseValue = totalStock * unitPurchasePrice   // 在庫数(YJ単位)に単価を掛ける
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		showAlert := false
		if !containsJcshms {
			uc := strings.TrimSpace(representativeMaster.UsageClassification)
			if uc != "5" && uc != "機" && uc != "6" && uc != "他" {
				showAlert = true
			}
		}

		yjGroupMap[yjCode] = ValuationYjGroup{
			YjCode:        yjCode,
			ProductName:   representativeMaster.ProductName,
			KanaName:      representativeMaster.KanaName,
			TotalYjStock:  totalStock,
			YjUnitName:    representativeMaster.YjUnitName,
			NhiValue:      totalNhiValue,
			PurchaseValue: totalPurchaseValue,
			ShowAlert:     showAlert,
		}
	}

	resultGroups := make(map[string]*ValuationGroup)
	for yjCode, yjGroupData := range yjGroupMap {
		// YJコードに対応するマスターが mastersInYjGroup に存在することを保証
		if masterList, ok := mastersInYjGroup[yjCode]; ok && len(masterList) > 0 {
			uc := masterList[0].UsageClassification
			group, ok := resultGroups[uc]
			if !ok {
				group = &ValuationGroup{UsageClassification: uc}
				resultGroups[uc] = group
			}
			group.YjGroups = append(group.YjGroups, yjGroupData)
			group.TotalNhiValue += yjGroupData.NhiValue
			group.TotalPurchaseValue += yjGroupData.PurchaseValue
		}
	}

	order := map[string]int{"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "歯": 3, "4": 4, "注": 4, "5": 5, "機": 5, "6": 6, "他": 6}
	var finalResult []ValuationGroup
	for _, group := range resultGroups {
		sort.Slice(group.YjGroups, func(i, j int) bool {
			return group.YjGroups[i].KanaName < group.YjGroups[j].KanaName
		})
		finalResult = append(finalResult, *group)
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

func getAllProductMastersFiltered(conn *sql.DB, query string, args ...interface{}) ([]*model.ProductMaster, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetAllProductMastersFiltered query failed: %w", err)
	}
	defer rows.Close()

	var masters []*model.ProductMaster
	for rows.Next() {
		m, err := scanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, m)
	}
	return masters, nil
}

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
	var queryArgs []interface{}
	if hasInventory {
		query = `SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
				 FROM transaction_records
				 WHERE jan_code = ? AND (transaction_date > ? OR (transaction_date = ? AND id > ?)) AND transaction_date <= ?`
		queryArgs = []interface{}{productCode, lastInventory.TransactionDate, lastInventory.TransactionDate, lastInventory.ID, date}
	} else {
		query = `SELECT SUM(CASE WHEN flag IN (1, 4, 11) THEN yj_quantity WHEN flag IN (2, 3, 5, 12) THEN -yj_quantity ELSE 0 END)
				 FROM transaction_records WHERE jan_code = ? AND transaction_date <= ?`
		queryArgs = []interface{}{productCode, date}
	}

	var nullNetChange sql.NullFloat64
	err = conn.QueryRow(query, queryArgs...).Scan(&nullNetChange)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	if hasInventory {
		return lastInventory.YjQuantity + nullNetChange.Float64, nil
	}
	return nullNetChange.Float64, nil
}
