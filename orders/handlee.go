// C:\Users\wasab\OneDrive\デスクトップ\WASABI\orders\handlee.go
package orders

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

type OrderCandidatesResponse struct {
	Candidates  []OrderCandidateYJGroup `json:"candidates"`
	Wholesalers []model.Wholesaler      `json:"wholesalers"`
}

type OrderCandidateYJGroup struct {
	model.StockLedgerYJGroup
	PackageLedgers []OrderCandidatePackageGroup `json:"packageLedgers"`
}

type OrderCandidatePackageGroup struct {
	model.StockLedgerPackageGroup
	Masters            []model.ProductMasterView `json:"masters"`
	ExistingBackorders []model.Backorder         `json:"existingBackorders"`
}

func GenerateOrderCandidatesHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kanaName := r.URL.Query().Get("kanaName")
		dosageForm := r.URL.Query().Get("dosageForm")
		shelfNumber := r.URL.Query().Get("shelfNumber")
		coefficientStr := r.URL.Query().Get("coefficient")
		coefficient, err := strconv.ParseFloat(coefficientStr, 64)
		if err != nil {
			coefficient = 1.3
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		}

		now := time.Now()
		endDate := "99991231"
		startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)

		filters := model.AggregationFilters{
			StartDate:   startDate.Format("20060102"),
			EndDate:     endDate,
			KanaName:    kanaName,
			DosageForm:  dosageForm,
			ShelfNumber: shelfNumber,
			Coefficient: coefficient,
		}

		yjGroups, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		allBackorders, err := db.GetAllBackordersList(conn)
		if err != nil {
			http.Error(w, "Failed to get backorder list for candidates", http.StatusInternalServerError)
			return
		}
		backordersByPackageKey := make(map[string][]model.Backorder)
		for _, bo := range allBackorders {
			key := fmt.Sprintf("%s|%s|%g|%s", bo.YjCode, bo.PackageForm, bo.JanPackInnerQty, bo.YjUnitName)
			backordersByPackageKey[key] = append(backordersByPackageKey[key], bo)
		}

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
						ExistingBackorders:      backordersByPackageKey[pkg.PackageKey],
					}
					for _, master := range pkg.Masters {
						tempJcshms := model.JCShms{
							JC037: master.PackageForm,
							JC039: master.YjUnitName,
							JC044: master.YjPackUnitQty,
							JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
							JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
							JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
						}
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

		wholesalers, err := db.GetAllWholesalers(conn)
		if err != nil {
			http.Error(w, "Failed to get wholesalers", http.StatusInternalServerError)
			return
		}

		response := OrderCandidatesResponse{
			Candidates:  candidates,
			Wholesalers: wholesalers,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

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
			// ▼▼▼【ここから修正】▼▼▼
			payload[i].OrderQuantity = payload[i].YjQuantity
			payload[i].RemainingQuantity = payload[i].YjQuantity
			// ▲▲▲【修正ここまで】▲▲▲
		}

		if err := db.InsertBackordersInTx(tx, payload); err != nil {
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
