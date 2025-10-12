// C:\Users\wasab\OneDrive\デスクトップ\WASABI\masteredit\handler.go
package masteredit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

// GetEditableMastersHandler returns a list of editable product masters.
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

// UpdateMasterHandler updates or inserts a product master record from the edit screen.
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

// CreateProvisionalMasterHandler はGS1コードに基づいてマスターを作成します。
// JCSHMSに存在すれば正規マスターを、存在しなければ仮マスターを作成します。
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

		// --- ステップ1: まずGS1コードでJCSHMSマスターを検索 ---
		jcshmsRecord, foundJanCode, err = db.GetJcshmsRecordByGS1(tx, req.Gs1Code)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Failed to search JCSHMS master by GS1: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// --- ステップ2: GS1で見つからなかった場合、JANコードで再検索 ---
		if jcshmsRecord == nil {
			jcshmsRecord, err = db.GetJcshmsRecordByJan(tx, req.ProductCode)
			if err != nil && err != sql.ErrNoRows {
				http.Error(w, "Failed to search JCSHMS master by JAN: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if jcshmsRecord != nil {
				foundJanCode = req.ProductCode // JAN検索で見つかったので、リクエストのJANコードを使用
			}
		}

		var yjCodeToReturn string
		if jcshmsRecord != nil {
			// --- JCSHMSに存在した場合：正規マスターとして登録 ---
			input := mappers.JcshmsToProductMasterInput(jcshmsRecord, foundJanCode)
			input.Gs1Code = req.Gs1Code // バーコードから読み取ったGS1コードを常に正とする

			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				http.Error(w, "Failed to create master from JCSHMS: "+err.Error(), http.StatusInternalServerError)
				return
			}
			yjCodeToReturn = input.YjCode
		} else {
			// --- JCSHMSに存在しない場合：仮マスターとして登録 ---
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
