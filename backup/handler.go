// C:\Dev\WASABI\backorder\handler.go

package backorder

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units" // unitsパッケージをインポート
)

// ▼▼▼ フロントエンドに渡すための専用のデータ構造を定義 ▼▼▼
type BackorderView struct {
	model.Backorder
	FormattedPackageSpec string `json:"formattedPackageSpec"`
}

// GetBackordersHandler は発注残リストを返します。
func GetBackordersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backorders, err := db.GetAllBackordersList(conn)
		if err != nil {
			http.Error(w, "Failed to get backorder list", http.StatusInternalServerError)
			return
		}

		// ▼▼▼ 取得したデータをViewモデルに変換 ▼▼▼
		backorderViews := make([]BackorderView, 0, len(backorders))
		for _, bo := range backorders {
			// units.FormatPackageSpec を使って正しい包装仕様を生成
			// backordersテーブルにはJANコード単位の情報がないため、YJコード単位の情報で組み立てる
			tempJcshms := model.JCShms{
				JC037: bo.PackageForm,
				JC039: bo.YjUnitName,
				JC044: bo.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: bo.JanPackInnerQty, Valid: true},
			}
			formattedSpec := units.FormatPackageSpec(&tempJcshms)

			backorderViews = append(backorderViews, BackorderView{
				Backorder:            bo,
				FormattedPackageSpec: formattedSpec,
			})
		}
		// ▲▲▲ 変換ここまで ▲▲▲

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(backorderViews) // 変換後のデータを返す
	}
}

// ▼▼▼ [修正点] 以下の関数をファイル末尾に追加 ▼▼▼
// DeleteBackorderHandler は発注残を削除します。
func DeleteBackorderHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload model.Backorder
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

		if err := db.DeleteBackorderInTx(tx, payload); err != nil {
			http.Error(w, "Failed to delete backorder: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "発注残を削除しました。"})
	}
}
