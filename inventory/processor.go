package inventory

import (
	"database/sql"
	"fmt"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
	"wasabi/model"
)

// ▼▼▼ [修正点] DAT/USAGEプロセッサと同様の定数を定義 ▼▼▼
const (
	FlagComplete    = "COMPLETE"
	FlagProvisional = "PROVISIONAL"
)

// ▲▲▲ 修正ここまで ▲▲▲

// ProcessInventoryRecords processes de-duplicated inventory records into transaction records.
func ProcessInventoryRecords(tx *sql.Tx, conn *sql.DB, records []model.UnifiedInputRecord) ([]model.TransactionRecord, error) {
	if len(records) == 0 {
		return []model.TransactionRecord{}, nil
	}

	// --- 1. Prepare necessary data in bulk ---
	var keyList, janList []string
	keySet, janSet := make(map[string]struct{}), make(map[string]struct{})
	for _, rec := range records {
		if rec.JanCode != "" && rec.JanCode != "0000000000000" {
			if _, seen := janSet[rec.JanCode]; !seen {
				janSet[rec.JanCode] = struct{}{}
				janList = append(janList, rec.JanCode)
			}
		}
		key := rec.JanCode
		if key == "" || key == "0000000000000" {
			key = fmt.Sprintf("9999999999999%s", rec.ProductName)
		}
		if _, seen := keySet[key]; !seen {
			keySet[key] = struct{}{}
			keyList = append(keyList, key)
		}
	}
	mastersMap, err := db.GetProductMastersByCodesMap(conn, keyList)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk get product masters: %w", err)
	}
	jcshmsMap, err := db.GetJcshmsByCodesMap(conn, janList)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk get jcshms: %w", err)
	}

	// --- 2. Process records one by one ---
	var finalRecords []model.TransactionRecord
	for _, rec := range records {
		tr := model.TransactionRecord{
			Flag:        0, // Inventory flag
			JanCode:     rec.JanCode,
			ProductName: rec.ProductName,
			YjQuantity:  rec.YjQuantity,
		}

		// --- 3. Call mastermanager to find or create the master ---
		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			return nil, fmt.Errorf("mastermanager failed for jan %s: %w", rec.JanCode, err)
		}

		// --- 4. Calculate inventory-specific transaction info using the confirmed master ---
		if master.JanPackInnerQty > 0 {
			tr.JanQuantity = tr.YjQuantity / master.JanPackInnerQty
		}

		mappers.MapProductMasterToTransaction(&tr, master)

		// ▼▼▼ [修正点] マスターの由来(Origin)によって処理ステータスを分岐 ▼▼▼
		if master.Origin == "JCSHMS" {
			tr.ProcessFlagMA = FlagComplete
			tr.ProcessingStatus = sql.NullString{String: "completed", Valid: true}
		} else {
			tr.ProcessFlagMA = FlagProvisional
			tr.ProcessingStatus = sql.NullString{String: "provisional", Valid: true}
		}
		// ▲▲▲ 修正ここまで ▲▲▲

		finalRecords = append(finalRecords, tr)
	}
	return finalRecords, nil
}
