package orders

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// OrderCandidatesResponse は発注準備画面へのレスポンスの構造体です。
type OrderCandidatesResponse struct {
	Candidates  []OrderCandidateYJGroup `json:"candidates"`
	Wholesalers []model.Wholesaler      `json:"wholesalers"`
}

// OrderCandidateYJGroup は表示用に変換されたYJグループです。
type OrderCandidateYJGroup struct {
	model.StockLedgerYJGroup
	PackageLedgers []OrderCandidatePackageGroup `json:"packageLedgers"`
}

// OrderCandidatePackageGroup は表示用に変換された包装グループです。
type OrderCandidatePackageGroup struct {
	model.StockLedgerPackageGroup
	Masters []model.ProductMasterView `json:"masters"`
}

// GenerateOrderCandidatesHandler は発注候補を生成して返します。
func GenerateOrderCandidatesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startDate := r.URL.Query().Get("startDate")
		endDate := r.URL.Query().Get("endDate")
		kanaName := r.URL.Query().Get("kanaName")
		dosageForm := r.URL.Query().Get("dosageForm")
		coefficientStr := r.URL.Query().Get("coefficient")
		coefficient, err := strconv.ParseFloat(coefficientStr, 64)
		if err != nil {
			coefficient = 1.3
		}

		// ▼▼▼ 修正箇所 ▼▼▼
		// パラメータをフィルタ用の構造体にまとめる
		filters := model.AggregationFilters{
			StartDate:   startDate,
			EndDate:     endDate,
			KanaName:    kanaName,
			DosageForm:  dosageForm,
			Coefficient: coefficient,
		}

		// 構造体を使って関数を呼び出す
		yjGroups, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		var candidates []OrderCandidateYJGroup
		for _, group := range yjGroups {
			if group.IsReorderNeeded {
				newYjGroup := OrderCandidateYJGroup{
					StockLedgerYJGroup: group,
					PackageLedgers:     []OrderCandidatePackageGroup{},
				}

				for _, pkg := range group.PackageLedgers {
					newPkgGroup := OrderCandidatePackageGroup{
						StockLedgerPackageGroup: pkg,
						Masters:                 []model.ProductMasterView{},
					}
					for _, master := range pkg.Masters {
						// ▼▼▼ [修正点] 組み立て包装の生成に必要なデータを全て渡すように修正 ▼▼▼
						tempJcshms := model.JCShms{
							JC037: master.PackageForm,
							JC039: master.YjUnitName,
							JC044: master.YjPackUnitQty,
							JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
							JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
							JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
						}
						// ▲▲▲ 修正ここまで ▲▲▲
						formattedSpec := units.FormatPackageSpec(&tempJcshms)

						newPkgGroup.Masters = append(newPkgGroup.Masters, model.ProductMasterView{
							ProductMaster:        *master,
							FormattedPackageSpec: formattedSpec,
						})
					}
					newYjGroup.PackageLedgers = append(newYjGroup.PackageLedgers, newPkgGroup)
				}
				candidates = append(candidates, newYjGroup)
			}
		}

		// データベースから全卸業者リストを取得
		wholesalers, err := db.GetAllWholesalers(conn)
		if err != nil {
			http.Error(w, "Failed to get wholesalers", http.StatusInternalServerError)
			return
		}

		// 発注候補と卸業者リストをまとめてレスポンスを作成
		response := OrderCandidatesResponse{
			Candidates:  candidates,
			Wholesalers: wholesalers,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// ▼▼▼ [修正点] PlaceOrderHandlerを新しいテーブル構造に合わせて修正 ▼▼▼
// PlaceOrderHandler は発注内容を受け取り、発注残テーブルに登録します。
func PlaceOrderHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []model.Backorder
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		today := time.Now().Format("20060102")
		for i := range payload {
			if payload[i].OrderDate == "" {
				payload[i].OrderDate = today
			}
		}

		if err := db.UpsertBackordersInTx(tx, payload); err != nil {
			http.Error(w, "Failed to save backorders", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "発注内容を発注残として登録しました。"})
	}
}
