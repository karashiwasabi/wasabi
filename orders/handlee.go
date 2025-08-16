package orders

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

// OrderCandidatesResponse は発注準備画面へのレスポンスの構造体です。
type OrderCandidatesResponse struct {
	Candidates  []model.StockLedgerYJGroup `json:"candidates"`
	Wholesalers []model.Wholesaler         `json:"wholesalers"`
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
		yjGroup, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		var candidates []model.StockLedgerYJGroup
		for _, group := range yjGroup {
			if group.IsReorderNeeded {
				candidates = append(candidates, group)
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
