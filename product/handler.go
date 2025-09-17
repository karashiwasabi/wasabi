// C:\Users\wasab\OneDrive\デスクトップ\WASABI\product\handler.go

package product

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// (kanaVariants, kanaRowMap, toHiragana, toKatakana, SearchProductsHandler は変更なし)
var kanaVariants = map[rune][]rune{
	'カ': {'ガ'}, 'キ': {'ギ'}, 'ク': {'グ'}, 'ケ': {'ゲ'}, 'コ': {'ゴ'},
	'サ': {'ザ'}, 'シ': {'ジ'}, 'ス': {'ズ'}, 'セ': {'ゼ'}, 'ソ': {'ゾ'},
	'タ': {'ダ'}, 'チ': {'ヂ'}, 'ツ': {'ヅ'}, 'テ': {'デ'}, 'ト': {'ド'},
	'ハ': {'バ', 'パ'}, 'ヒ': {'ビ', 'ピ'}, 'フ': {'ブ', 'プ'}, 'ヘ': {'ベ', 'ペ'}, 'ホ': {'ボ', 'ポ'},
}
var kanaRowMap = map[string][]string{
	"ア": {"ア", "イ", "ウ", "エ", "オ"}, "カ": {"カ", "キ", "ク", "ケ", "コ"}, "サ": {"サ", "シ", "ス", "セ", "ソ"},
	"タ": {"タ", "チ", "ツ", "テ", "ト"}, "ナ": {"ナ", "ニ", "ヌ", "ネ", "ノ"}, "ハ": {"ハ", "ヒ", "フ", "ヘ", "ホ"},
	"マ": {"マ", "ミ", "ム", "メ", "モ"}, "ヤ": {"ヤ", "ユ", "ヨ"}, "ラ": {"ラ", "リ", "ル", "レ", "ロ"},
	"ワ": {"ワ", "ヰ", "ヱ", "ヲ", "ン"},
}

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
				StartDate: startDate.Format("20060102"), EndDate: endDate,
				ExcludeZeroStock: true, KanaName: kanaInitial, DosageForm: dosageForm,
			}
			tx, txErr := conn.Begin()
			if txErr != nil {
				http.Error(w, "Failed to start transaction for dead stock search", http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()
			deadStockGroups, dsErr := db.GetDeadStockList(tx, filters)
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
								JC037: master.PackageForm, JC039: master.YjUnitName, JC044: master.YjPackUnitQty,
								JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
								JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
								JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
							}
							view := model.ProductMasterView{
								ProductMaster: master, FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
								JanUnitName: units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode)),
							}
							results = append(results, view)
							seenYjCodes[master.YjCode] = true
						}
					}
				}
			}
		} else {
			query := `SELECT ` + db.SelectColumns + ` FROM product_master p WHERE p.yj_code != '' AND p.origin != 'PROVISIONAL'`
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
func toHiragana(s string) string {
	r := []rune(s)
	for i, c := range r {
		if c >= 'ァ' && c <= 'ヶ' {
			r[i] = c - 'ァ' + 'ぁ'
		}
	}
	return string(r)
}
func toKatakana(s string) string {
	r := []rune(s)
	for i, c := range r {
		if c >= 'ぁ' && c <= 'ゖ' {
			r[i] = c - 'ぁ' + 'ァ'
		}
	}
	return string(r)
}

// ▼▼▼【ここから全面的に修正】▼▼▼
// ProductLedgerResponse は管理台帳APIのレスポンス構造体です。
type ProductLedgerResponse struct {
	LedgerTransactions []model.LedgerTransaction `json:"ledgerTransactions"`
	PrecompDetails     []model.TransactionRecord `json:"precompDetails"`
}

// GetProductLedgerHandler は、トランザクション毎の在庫推移と関連する予製情報を返します。
func GetProductLedgerHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productCode := strings.TrimPrefix(r.URL.Path, "/api/ledger/product/")
		if productCode == "" {
			http.Error(w, "Product code is required", http.StatusBadRequest)
			return
		}

		// 1. 本日時点の理論在庫を計算 (未来の取引もすべて考慮した最終在庫)
		endingBalance, err := db.CalculateCurrentStockForProduct(conn, productCode)
		if err != nil {
			http.Error(w, "Failed to calculate current stock: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. 本日より未来のトランザクションを取得し、変動量を計算
		now := time.Now()
		futureTxs, err := db.GetAllTransactionsForProductAfterDate(conn, productCode, now.Format("20060102"))
		if err != nil {
			http.Error(w, "Failed to get future transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var futureChange float64
		for _, tx := range futureTxs {
			futureChange += tx.SignedYjQty()
		}

		// 3. 本日時点の在庫を計算 (最終在庫 - 未来の変動)
		stockAsOfToday := endingBalance - futureChange

		// 4. 過去30日間のトランザクションを降順で取得
		endDate := now
		startDate := endDate.AddDate(0, 0, -30)
		transactions, err := db.GetTransactionsForProductInDateRange(conn, productCode, startDate.Format("20060102"), endDate.Format("20060102"))
		if err != nil {
			http.Error(w, "Failed to get recent transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 5. 在庫を逆算しながらLedgerTransactionのスライスを作成
		var ledgerTransactions []model.LedgerTransaction
		runningBalance := stockAsOfToday
		for _, tx := range transactions {
			entry := model.LedgerTransaction{
				TransactionRecord: tx,
				RunningBalance:    runningBalance,
			}
			ledgerTransactions = append(ledgerTransactions, entry)
			runningBalance -= tx.SignedYjQty() // 1つ前の取引の在庫を計算
		}

		// 6. 結果を昇順に並び替え
		sort.SliceStable(ledgerTransactions, func(i, j int) bool {
			if ledgerTransactions[i].TransactionDate != ledgerTransactions[j].TransactionDate {
				return ledgerTransactions[i].TransactionDate < ledgerTransactions[j].TransactionDate
			}
			return ledgerTransactions[i].ID < ledgerTransactions[j].ID
		})

		// 7. 関連する予製情報を取得 (productCodeは単一のJANコード)
		precompDetails, err := db.GetPreCompoundingDetailsByProductCodes(conn, []string{productCode})
		if err != nil {
			http.Error(w, "Failed to get pre-compounding details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 8. レスポンスを構築
		response := ProductLedgerResponse{
			LedgerTransactions: ledgerTransactions,
			PrecompDetails:     precompDetails,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// ▲▲▲【修正ここまで】▲▲▲
