// C:\Dev\WASABI\inventory\manual_handler.go

package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/model"
)

// ListInventoryProductsHandler returns all product masters for the manual inventory screen.
func ListInventoryProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get product list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(products)
	}
}

type ManualInventoryRecord struct {
	ProductCode string  `json:"productCode"`
	YjQuantity  float64 `json:"yjQuantity"`
}

type ManualInventoryPayload struct {
	Date    string                  `json:"date"`
	Records []ManualInventoryRecord `json:"records"`
}

// SaveManualInventoryHandler saves the manually entered inventory counts.
func SaveManualInventoryHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload ManualInventoryPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if payload.Date == "" {
			http.Error(w, "Date is required", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := db.DeleteTransactionsByFlagAndDate(tx, 0, payload.Date); err != nil {
			http.Error(w, "Failed to clear old inventory data", http.StatusInternalServerError)
			return
		}

		var productCodes []string
		recordsMap := make(map[string]float64)
		for _, rec := range payload.Records {
			if rec.YjQuantity != 0 {
				productCodes = append(productCodes, rec.ProductCode)
				recordsMap[rec.ProductCode] = rec.YjQuantity
			}
		}

		if len(productCodes) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "保存するデータがありませんでした。"})
			return
		}

		mastersMap, err := db.GetProductMastersByCodesMap(tx, productCodes)
		if err != nil {
			http.Error(w, "Failed to get product masters", http.StatusInternalServerError)
			return
		}

		var finalRecords []model.TransactionRecord
		receiptNumber := fmt.Sprintf("INV%s", payload.Date)

		for i, code := range productCodes {
			master, ok := mastersMap[code]
			if !ok {
				continue
			}

			tr := model.TransactionRecord{
				TransactionDate: payload.Date,
				Flag:            0, // 0 = Inventory
				JanCode:         master.ProductCode,
				YjQuantity:      recordsMap[code],
				ReceiptNumber:   receiptNumber,
				LineNumber:      fmt.Sprintf("%d", i+1),
			}

			// ▼▼▼ [修正点] ProcessingStatusの設定を削除 ▼▼▼
			if master.Origin == "JCSHMS" {
				tr.ProcessFlagMA = "COMPLETE"
			} else {
				tr.ProcessFlagMA = "PROVISIONAL"
			}
			// ▲▲▲ 修正ここまで ▲▲▲

			if master.JanPackInnerQty > 0 {
				tr.JanQuantity = tr.YjQuantity / master.JanPackInnerQty
			}

			mappers.MapProductMasterToTransaction(&tr, master)
			finalRecords = append(finalRecords, tr)
		}

		if err := db.PersistTransactionRecordsInTx(tx, finalRecords); err != nil {
			http.Error(w, "Failed to save inventory records", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("%d件の棚卸データを保存しました。", len(finalRecords))})
	}
}