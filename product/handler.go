// C:\Users\wasab\OneDrive\デスクトップ\WASABI\product\handler.go
package product

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

var kanaRowMap = map[string][]string{
	"ア": {"ア", "イ", "ウ", "エ", "オ"},
	"カ": {"カ", "キ", "ク", "ケ", "コ"},
	"サ": {"サ", "シ", "ス", "セ", "ソ"},
	"タ": {"タ", "チ", "ツ", "テ", "ト"},
	"ナ": {"ナ", "ニ", "ヌ", "ネ", "ノ"},
	"ハ": {"ハ", "ヒ", "フ", "ヘ", "ホ"},
	"マ": {"マ", "ミ", "ム", "メ", "モ"},
	"ヤ": {"ヤ", "ユ", "ヨ"},
	"ラ": {"ラ", "リ", "ル", "レ", "ロ"},
	"ワ": {"ワ", "ヰ", "ヱ", "ヲ", "ン"},
}

func toKatakana(s string) string {
	var res string
	for _, r := range s {
		if r >= 'ぁ' && r <= 'ゔ' {
			res += string(r + 0x60)
		} else {
			res += string(r)
		}
	}
	return res
}
func toHiragana(s string) string {
	var res string
	for _, r := range s {
		if r >= 'ァ' && r <= 'ヴ' {
			res += string(r - 0x60)
		} else {
			res += string(r)
		}
	}
	return res
}

var kanaVariants = map[rune][]rune{
	'ア': {'ァ'}, 'イ': {'ィ'}, 'ウ': {'ゥ'}, 'エ': {'ェ'}, 'オ': {'ォ'},
	'カ': {'ガ'}, 'キ': {'ギ'}, 'ク': {'グ'}, 'ケ': {'ゲ'}, 'コ': {'ゴ'},
	'サ': {'ザ'}, 'シ': {'ジ'}, 'ス': {'ズ'}, 'セ': {'ゼ'}, 'ソ': {'ゾ'},
	'タ': {'ダ'}, 'チ': {'ヂ'}, 'ツ': {'ッ', 'ヅ'}, 'テ': {'デ'}, 'ト': {'ド'},
	'ハ': {'バ', 'パ'}, 'ヒ': {'ビ', 'ピ'}, 'フ': {'ブ', 'プ'}, 'ヘ': {'ベ', 'ペ'}, 'ホ': {'ボ', 'ポ'},
	'ヤ': {'ャ'}, 'ユ': {'ュ'}, 'ヨ': {'ョ'},
	'ワ': {'ヮ'},
}

func SearchProductsHandler(conn *sql.DB) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		dosageForm := q.Get("dosageForm")
		kanaInitial := q.Get("kanaInitial")
		searchQuery := q.Get("q")
		isDeadStockOnly := q.Get("deadStockOnly") == "true"
		drugTypesParam := q.Get("drugTypes")
		shelfNumber := q.Get("shelfNumber")
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
				KanaName:         searchQuery,
				DosageForm:       dosageForm,
				ShelfNumber:      shelfNumber,
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
							view := mappers.ToProductMasterView(&master)
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
			if shelfNumber != "" {
				query += " AND p.shelf_number LIKE ?"
				args = append(args, "%"+shelfNumber+"%")
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
					view := mappers.ToProductMasterView(master)
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

type ProductLedgerResponse struct {
	LedgerTransactions []model.LedgerTransaction `json:"ledgerTransactions"`
	PrecompDetails     []model.TransactionRecord `json:"precompDetails"`
}

// ▼▼▼【ここから修正】▼▼▼
func GetProductLedgerHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productCode := strings.TrimPrefix(r.URL.Path, "/api/ledger/product/")
		if productCode == "" {
			http.Error(w, "Product code is required", http.StatusBadRequest)
			return
		}

		// 対象製品のマスター情報を取得してYJコードを得る
		master, err := db.GetProductMasterByCode(conn, productCode)
		if err != nil {
			http.Error(w, "Failed to get master for product: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if master == nil {
			http.Error(w, "Product master not found", http.StatusNotFound)
			return
		}

		// 期間設定 (過去30日)
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		// 修正済みのGetStockLedgerを呼び出す
		filters := model.AggregationFilters{
			StartDate: startDate.Format("20060102"),
			EndDate:   endDate.Format("20060102"),
			YjCode:    master.YjCode,
		}
		ledgerGroups, err := db.GetStockLedger(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get stock ledger for product: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 結果から該当製品の取引履歴を抽出する
		var ledgerTxs []model.LedgerTransaction
		for _, group := range ledgerGroups {
			for _, pkg := range group.PackageLedgers {
				for _, m := range pkg.Masters {
					if m.ProductCode == productCode {
						ledgerTxs = pkg.Transactions
						goto Found
					}
				}
			}
		}
	Found:

		// 予製情報を取得
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

// ▲▲▲【修正ここまで】▲▲▲

func GetMasterByCodeHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productCode := strings.TrimPrefix(r.URL.Path, "/api/master/by_code/")
		if productCode == "" {
			http.Error(w, "Product code is required", http.StatusBadRequest)
			return
		}

		master, err := db.GetProductMasterByCode(conn, productCode)
		if err != nil {
			http.Error(w, "Failed to get product by code: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if master == nil {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}

		masterView := mappers.ToProductMasterView(master)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masterView)
	}
}
