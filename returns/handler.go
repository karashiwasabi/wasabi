// C:\Users\wasab\OneDrive\デスクトップ\WASABI\returns\handler.go

package returns

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

// GenerateReturnCandidatesHandler は返品可能リストを生成します
func GenerateReturnCandidatesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// ▼▼▼ [修正点] 係数をフロントエンドから受け取るようにする ▼▼▼
		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			// 係数が指定されなかった、または無効な場合はデフォルト値を使用
			coefficient = 1.5
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		filters := model.AggregationFilters{
			StartDate:   q.Get("startDate"),
			EndDate:     q.Get("endDate"),
			KanaName:    q.Get("kanaName"),
			DosageForm:  q.Get("dosageForm"),
			Coefficient: coefficient,
		}

		yjGroups, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get stock ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var returnCandidates []model.StockLedgerYJGroup

		for _, group := range yjGroups {
			var returnablePackages []model.StockLedgerPackageGroup
			isGroupAdded := false

			for _, pkg := range group.PackageLedgers {
				// ▼▼▼ [修正点] 係数を考慮した発注点(ReorderPoint)を使用するロジックに修正 ▼▼▼
				// 条件: 在庫 > 発注点 + 1包装あたりの数量
				if len(pkg.Masters) > 0 {
					yjPackUnitQty := pkg.Masters[0].YjPackUnitQty
					// 発注点が0より大きく、かつ在庫が「発注点＋1包装」を上回る場合
					if pkg.ReorderPoint > 0 && pkg.EffectiveEndingBalance > (pkg.ReorderPoint+yjPackUnitQty) {
						returnablePackages = append(returnablePackages, pkg)
						isGroupAdded = true
					}
				}
				// ▲▲▲ 修正ここまで ▲▲▲
			}

			if isGroupAdded {
				newGroup := group
				newGroup.PackageLedgers = returnablePackages
				returnCandidates = append(returnCandidates, newGroup)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(returnCandidates)
	}
}
