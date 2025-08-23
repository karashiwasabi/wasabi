// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\guided_inventory.go

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/mappers"
	"wasabi/model"
)

// SaveGuidedInventoryDataは、棚卸調整画面からのデータをトランザクション内で保存します。
// 1. 関連する包装の既存棚卸データを削除
// 2. 画面で入力された棚卸データを登録（入力がなかった包装は在庫0で登録）
// 3. 数量が1以上の品目の既存デッドストック情報を削除
// 4. 新しいデッドストック情報を登録
// SaveGuidedInventoryDataは、棚卸調整画面からのデータをトランザクション内で保存します。
func SaveGuidedInventoryData(tx *sql.Tx, date string, allPackagings []model.ProductMaster, inventoryData map[string]float64, deadstockData []model.DeadStockRecord) error {

	var allProductCodes []string
	mastersMap := make(map[string]*model.ProductMaster)
	for _, pkg := range allPackagings {
		allProductCodes = append(allProductCodes, pkg.ProductCode)
		// ポインタを渡すために一時変数を利用
		p := pkg
		mastersMap[pkg.ProductCode] = &p
	}

	// 関連する包装の既存棚卸データをまとめて削除
	if err := DeleteTransactionsByFlagAndDateAndCodes(tx, 0, date, allProductCodes); err != nil {
		return fmt.Errorf("failed to delete old inventory records: %w", err)
	}

	// ▼▼▼ 修正点: 棚卸レコードの保存ロジックをより厳密化 ▼▼▼
	const q = `
INSERT INTO transaction_records (
    transaction_date, client_code, receipt_number, line_number, flag,
    jan_code, yj_code, product_name, kana_name, usage_classification, package_form, package_spec, maker_name,
    dat_quantity, jan_pack_inner_qty, jan_quantity, jan_pack_unit_qty, jan_unit_name, jan_unit_code,
    yj_quantity, yj_pack_unit_qty, yj_unit_name, unit_price, purchase_price, supplier_wholesale,
	subtotal, tax_amount, tax_rate, expiry_date, lot_number, flag_poison,
    flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant,
    flag_stimulant_raw, process_flag_ma
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare statement for inventory records: %w", err)
	}
	defer stmt.Close()

	receiptNumber := fmt.Sprintf("INV%s", date)
	var productCodesWithInventory []string

	for i, productCode := range allProductCodes {
		master, ok := mastersMap[productCode]
		if !ok {
			continue // マスターが見つからない場合はスキップ
		}

		yjQty := inventoryData[productCode] // 入力がなければゼロ値

		if yjQty > 0 {
			productCodesWithInventory = append(productCodesWithInventory, productCode)
		}

		// 棚卸レコード用のクリーンな構造体を作成
		tr := model.TransactionRecord{
			TransactionDate: date,
			Flag:            0,
			ReceiptNumber:   receiptNumber,
			LineNumber:      fmt.Sprintf("ADJ%d", i+1),
			YjQuantity:      yjQty,
			ProcessFlagMA:   "COMPLETE",
		}

		// mappers.MapProductMasterToTransaction を使いつつ、不要な情報をクリア
		mappers.MapProductMasterToTransaction(&tr, master)
		tr.ClientCode = ""        // 不要な情報をクリア
		tr.SupplierWholesale = "" // 不要な情報をクリア
		tr.Subtotal = 0           // 棚卸に金額はない

		if master.JanPackInnerQty > 0 {
			tr.JanQuantity = yjQty / master.JanPackInnerQty
		}

		// ステートメントを実行
		_, err := stmt.Exec(
			tr.TransactionDate, tr.ClientCode, tr.ReceiptNumber, tr.LineNumber, tr.Flag,
			tr.JanCode, tr.YjCode, tr.ProductName, tr.KanaName, tr.UsageClassification, tr.PackageForm, tr.PackageSpec, tr.MakerName,
			tr.DatQuantity, tr.JanPackInnerQty, tr.JanQuantity, tr.JanPackUnitQty, tr.JanUnitName, tr.JanUnitCode,
			tr.YjQuantity, tr.YjPackUnitQty, tr.YjUnitName, tr.UnitPrice, tr.PurchasePrice, tr.SupplierWholesale,
			tr.Subtotal, tr.TaxAmount, tr.TaxRate, tr.ExpiryDate, tr.LotNumber, tr.FlagPoison,
			tr.FlagDeleterious, tr.FlagNarcotic, tr.FlagPsychotropic, tr.FlagStimulant,
			tr.FlagStimulantRaw, tr.ProcessFlagMA,
		)
		if err != nil {
			return fmt.Errorf("failed to insert inventory record for %s: %w", productCode, err)
		}
	}
	// ▲▲▲ 修正ここまで ▲▲▲

	// デッドストックデータの保存
	if len(productCodesWithInventory) > 0 {
		// 数量が入力された品目のデッドストック情報のみをフィルタリング
		var relevantDeadstockData []model.DeadStockRecord
		for _, ds := range deadstockData {
			for _, pid := range productCodesWithInventory {
				if ds.ProductCode == pid {
					relevantDeadstockData = append(relevantDeadstockData, ds)
					break
				}
			}
		}

		if err := DeleteDeadStockRecordsByProductCodes(tx, productCodesWithInventory); err != nil {
			return fmt.Errorf("failed to delete old dead stock records: %w", err)
		}
		if err := UpsertDeadStockRecordsInTx(tx, relevantDeadstockData); err != nil {
			return fmt.Errorf("failed to upsert new dead stock records: %w", err)
		}
	}

	return nil
}

// DeleteDeadStockRecordsByProductCodes は指定された製品コード群のデッドストックレコードを削除します。
func DeleteDeadStockRecordsByProductCodes(tx *sql.Tx, productCodes []string) error {
	if len(productCodes) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf("DELETE FROM dead_stock_list WHERE product_code IN (%s)", placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete dead stock records by product codes: %w", err)
	}
	return nil
}
