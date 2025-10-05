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

// ListInventoryProductsHandler returns all product masters with their last inventory date.
func ListInventoryProductsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := db.GetAllProductMasters(conn)
		if err != nil {
			http.Error(w, "Failed to get product list: "+err.Error(), http.StatusInternalServerError)
			return
		}

		dateMap, err := db.GetLastInventoryDateMap(conn)
		if err != nil {
			http.Error(w, "Failed to get last inventory dates: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var result []model.InventoryProductView
		for _, p := range products {
			result = append(result, model.InventoryProductView{
				ProductMaster:     *p,
				LastInventoryDate: dateMap[p.ProductCode],
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
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

		var productCodes []string
		recordsMap := make(map[string]float64)
		for _, rec := range payload.Records {
			productCodes = append(productCodes, rec.ProductCode)
			recordsMap[rec.ProductCode] = rec.YjQuantity
		}

		if len(productCodes) > 0 {
			if err := db.DeleteTransactionsByFlagAndDateAndCodes(tx, 0, payload.Date, productCodes); err != nil {
				http.Error(w, "Failed to clear old inventory data for specified products", http.StatusInternalServerError)
				return
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

			if master.Origin == "JCSHMS" {
				tr.ProcessFlagMA = "COMPLETE"
			} else {
				tr.ProcessFlagMA = "PROVISIONAL"
			}

			if master.JanPackInnerQty > 0 {
				tr.JanQuantity = tr.YjQuantity / master.JanPackInnerQty
			}

			mappers.MapProductMasterToTransaction(&tr, master)

			// ▼▼▼【修正】Subtotalを計算する処理を追加 ▼▼▼
			tr.Subtotal = tr.YjQuantity * tr.UnitPrice
			// ▲▲▲【修正ここまで】▲▲▲

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
