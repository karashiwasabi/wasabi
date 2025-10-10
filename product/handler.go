package product

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// (kanaRowMap, toKatakana, toHiragana, kanaVariants はファイル内に存在するものとします)
var kanaRowMap = map[string][]string{}

func toKatakana(s string) string { return s }
func toHiragana(s string) string { return s }

var kanaVariants = map[rune][]rune{}

// ▼▼▼【ここから修正】▼▼▼
// 関数の構造を、コンパイラが誤解しない、より明確な形式に書き換えました。
func SearchProductsHandler(conn *sql.DB) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		dosageForm := q.Get("dosageForm")
		kanaInitial := q.Get("kanaInitial")
		searchQuery := q.Get("q")
		isDeadStockOnly := q.Get("deadStockOnly") == "true"
		drugTypesParam := q.Get("drugTypes")
		var results []model.ProductMasterView

		if isDeadStockOnly {
			cfg, err := config.LoadConfig()
			if err != nil {
				http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
				return
			}
			now := time.Now()
			endDate := "99991231"
			startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)
			filters := model.DeadStockFilters{
				StartDate:        startDate.Format("20060102"),
				EndDate:          endDate,
				ExcludeZeroStock: true,
				KanaName:         kanaInitial,
				DosageForm:       dosageForm,
			}

			deadStockGroups, dsErr := db.GetDeadStockList(conn, filters)
			if dsErr != nil {
				http.Error(w, "Failed to get dead stock list: "+dsErr.Error(), http.StatusInternalServerError)
				return
			}
			seenYjCodes := make(map[string]bool)
			for _, group := range deadStockGroups {
				for _, pkg := range group.PackageGroups {
					for _, prod := range pkg.Products {
						if !seenYjCodes[prod.YjCode] {
							master := prod.ProductMaster
							tempJcshms := model.JCShms{
								JC037: master.PackageForm,
								JC039: master.YjUnitName,
								JC044: master.YjPackUnitQty,
								JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
								JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
								JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
							}
							view := model.ProductMasterView{
								ProductMaster:        master,
								FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
								JanUnitName:          units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode)),
							}
							results = append(results, view)
							seenYjCodes[master.YjCode] = true
						}
					}
				}
			}
		} else {
			query := `SELECT ` + db.SelectColumns + ` FROM product_master p WHERE p.yj_code != ''`
			var args []interface{}
			if dosageForm != "" {
				query += " AND p.usage_classification LIKE ?"
				args = append(args, "%"+dosageForm+"%")
			}
			if kanaInitial != "" {
				if kanaChars, ok := kanaRowMap[toKatakana(kanaInitial)]; ok {
					var conditions []string
					for _, charStr := range kanaChars {
						baseRunes := []rune(charStr)
						if len(baseRunes) == 0 {
							continue
						}
						baseRune := baseRunes[0]
						charsToTest := []rune{baseRune}
						if variants, found := kanaVariants[baseRune]; found {
							charsToTest = append(charsToTest, variants...)
						}
						for _, char := range charsToTest {
							kataChar, hiraChar := string(char), toHiragana(string(char))
							conditions = append(conditions, "p.kana_name LIKE ? OR p.kana_name LIKE ?")
							args = append(args, kataChar+"%", hiraChar+"%")
						}
					}
					if len(conditions) > 0 {
						query += " AND (" + strings.Join(conditions, " OR ") + ")"
					}
				}
			}
			if drugTypesParam != "" {
				drugTypes := strings.Split(drugTypesParam, ",")
				if len(drugTypes) > 0 && drugTypes[0] != "" {
					var conditions []string
					flagMap := map[string]string{
						"poison": "p.flag_poison = 1", "deleterious": "p.flag_deleterious = 1",
						"narcotic": "p.flag_narcotic = 1", "psychotropic1": "p.flag_psychotropic = 1",
						"psychotropic2": "p.flag_psychotropic = 2", "psychotropic3": "p.flag_psychotropic = 3",
					}
					for _, dt := range drugTypes {
						if cond, ok := flagMap[dt]; ok {
							conditions = append(conditions, cond)
						}
					}
					if len(conditions) > 0 {
						query += " AND (" + strings.Join(conditions, " OR ") + ")"
					}
				}
			}
			if searchQuery != "" {
				query += " AND (p.kana_name LIKE ? OR p.product_name LIKE ?)"
				args = append(args, "%"+searchQuery+"%", "%"+searchQuery+"%")
			}
			query += ` ORDER BY
				CASE
					WHEN TRIM(p.usage_classification) = '内' OR TRIM(p.usage_classification) = '1' THEN 1
					WHEN TRIM(p.usage_classification) = '外' OR TRIM(p.usage_classification) = '2' THEN 2
					WHEN TRIM(p.usage_classification) = '注' OR TRIM(p.usage_classification) = '3' THEN 3
					WHEN TRIM(p.usage_classification) = '歯' OR TRIM(p.usage_classification) = '4' THEN 4
					WHEN TRIM(p.usage_classification) = '機' OR TRIM(p.usage_classification) = '5' THEN 5
					WHEN TRIM(p.usage_classification) = '他' OR TRIM(p.usage_classification) = '6' THEN 6
					ELSE 7
				END,
				p.kana_name`
			rows, queryErr := conn.Query(query, args...)
			if queryErr != nil {
				http.Error(w, "Failed to search products: "+queryErr.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			seenYjCodes := make(map[string]bool)
			for rows.Next() {
				master, scanErr := db.ScanProductMaster(rows)
				if scanErr != nil {
					http.Error(w, "Failed to scan product: "+scanErr.Error(), http.StatusInternalServerError)
					return
				}
				if !seenYjCodes[master.YjCode] {
					tempJcshms := model.JCShms{
						JC037: master.PackageForm, JC039: master.YjUnitName, JC044: master.YjPackUnitQty,
						JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
						JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
						JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
					}
					view := model.ProductMasterView{
						ProductMaster: *master, FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
						JanUnitName: units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode)),
					}
					results = append(results, view)
					seenYjCodes[master.YjCode] = true
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
	return handler
}

// ▲▲▲【修正ここまで】▲▲▲

type ProductLedgerResponse struct {
	LedgerTransactions []model.LedgerTransaction `json:"ledgerTransactions"`
	PrecompDetails     []model.TransactionRecord `json:"precompDetails"`
}

func GetProductLedgerHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productCode := strings.TrimPrefix(r.URL.Path, "/api/ledger/product/")
		if productCode == "" {
			http.Error(w, "Product code is required", http.StatusBadRequest)
			return
		}

		// GetStockLedgerのロジックを参考に、単一製品コードに特化した台帳を生成する

		// 1. 対象製品の全期間の取引を取得
		txRows, err := conn.Query(`SELECT `+db.TransactionColumns+` FROM transaction_records WHERE jan_code = ? ORDER BY transaction_date, id`, productCode)
		if err != nil {
			http.Error(w, "Failed to get transactions for product: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer txRows.Close()

		var allTxsForProduct []*model.TransactionRecord
		for txRows.Next() {
			t, err := db.ScanTransactionRecord(txRows)
			if err != nil {
				http.Error(w, "Failed to scan transaction record: "+err.Error(), http.StatusInternalServerError)
				return
			}
			allTxsForProduct = append(allTxsForProduct, t)
		}

		// 2. 表示期間を設定（直近30日間とする）
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)
		startDateStr := startDate.Format("20060102")
		endDateStr := endDate.Format("20060102")

		// 3. 期間前在庫（繰越在庫）を計算
		var startingBalance float64
		latestInventoryDateBeforePeriod := ""
		txsBeforePeriod := []*model.TransactionRecord{}
		inventorySumsByDate := make(map[string]float64)

		for _, t := range allTxsForProduct {
			if t.TransactionDate < startDateStr {
				txsBeforePeriod = append(txsBeforePeriod, t)
				if t.Flag == 0 { // 棚卸レコード
					inventorySumsByDate[t.TransactionDate] += t.YjQuantity
					if t.TransactionDate > latestInventoryDateBeforePeriod {
						latestInventoryDateBeforePeriod = t.TransactionDate
					}
				}
			}
		}

		if latestInventoryDateBeforePeriod != "" {
			startingBalance = inventorySumsByDate[latestInventoryDateBeforePeriod]
			for _, t := range txsBeforePeriod {
				if t.TransactionDate > latestInventoryDateBeforePeriod {
					startingBalance += t.SignedYjQty()
				}
			}
		} else {
			for _, t := range txsBeforePeriod {
				startingBalance += t.SignedYjQty()
			}
		}

		// 4. 期間内の変動と残高を計算
		var ledgerTxs []model.LedgerTransaction
		runningBalance := startingBalance

		periodInventorySums := make(map[string]float64)
		for _, t := range allTxsForProduct {
			if t.TransactionDate >= startDateStr && t.TransactionDate <= endDateStr && t.Flag == 0 {
				periodInventorySums[t.TransactionDate] += t.YjQuantity
			}
		}

		lastProcessedDate := ""
		for _, t := range allTxsForProduct {
			if t.TransactionDate >= startDateStr && t.TransactionDate <= endDateStr {
				if t.TransactionDate != lastProcessedDate && lastProcessedDate != "" {
					if inventorySum, ok := periodInventorySums[lastProcessedDate]; ok {
						runningBalance = inventorySum
					}
				}

				// 棚卸(flag=0)の場合はその日の棚卸合計値で残高を上書きし、それ以外は変動量を加算する
				if t.Flag == 0 {
					if inventorySum, ok := periodInventorySums[t.TransactionDate]; ok {
						runningBalance = inventorySum
					}
				} else {
					runningBalance += t.SignedYjQty()
				}

				ledgerTxs = append(ledgerTxs, model.LedgerTransaction{TransactionRecord: *t, RunningBalance: runningBalance})
				lastProcessedDate = t.TransactionDate
			}
		}

		// 5. 関連する予製引当を取得
		precomps, err := db.GetPreCompoundingDetailsByProductCodes(conn, []string{productCode})
		if err != nil {
			http.Error(w, "Failed to get precomp details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProductLedgerResponse{
			LedgerTransactions: ledgerTxs,
			PrecompDetails:     precomps,
		})
	}
}
