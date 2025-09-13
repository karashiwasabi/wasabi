// C:\Users\wasab\OneDrive\デスクトップ\WASABI\aggregation\handler.go

package aggregation

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time" // time パッケージをインポート
	"wasabi/config"
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
 * - startDate, endDate: 集計期間（設定の日数に基づいて動的に計算）
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

		// ▼▼▼【ここから修正】▼▼▼
		// 設定ファイルから集計日数を読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 日数から期間を動的に計算
		now := time.Now()
		// 終了日は無制限とするため、実質的に未来の最大値を設定
		endDate := "99991231"
		startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)

		// フィルタ構造体に計算した値を使用する
		filters := model.AggregationFilters{
			StartDate:    startDate.Format("20060102"),
			EndDate:      endDate,
			KanaName:     q.Get("kanaName"),
			DrugTypes:    strings.Split(q.Get("drugTypes"), ","),
			DosageForm:   q.Get("dosageForm"),
			Coefficient:  coefficient,
			MovementOnly: q.Get("movementOnly") == "true",
		}
		// ▲▲▲【修正ここまで】▲▲▲

		results, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get aggregated data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
