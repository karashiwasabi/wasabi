// C:\Users\wasab\OneDrive\デスクトップ\WASABI\product\handler.go

package product

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// kanaRowMap defines the characters for each row of the Japanese syllabary.
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

// SearchProductsHandler searches for products by dosage form and the first character of their Kana name.
func SearchProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		dosageForm := q.Get("dosageForm")
		kanaInitial := q.Get("kanaInitial")

		query := `SELECT ` + db.SelectColumns + ` FROM product_master WHERE yj_code != ''`
		var args []interface{}

		if dosageForm != "" {
			query += " AND usage_classification = ?"
			args = append(args, dosageForm)
		}

		if kanaInitial != "" {
			if kanaChars, ok := kanaRowMap[toKatakana(kanaInitial)]; ok {
				var conditions []string
				for _, char := range kanaChars {
					hiraChar := toHiragana(char)
					kataChar := toKatakana(char)
					conditions = append(conditions, "kana_name LIKE ? OR kana_name LIKE ?")
					args = append(args, kataChar+"%", hiraChar+"%")
				}
				query += " AND (" + strings.Join(conditions, " OR ") + ")"
			}
		}

		query += " ORDER BY kana_name"

		rows, err := conn.Query(query, args...)
		if err != nil {
			http.Error(w, "Failed to search products: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []model.ProductMasterView
		seenYjCodes := make(map[string]bool)
		for rows.Next() {
			master, err := db.ScanProductMaster(rows)
			if err != nil {
				http.Error(w, "Failed to scan product: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if !seenYjCodes[master.YjCode] {
				tempJcshms := model.JCShms{
					JC037: master.PackageForm,
					JC039: master.YjUnitName,
					JC044: master.YjPackUnitQty,
					JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
					JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
					JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
				}
				formattedSpec := units.FormatPackageSpec(&tempJcshms)

				view := model.ProductMasterView{
					ProductMaster:        *master,
					FormattedPackageSpec: formattedSpec,
				}
				results = append(results, view)
				seenYjCodes[master.YjCode] = true
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
