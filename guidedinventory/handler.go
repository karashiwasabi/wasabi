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

type StockLedgerYJGroupView struct {
	model.StockLedgerYJGroup
	PackageLedgers []StockLedgerPackageGroupView `json:"packageLedgers"`
}
type StockLedgerPackageGroupView struct {
	model.StockLedgerPackageGroup
	Masters []model.ProductMasterView `json:"masters"`
}
type ResponseDataView struct {
	TransactionLedger []StockLedgerYJGroupView  `json:"transactionLedger"`
	YesterdaysStock   *StockLedgerYJGroupView   `json:"yesterdaysStock"`
	PrecompDetails    []model.TransactionRecord `json:"precompDetails"`
	DeadStockDetails  []model.DeadStockRecord   `json:"deadStockDetails"`
}

// ▼▼▼【ここから修正】▼▼▼
// データベースから取得したモデルを、画面表示用のビューモデルに変換する
func convertToView(yjGroups []model.StockLedgerYJGroup) []StockLedgerYJGroupView {
	if yjGroups == nil {
		return nil
	}

	viewGroups := make([]StockLedgerYJGroupView, 0, len(yjGroups))

	for _, group := range yjGroups {
		newYjGroup := StockLedgerYJGroupView{
			StockLedgerYJGroup: group,
			PackageLedgers:     make([]StockLedgerPackageGroupView, 0, len(group.PackageLedgers)),
		}

		for _, pkg := range group.PackageLedgers {
			newPkgGroup := StockLedgerPackageGroupView{
				StockLedgerPackageGroup: pkg,
				Masters:                 make([]model.ProductMasterView, 0, len(pkg.Masters)),
			}

			for _, master := range pkg.Masters {
				// 包装仕様の文字列を生成するために一時的な構造体にデータを詰め替える
				tempJcshms := model.JCShms{
					JC037: master.PackageForm,
					JC039: master.YjUnitName,
					JC044: master.YjPackUnitQty,
					JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
					JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
					JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
				}

				// JAN単位名を解決する
				var janUnitName string
				if master.JanUnitCode == 0 {
					janUnitName = master.YjUnitName
				} else {
					janUnitName = units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode))
				}

				newMasterView := model.ProductMasterView{
					ProductMaster:        *master,
					FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
					JanUnitName:          janUnitName,
				}
				newPkgGroup.Masters = append(newPkgGroup.Masters, newMasterView)
			}
			newYjGroup.PackageLedgers = append(newYjGroup.PackageLedgers, newPkgGroup)
		}
		viewGroups = append(viewGroups, newYjGroup)
	}
	return viewGroups
}

// ▲▲▲【修正ここまで】▲▲▲

func GetInventoryDataHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		yjCode := q.Get("yjCode")
		if yjCode == "" {
			http.Error(w, "yjCode is a required parameter", http.StatusBadRequest)
			return
		}
		now := time.Now()
		endDate := now
		startDate := now.AddDate(0, 0, -30)
		yesterdayDate := now.AddDate(0, 0, -1)
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
		filtersYesterday := model.AggregationFilters{
			StartDate: startDate.Format("20060102"),
			EndDate:   yesterdayDate.Format("20060102"),
			YjCode:    yjCode,
		}
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
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction for dead stock details", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		deadStockDetails, err := db.GetDeadStockByYjCode(tx, yjCode)
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
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		masters, err := db.GetProductMastersByYjCode(tx, payload.YjCode)
		if err != nil {
			http.Error(w, "Failed to get product masters for yj: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var allPackagings []model.ProductMaster
		for _, m := range masters {
			allPackagings = append(allPackagings, *m)
		}
		if err := db.SaveGuidedInventoryData(tx, payload.Date, payload.YjCode, allPackagings, payload.InventoryData, payload.DeadStockData); err != nil {
			http.Error(w, "Failed to save inventory data: "+err.Error(), http.StatusInternalServerError)
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
