package dat

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

// ProcessDatRecords は、重複除去済みのDATレコードを受け取り、トランザクションレコードを生成します。
func ProcessDatRecords(tx *sql.Tx, conn *sql.DB, records []model.UnifiedInputRecord) ([]model.TransactionRecord, error) {
	if len(records) == 0 {
		return []model.TransactionRecord{}, nil
	}

	// --- 1. 処理に必要な情報を一括で準備 ---
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

	// --- 2. レコードを一件ずつ処理 ---
	var finalRecords []model.TransactionRecord
	for _, rec := range records {
		ar := model.TransactionRecord{
			TransactionDate: rec.Date, ClientCode: rec.ClientCode, ReceiptNumber: rec.ReceiptNumber,
			LineNumber: rec.LineNumber, Flag: rec.Flag, JanCode: rec.JanCode,
			ProductName: rec.ProductName, DatQuantity: rec.DatQuantity, UnitPrice: rec.UnitPrice,
			Subtotal: rec.Subtotal, ExpiryDate: rec.ExpiryDate, LotNumber: rec.LotNumber,
		}

		// --- 3. mastermanagerを呼び出してマスターを確定 ---
		master, err := mastermanager.FindOrCreate(tx, rec.JanCode, rec.ProductName, mastersMap, jcshmsMap)
		if err != nil {
			return nil, fmt.Errorf("mastermanager failed for jan %s: %w", rec.JanCode, err)
		}

		// --- 4. 確定したマスターを基に、DAT固有のトランザクション情報を計算・設定 ---
		if master.JanPackUnitQty > 0 {
			ar.JanQuantity = ar.DatQuantity * master.JanPackUnitQty
		}
		if master.YjPackUnitQty > 0 {
			ar.YjQuantity = ar.DatQuantity * master.YjPackUnitQty
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
