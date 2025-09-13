// C:\Users\wasab\OneDrive\デスクトップ\WASABI\guidedinventory\handler.go

package guidedinventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

type StockLedgerPackageGroupView struct {
	model.StockLedgerPackageGroup
	Masters []model.ProductMasterView `json:"masters"`
}

type StockLedgerYJGroupView struct {
	model.StockLedgerYJGroup
	PackageLedgers []StockLedgerPackageGroupView `json:"packageLedgers"`
}

type ResponseDataView struct {
	TransactionLedger []StockLedgerYJGroupView  `json:"transactionLedger"`
	YesterdaysStock   *StockLedgerYJGroupView   `json:"yesterdaysStock"`
	PrecompDetails    []model.TransactionRecord `json:"precompDetails"`
	DeadStockDetails  []model.DeadStockRecord   `json:"deadStockDetails"`
}

func GetInventoryDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		yjCode := q.Get("yjCode")

		if yjCode == "" {
			http.Error(w, "yjCode is a required parameter", http.StatusBadRequest)
			return
		}

		// ▼▼▼【ここから修正】▼▼▼
		// 設定ファイルに依存せず、期間を「本日より30日前まで」に固定する
		now := time.Now()
		endDate := now
		startDate := now.AddDate(0, 0, -30) // 30日前に固定
		yesterdayDate := now.AddDate(0, 0, -1)

		// 1. 本日までの取引履歴を含む元帳を取得
		filtersToday := model.AggregationFilters{
			StartDate: startDate.Format("20060102"),
			EndDate:   endDate.Format("20060102"),
			YjCode:    yjCode,
		}
		ledgerToday, err := db.GetStockLedger(conn, filtersToday)
		if err != nil {
			http.Error(w, "Failed to get today's stock ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. 前日時点の理論在庫を計算するための元帳を取得
		filtersYesterday := model.AggregationFilters{
			StartDate: startDate.Format("20060102"),
			EndDate:   yesterdayDate.Format("20060102"),
			YjCode:    yjCode,
		}
		// ▲▲▲【修正ここまで】▲▲▲

		ledgerYesterday, err := db.GetStockLedger(conn, filtersYesterday)
		if err != nil {
			http.Error(w, "Failed to get yesterday's stock ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}

		transactionLedgerView := convertToView(ledgerToday)
		var yesterdaysStockView *StockLedgerYJGroupView
		if len(ledgerYesterday) > 0 {
			view := convertToView(ledgerYesterday)
			if len(view) > 0 {
				yesterdaysStockView = &view[0]
			}
		}

		var productCodes []string
		if len(ledgerToday) > 0 {
			for _, pkg := range ledgerToday[0].PackageLedgers {
				for _, master := range pkg.Masters {
					productCodes = append(productCodes, master.ProductCode)
				}
			}
		}

		precompDetails, err := db.GetPreCompoundingDetailsByProductCodes(conn, productCodes)
		if err != nil {
			http.Error(w, "Failed to get pre-compounding details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		deadStockDetails, err := db.GetDeadStockByProductCodes(conn, productCodes)
		if err != nil {
			log.Printf("WARN: Failed to get dead stock details for inventory adjustment: %v", err)
		}

		response := ResponseDataView{
			TransactionLedger: transactionLedgerView,
			YesterdaysStock:   yesterdaysStockView,
			PrecompDetails:    precompDetails,
			DeadStockDetails:  deadStockDetails,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// (これ以降の convertToView, SaveInventoryDataHandler などの関数は変更ありません)
func convertToView(ledgerData []model.StockLedgerYJGroup) []StockLedgerYJGroupView {
	view := make([]StockLedgerYJGroupView, 0, len(ledgerData))
	for _, yjGroup := range ledgerData {
		newYjGroupView := StockLedgerYJGroupView{
			StockLedgerYJGroup: yjGroup,
			PackageLedgers:     make([]StockLedgerPackageGroupView, 0, len(yjGroup.PackageLedgers)),
		}
		for _, pkgLedger := range yjGroup.PackageLedgers {
			newPkgLedgerView := StockLedgerPackageGroupView{
				StockLedgerPackageGroup: pkgLedger,
				Masters:                 make([]model.ProductMasterView, 0, len(pkgLedger.Masters)),
			}
			for _, master := range pkgLedger.Masters {
				tempJcshms := model.JCShms{
					JC037: master.PackageForm, JC039: master.YjUnitName, JC044: master.YjPackUnitQty,
					JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
					JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
					JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
				}
				masterView := model.ProductMasterView{
					ProductMaster:        *master,
					FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
					JanUnitName:          units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode)),
				}
				newPkgLedgerView.Masters = append(newPkgLedgerView.Masters, masterView)
			}
			newYjGroupView.PackageLedgers = append(newYjGroupView.PackageLedgers, newPkgLedgerView)
		}
		view = append(view, newYjGroupView)
	}
	return view
}

type SavePayload struct {
	Date          string                  `json:"date"`
	YjCode        string                  `json:"yjCode"`
	InventoryData map[string]float64      `json:"inventoryData"`
	DeadStockData []model.DeadStockRecord `json:"deadStockData"`
}

func SaveInventoryDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload SavePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("ERROR: Failed to decode request body in SaveInventoryDataHandler: %v", err)
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if payload.Date == "" || payload.YjCode == "" {
			http.Error(w, "Date and YjCode are required", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			log.Printf("ERROR: Failed to start transaction: %v", err)
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		allPackagingsViews, err := db.GetProductMastersByYjCode(tx, payload.YjCode)
		if err != nil {
			log.Printf("ERROR: Failed to get product masters for yj_code %s: %v", payload.YjCode, err)
			http.Error(w, "Failed to get product masters for yj_code: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var masterPackagings []model.ProductMaster
		for _, p := range allPackagingsViews {
			masterPackagings = append(masterPackagings, p.ProductMaster)
		}

		if err := db.SaveGuidedInventoryData(tx, payload.Date, payload.YjCode, masterPackagings, payload.InventoryData, payload.DeadStockData); err != nil {
			log.Printf("ERROR: Failed to save guided inventory data: %v", err)
			http.Error(w, "Failed to save guided inventory data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			log.Printf("ERROR: Failed to commit transaction: %v", err)
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "棚卸データを保存しました。"})
	}
}
