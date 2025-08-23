// C:\Users\wasab\OneDrive\デスクトップ\WASABI\guidedinventory\handler.go

package guidedinventory

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
	"wasabi/model"
)

// ResponseData は棚卸調整画面に必要な全てのデータをまとめた構造体です。
type ResponseData struct {
	StockLedger    []model.StockLedgerYJGroup    `json:"stockLedger"`
	PrecompDetails []db.PreCompoundingDetailView `json:"precompDetails"`
}

// GetInventoryDataHandler は、指定されたYJコードの在庫元帳と予製明細を取得します。
func GetInventoryDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filters := model.AggregationFilters{
			StartDate: q.Get("startDate"),
			EndDate:   q.Get("endDate"),
			YjCode:    q.Get("yjCode"),
		}

		if filters.YjCode == "" || filters.StartDate == "" || filters.EndDate == "" {
			http.Error(w, "yjCode, startDate, and endDate are required parameters", http.StatusBadRequest)
			return
		}

		// 1. 在庫元帳データを取得
		stockLedger, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get stock ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. 関連する製品コードを収集
		var productCodes []string
		if len(stockLedger) > 0 {
			for _, pkgLedger := range stockLedger[0].PackageLedgers {
				for _, master := range pkgLedger.Masters {
					productCodes = append(productCodes, master.ProductCode)
				}
			}
		}

		// 3. 予製明細データを取得
		precompDetails, err := db.GetPreCompoundingRecordsByProductCodes(conn, productCodes)
		if err != nil {
			http.Error(w, "Failed to get pre-compounding details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 4. 結果をまとめてレスポンスとして返す
		response := ResponseData{
			StockLedger:    stockLedger,
			PrecompDetails: precompDetails,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// SavePayload は棚卸データの保存時にフロントエンドから受け取るデータの形式です。
type SavePayload struct {
	Date          string                  `json:"date"`
	YjCode        string                  `json:"yjCode"`
	InventoryData map[string]float64      `json:"inventoryData"`
	DeadStockData []model.DeadStockRecord `json:"deadStockData"`
}

// SaveInventoryDataHandler は棚卸調整画面で入力されたデータを保存します。
func SaveInventoryDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload SavePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if payload.Date == "" || payload.YjCode == "" {
			http.Error(w, "Date and YjCode are required", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// YJコードに紐づく全ての包装マスターを取得 (ゼロ埋め処理のため)
		allPackagings, err := db.GetProductMastersByYjCode(tx, payload.YjCode)
		if err != nil {
			http.Error(w, "Failed to get product masters for yj_code: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var masterPackagings []model.ProductMaster
		for _, p := range allPackagings {
			masterPackagings = append(masterPackagings, p.ProductMaster)
		}

		// DB保存用の関数を呼び出し
		if err := db.SaveGuidedInventoryData(tx, payload.Date, masterPackagings, payload.InventoryData, payload.DeadStockData); err != nil {
			http.Error(w, "Failed to save guided inventory data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "棚卸データを保存しました。"})
	}
}
