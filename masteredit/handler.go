// C:\Users\wasab\OneDrive\デスクトップ\WASABI\masteredit\handler.go (全体)
package masteredit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

// (GetEditableMastersHandler, UpdateMasterHandler, CreateProvisionalMasterHandler, SetOrderStoppedHandler は変更なし)

func GetEditableMastersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		masters, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get editable masters", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masters)
	}
}
func UpdateMasterHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var input model.ProductMasterInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if input.ProductCode == "" {
			http.Error(w, "Product Code (JAN) cannot be empty.", http.StatusBadRequest)
			return
		}
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if err := db.UpsertProductMasterInTx(tx, input); err != nil {
			http.Error(w, "Failed to upsert product master", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Saved successfully."})
	}
}

type CreateProvisionalMasterRequest struct {
	Gs1Code     string `json:"gs1Code"`
	ProductCode string `json:"productCode"` // 13-digit JAN
}

func CreateProvisionalMasterHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateProvisionalMasterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Gs1Code == "" || req.ProductCode == "" {
			http.Error(w, "gs1Code and productCode are required", http.StatusBadRequest)
			return
		}
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		var jcshmsRecord *model.JCShms
		var foundJanCode string
		jcshmsRecord, foundJanCode, err = db.GetJcshmsRecordByGS1(tx, req.Gs1Code)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Failed to search JCSHMS master by GS1: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if jcshmsRecord == nil {
			jcshmsRecord, err = db.GetJcshmsRecordByJan(tx, req.ProductCode)
			if err != nil && err != sql.ErrNoRows {
				http.Error(w, "Failed to search JCSHMS master by JAN: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if jcshmsRecord != nil {
				foundJanCode = req.ProductCode
			}
		}
		var yjCodeToReturn string
		if jcshmsRecord != nil {
			input := mappers.JcshmsToProductMasterInput(jcshmsRecord, foundJanCode)
			input.Gs1Code = req.Gs1Code
			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				http.Error(w, "Failed to create master from JCSHMS: "+err.Error(), http.StatusInternalServerError)
				return
			}
			yjCodeToReturn = input.YjCode
		} else {
			newYjCode, err := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
			if err != nil {
				http.Error(w, "Failed to generate new YJ code", http.StatusInternalServerError)
				return
			}
			provisionalInput := model.ProductMasterInput{
				ProductCode: req.ProductCode,
				Gs1Code:     req.Gs1Code,
				YjCode:      newYjCode,
				ProductName: fmt.Sprintf("(JCSHMS未登録 %s)", req.Gs1Code),
				Origin:      "PROVISIONAL",
			}
			if err := db.UpsertProductMasterInTx(tx, provisionalInput); err != nil {
				http.Error(w, "Failed to create provisional master", http.StatusInternalServerError)
				return
			}
			yjCodeToReturn = newYjCode
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"yjCode": yjCodeToReturn})
	}
}

type SetOrderStoppedRequest struct {
	ProductCode string `json:"productCode"`
	Status      int    `json:"status"` // 0: 発注可, 1: 発注不可
}

func SetOrderStoppedHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SetOrderStoppedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.ProductCode == "" {
			http.Error(w, "productCode is required", http.StatusBadRequest)
			return
		}
		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		master, err := db.GetProductMasterByCode(tx, req.ProductCode)
		if err != nil {
			http.Error(w, "Failed to get product master: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if master == nil {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		master.IsOrderStopped = req.Status
		input := model.ProductMasterInput{
			ProductCode:         master.ProductCode,
			YjCode:              master.YjCode,
			Gs1Code:             master.Gs1Code,
			ProductName:         master.ProductName,
			Specification:       master.Specification,
			KanaName:            master.KanaName,
			MakerName:           master.MakerName,
			UsageClassification: master.UsageClassification,
			PackageForm:         master.PackageForm,
			YjUnitName:          master.YjUnitName,
			YjPackUnitQty:       master.YjPackUnitQty,
			JanPackInnerQty:     master.JanPackInnerQty,
			JanUnitCode:         master.JanUnitCode,
			JanPackUnitQty:      master.JanPackUnitQty,
			Origin:              master.Origin,
			NhiPrice:            master.NhiPrice,
			PurchasePrice:       master.PurchasePrice,
			FlagPoison:          master.FlagPoison,
			FlagDeleterious:     master.FlagDeleterious,
			FlagNarcotic:        master.FlagNarcotic,
			FlagPsychotropic:    master.FlagPsychotropic,
			FlagStimulant:       master.FlagStimulant,
			FlagStimulantRaw:    master.FlagStimulantRaw,
			IsOrderStopped:      master.IsOrderStopped,
			SupplierWholesale:   master.SupplierWholesale,
			GroupCode:           master.GroupCode,
			ShelfNumber:         master.ShelfNumber,
			Category:            master.Category,
			UserNotes:           master.UserNotes,
		}
		if err := db.UpsertProductMasterInTx(tx, input); err != nil {
			http.Error(w, "Failed to update product master", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "更新しました。"})
	}
}

// ▼▼▼【ここから追加】▼▼▼
// BulkUpdateShelfNumbersRequest は棚番一括更新APIのリクエストボディの構造体です。
type BulkUpdateShelfNumbersRequest struct {
	ShelfNumber string   `json:"shelfNumber"`
	Gs1Codes    []string `json:"gs1Codes"`
}

// BulkUpdateShelfNumbersHandler は、複数のGS1コードに紐づく品目の棚番を一括で更新します。
func BulkUpdateShelfNumbersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req BulkUpdateShelfNumbersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.ShelfNumber == "" || len(req.Gs1Codes) == 0 {
			http.Error(w, "shelfNumber and gs1Codes are required", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare("UPDATE product_master SET shelf_number = ? WHERE gs1_code = ?")
		if err != nil {
			http.Error(w, "Failed to prepare update statement: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		var updatedCount int
		var notFoundCodes []string

		for _, gs1Code := range req.Gs1Codes {
			// (01)が付いている場合や前後に空白がある場合を考慮
			cleanGs1 := strings.TrimSpace(strings.TrimPrefix(gs1Code, "(01)"))

			res, err := stmt.Exec(req.ShelfNumber, cleanGs1)
			if err != nil {
				// 個々のエラーはログに出力するが、処理は続行する
				log.Printf("Failed to update shelf number for gs1_code %s: %v", cleanGs1, err)
				continue
			}

			rowsAffected, err := res.RowsAffected()
			if err != nil {
				log.Printf("Failed to get affected rows for gs1_code %s: %v", cleanGs1, err)
				continue
			}

			if rowsAffected > 0 {
				updatedCount++
			} else {
				notFoundCodes = append(notFoundCodes, gs1Code)
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		message := fmt.Sprintf("%d件の棚番を更新しました。", updatedCount)
		if len(notFoundCodes) > 0 {
			message += fmt.Sprintf(" %d件は見つかりませんでした: %s", len(notFoundCodes), strings.Join(notFoundCodes, ", "))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": message})
	}
}

// ▲▲▲【追加ここまで】▲▲▲
