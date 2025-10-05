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

func SearchProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			// ▼▼▼【ここからが修正箇所です】▼▼▼
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
			// ▲▲▲【修正ここまで】▲▲▲
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
			query += " ORDER BY p.kana_name"
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
}

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
		txs, err := db.GetAllTransactionsForProductAfterDate(conn, productCode, "20000101")
		if err != nil {
			http.Error(w, "Failed to get transaction history: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var ledgerTxs []model.LedgerTransaction
		var runningBalance float64
		for _, tx := range txs {
			runningBalance += tx.SignedYjQty()
			ledgerTxs = append(ledgerTxs, model.LedgerTransaction{
				TransactionRecord: tx,
				RunningBalance:    runningBalance,
			})
		}
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
