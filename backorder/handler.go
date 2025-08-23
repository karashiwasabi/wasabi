// C:\Dev\WASABI\backorder\handler.go

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

type BackorderView struct {
	model.Backorder
	FormattedPackageSpec string `json:"formattedPackageSpec"`
}

func GetBackordersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backorders, err := db.GetAllBackordersList(conn)
		if err != nil {
			http.Error(w, "Failed to get backorder list", http.StatusInternalServerError)
			return
		}

		backorderViews := make([]BackorderView, 0, len(backorders))
		for _, bo := range backorders {
			tempJcshms := model.JCShms{
				JC037: bo.PackageForm,
				JC039: bo.YjUnitName,
				JC044: bo.YjPackUnitQty,
				JA006: sql.NullFloat64{Float64: bo.JanPackInnerQty, Valid: true},
				JA008: sql.NullFloat64{Float64: bo.JanPackUnitQty, Valid: true},
				JA007: sql.NullString{String: fmt.Sprintf("%d", bo.JanUnitCode), Valid: true},
			}

			// ▼▼▼ [修正点] 呼び出す関数を新しい簡易版に変更 ▼▼▼
			formattedSpec := units.FormatSimplePackageSpec(&tempJcshms)
			// ▲▲▲ 修正ここまで ▲▲▲

			backorderViews = append(backorderViews, BackorderView{
				Backorder:            bo,
				FormattedPackageSpec: formattedSpec,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(backorderViews)
	}
}

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
