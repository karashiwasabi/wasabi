// C:\Users\wasab\OneDrive\デスクトップ\WASABI\product\handler.go

package product

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time" // time パッケージをインポート
	"wasabi/config"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

var kanaVariants = map[rune][]rune{
	'カ': {'ガ'}, 'キ': {'ギ'}, 'ク': {'グ'}, 'ケ': {'ゲ'}, 'コ': {'ゴ'},
	'サ': {'ザ'}, 'シ': {'ジ'}, 'ス': {'ズ'}, 'セ': {'ゼ'}, 'ソ': {'ゾ'},
	'タ': {'ダ'}, 'チ': {'ヂ'}, 'ツ': {'ヅ'}, 'テ': {'デ'}, 'ト': {'ド'},
	'ハ': {'バ', 'パ'}, 'ヒ': {'ビ', 'ピ'}, 'フ': {'ブ', 'プ'}, 'ヘ': {'ベ', 'ペ'}, 'ホ': {'ボ', 'ポ'},
}

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

func SearchProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		dosageForm := q.Get("dosageForm")
		kanaInitial := q.Get("kanaInitial")
		searchQuery := q.Get("q")
		isDeadStockOnly := q.Get("deadStockOnly") == "true"

		var results []model.ProductMasterView

		if isDeadStockOnly {
			// ▼▼▼【ここから修正】▼▼▼
			// 設定ファイルから集計日数を読み込む
			cfg, err := config.LoadConfig()
			if err != nil {
				http.Error(w, "設定ファイルの読み込みに失敗しました: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// 日数から期間を動的に計算
			now := time.Now()
			endDate := "99991231" // 終了日は無制限
			startDate := now.AddDate(0, 0, -cfg.CalculationPeriodDays)

			filters := model.DeadStockFilters{
				StartDate:        startDate.Format("20060102"),
				EndDate:          endDate,
				ExcludeZeroStock: true, // 在庫0は除外
				KanaName:         kanaInitial,
				DosageForm:       dosageForm,
			}
			// ▲▲▲【修正ここまで】▲▲▲

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
			query := `SELECT ` + db.SelectColumns + ` FROM product_master WHERE yj_code != ''`
			var args []interface{}

			if dosageForm != "" {
				query += " AND usage_classification LIKE ?"
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
							kataChar := string(char)
							hiraChar := toHiragana(kataChar)
							conditions = append(conditions, "kana_name LIKE ? OR kana_name LIKE ?")
							args = append(args, kataChar+"%", hiraChar+"%")
						}
					}
					if len(conditions) > 0 {
						query += " AND (" + strings.Join(conditions, " OR ") + ")"
					}
				}
			}

			if searchQuery != "" {
				query += " AND (kana_name LIKE ? OR product_name LIKE ?)"
				args = append(args, "%"+searchQuery+"%", "%"+searchQuery+"%")
			}

			query += " ORDER BY kana_name"

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
						ProductMaster:        *master,
						FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
						JanUnitName:          units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode)),
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
