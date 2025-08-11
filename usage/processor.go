package usage

import (
	"database/sql"
	"fmt"
	"wasabi/db"
	"wasabi/mappers"
	"wasabi/mastermanager"
	"wasabi/model"
)

// ProcessFlagMA の値を定数として定義
const (
	FlagComplete    = "COMPLETE"    // データ完了（JCSHMS由来のマスター）
	FlagProvisional = "PROVISIONAL" // 暫定データ（仮マスター）、継続的な更新対象
)

// ProcessUsageRecords processes de-duplicated USAGE records into transaction records.
func ProcessUsageRecords(tx *sql.Tx, conn *sql.DB, records []model.UnifiedInputRecord) ([]model.TransactionRecord, error) {
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
		ar := model.TransactionRecord{
			TransactionDate: rec.Date,
			Flag:            3, // USAGE flag
			JanCode:         rec.JanCode,
			YjCode:          rec.YjCode,
			ProductName:     rec.ProductName,
			YjQuantity:      rec.YjQuantity,
			YjUnitName:      rec.YjUnitName,
		}

		// --- 3. Call mastermanager to find or create the master ---
		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			return nil, fmt.Errorf("mastermanager failed for jan %s: %w", rec.JanCode, err)
		}

		// --- 4. Calculate USAGE-specific transaction info using the confirmed master ---
		// Ensure the transaction's JanCode matches the master's primary key
		ar.JanCode = master.ProductCode

		if master.JanPackInnerQty > 0 {
			ar.JanQuantity = ar.YjQuantity / master.JanPackInnerQty
		} else {
			ar.JanQuantity = ar.YjQuantity // Default to 1-to-1 if not specified
		}

		// マスター情報をトランザクションにマッピング
		mappers.MapProductMasterToTransaction(&ar, master)

		// ▼▼▼ 修正箇所 ▼▼▼
		// マスターの由来(Origin)によって処理ステータスを分岐する
		if master.Origin == "JCSHMS" {
			// JCSHMS由来のマスターが見つかった場合は、処理完了とする
			ar.ProcessFlagMA = FlagComplete
			ar.ProcessingStatus = sql.NullString{String: "completed", Valid: true}
		} else {
			// JCSHMS由来でないマスター（手入力やPROVISIONAL）に紐付いた場合は、
			// まだ情報が不完全な可能性があるため、仮登録状態とする
			ar.ProcessFlagMA = FlagProvisional
			ar.ProcessingStatus = sql.NullString{String: "provisional", Valid: true}
		}
		// ▲▲▲ 修正箇所 ▲▲▲

		finalRecords = append(finalRecords, ar)
	}
	return finalRecords, nil
}
