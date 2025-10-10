// C:\Users\wasab\OneDrive\デスクトップ\WASABI\returns\handler.go

package returns

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort" // 👈 インポートを追加
	"strconv"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
)

// GenerateReturnCandidatesHandler は返品可能リストを生成します
func GenerateReturnCandidatesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		now := time.Now()
		endDate := "99991231"
		startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)
		startDateStr := startDate.Format("20060102")

		filters := model.AggregationFilters{
			StartDate:   startDateStr,
			EndDate:     endDate,
			KanaName:    q.Get("kanaName"),
			DosageForm:  q.Get("dosageForm"),
			Coefficient: coefficient,
		}

		// ステップ1: 過去のデータから使用量を分析し、発注点を計算する
		yjGroups, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get stock ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// ステップ2: 「今現在」のリアルタイム在庫を取得する
		currentStockMap, err := db.GetAllCurrentStockMap(conn)
		if err != nil {
			http.Error(w, "Failed to get current stock map: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var returnCandidates []model.StockLedgerYJGroup
		for _, group := range yjGroups {
			var returnablePackages []model.StockLedgerPackageGroup
			isGroupAdded := false

			for _, pkg := range group.PackageLedgers {
				// ステップ3: 包装ごとに「今現在」の在庫を計算する
				var currentStockForPackage float64
				var productCodesInPackage []string
				for _, master := range pkg.Masters {
					currentStockForPackage += currentStockMap[master.ProductCode]
					productCodesInPackage = append(productCodesInPackage, master.ProductCode)
				}

				trueEffectiveBalance := currentStockForPackage

				// ステップ4: 「発注点」と「今現在の有効在庫」を比較する
				if len(pkg.Masters) > 0 {
					yjPackUnitQty := pkg.Masters[0].YjPackUnitQty
					if pkg.ReorderPoint > 0 && trueEffectiveBalance > (pkg.ReorderPoint+yjPackUnitQty) {

						pkg.EffectiveEndingBalance = trueEffectiveBalance

						if len(productCodesInPackage) > 0 {
							deliveryHistory, err := getDeliveryHistory(conn, productCodesInPackage, startDateStr, endDate)
							if err != nil {
								fmt.Printf("WARN: Failed to get delivery history for package %s: %v\n", pkg.PackageKey, err)
							}
							pkg.DeliveryHistory = deliveryHistory
						}

						returnablePackages = append(returnablePackages, pkg)
						isGroupAdded = true
					}
				}
			}

			if isGroupAdded {
				newGroup := group
				newGroup.PackageLedgers = returnablePackages
				returnCandidates = append(returnCandidates, newGroup)
			}
		}

		// ▼▼▼【ここから追加】▼▼▼
		// 返品候補リストを剤型優先、次にカナ名順でソートする
		sort.Slice(returnCandidates, func(i, j int) bool {
			prio := map[string]int{
				"1": 1, "内": 1, "2": 2, "外": 2, "3": 3, "注": 3,
				"4": 4, "歯": 4, "5": 5, "機": 5, "6": 6, "他": 6,
			}

			// 各YJグループから代表のマスターを取得する
			var masterI, masterJ *model.ProductMaster
			if len(returnCandidates[i].PackageLedgers) > 0 && len(returnCandidates[i].PackageLedgers[0].Masters) > 0 {
				masterI = returnCandidates[i].PackageLedgers[0].Masters[0]
			}
			if len(returnCandidates[j].PackageLedgers) > 0 && len(returnCandidates[j].PackageLedgers[0].Masters) > 0 {
				masterJ = returnCandidates[j].PackageLedgers[0].Masters[0]
			}

			// マスターが取得できなかった場合はYJコードで比較する
			if masterI == nil || masterJ == nil {
				return returnCandidates[i].YjCode < returnCandidates[j].YjCode
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
		// ▲▲▲【追加ここまで】▲▲▲

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(returnCandidates)
	}
}

func getDeliveryHistory(conn *sql.DB, productCodes []string, startDate, endDate string) ([]model.TransactionRecord, error) {
	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf(`SELECT `+db.TransactionColumns+` FROM transaction_records 
		WHERE flag = 1 AND jan_code IN (%s) AND transaction_date BETWEEN ? AND ? 
		ORDER BY transaction_date DESC, id DESC`, placeholders)

	args := make([]interface{}, 0, len(productCodes)+2)
	for _, code := range productCodes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.TransactionRecord
	for rows.Next() {
		r, err := db.ScanTransactionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}
	return records, nil
}
