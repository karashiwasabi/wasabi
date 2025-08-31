// C:\Users\wasab\OneDrive\デスクトップ\WASABI\aggregation\handler.go

package aggregation

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"
)

/**
 * @brief 在庫元帳データ（集計結果）を取得するためのHTTPハンドラを返します。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * HTTPリクエストのクエリパラメータからフィルタ条件を抽出し、
 * それに基づいて在庫元帳データを生成してJSON形式で返却します。
 * - coefficient: 発注点係数 (デフォルト: 1.5)
 * - startDate, endDate: 集計期間
 * - kanaName: 製品名/カナ名での絞り込み
 * - drugTypes: 薬品種別での絞り込み (毒, 劇など)
 * - dosageForm: 剤型での絞り込み
 * - movementOnly: 期間内に動きがあった品目のみを対象とするか
 */
func GetAggregationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		coefficient, err := strconv.ParseFloat(q.Get("coefficient"), 64)
		if err != nil {
			coefficient = 1.5 // Default value
		}

		filters := model.AggregationFilters{
			StartDate:    q.Get("startDate"),
			EndDate:      q.Get("endDate"),
			KanaName:     q.Get("kanaName"),
			DrugTypes:    strings.Split(q.Get("drugTypes"), ","),
			DosageForm:   q.Get("dosageForm"),
			Coefficient:  coefficient,
			MovementOnly: q.Get("movementOnly") == "true",
		}

		results, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get aggregated data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
