// C:\Users\wasab\OneDrive\デスクトップ\WASABI\backorder\handler.go

package backorder

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/model"
	"wasabi/units"
)

// BackorderView は発注残データを画面表示用に整形するための構造体です。
// model.Backorder の全フィールドに加え、画面表示用の包装仕様文字列を持ちます。
type BackorderView struct {
	model.Backorder
	FormattedPackageSpec string `json:"formattedPackageSpec"`
}

/**
 * @brief 全ての発注残リストを取得し、画面表示用に整形して返すためのHTTPハンドラです。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 */
func GetBackordersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backorders, err := db.GetAllBackordersList(conn)
		if err != nil {
			http.Error(w, "Failed to get backorder list", http.StatusInternalServerError)
			return
		}

		backorderViews := make([]BackorderView, 0, len(backorders))
		for _, bo := range backorders {
			// unitsパッケージの関数に渡すため、一時的にJCShmsモデルの形式に変換
			tempJcshms := model.JCShms{
				JC037: bo.PackageForm,
				JC039: bo.YjUnitName,
				JC044: bo.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: bo.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: bo.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", bo.JanUnitCode), Valid: true},
			}

			formattedSpec := units.FormatSimplePackageSpec(&tempJcshms)

			backorderViews = append(backorderViews, BackorderView{
				Backorder:            bo,
				FormattedPackageSpec: formattedSpec,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(backorderViews)
	}
}

/**
 * @brief 単一の発注残レコードを削除するためのHTTPハンドラです。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * HTTPリクエストのボディから削除対象のBackorder情報を受け取り、DBから削除します。
 */
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

/**
 * @brief 複数の発注残レコードを一括で削除するためのHTTPハンドラです。
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * HTTPリクエストのボディから削除対象のBackorder情報の配列を受け取り、ループ処理でDBから削除します。
 * 処理は単一のトランザクション内で行われ、一件でも失敗した場合は全てロールバックされます。
 */
func BulkDeleteBackordersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []model.Backorder
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(payload) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "削除する項目がありません。"})
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		for _, bo := range payload {
			if err := db.DeleteBackorderInTx(tx, bo); err != nil {
				http.Error(w, "Failed to delete backorder: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "選択された発注残を削除しました。"})
	}
}
