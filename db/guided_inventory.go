// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\guided_inventory.go

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/mappers"
	"wasabi/model"
)

/**
 * @brief 棚卸調整画面で入力された在庫・ロット情報を保存します。
 * @param tx SQLトランザクションオブジェクト
 * @param date 棚卸日 (YYYYMMDD)
 * @param yjCode 対象のYJコード
 * @param allPackagings 対象YJコードに属する全包装のマスター情報
 * @param inventoryData JANコードをキー、YJ単位での在庫数量を値とするマップ
 * @param deadstockData 保存するロット・期限情報のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 1. 指定された日付・YJコードグループの既存の棚卸レコード(flag=0)を全て削除します。
 * 2. YJコードグループに属する全ての包装について、新しい在庫数を登録します（入力がなければ0で登録）。
 * 3. 在庫数が1以上ある品物について、既存のロット・期限情報を削除し、新しい情報を登録します。
 */
func SaveGuidedInventoryData(tx *sql.Tx, date string, yjCode string, allPackagings []model.ProductMaster, inventoryData map[string]float64, deadstockData []model.DeadStockRecord) error {
	var allProductCodes []string
	mastersMap := make(map[string]*model.ProductMaster)
	for _, pkg := range allPackagings {
		allProductCodes = append(allProductCodes, pkg.ProductCode)
		p := pkg
		mastersMap[pkg.ProductCode] = &p
	}

	if err := DeleteTransactionsByFlagAndDateAndCodes(tx, 0, date, allProductCodes); err != nil {
		return fmt.Errorf("failed to delete old inventory records: %w", err)
	}

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

	// 伝票番号にYJコードを含めることで、一意性を保証する
	receiptNumber := fmt.Sprintf("ADJ-%s-%s", date, yjCode)
	var productCodesWithInventory []string

	for i, productCode := range allProductCodes {
		master, ok := mastersMap[productCode]
		if !ok {
			continue
		}

		yjQty := inventoryData[productCode] // マップにキーがなくても0が返る

		if yjQty > 0 {
			productCodesWithInventory = append(productCodesWithInventory, productCode)
		}

		tr := model.TransactionRecord{
			TransactionDate: date,
			Flag:            0,
			ReceiptNumber:   receiptNumber,
			LineNumber:      fmt.Sprintf("%d", i+1), // 行番号はYJグループ内での連番とする
			YjQuantity:      yjQty,
			ProcessFlagMA:   "COMPLETE",
		}

		mappers.MapProductMasterToTransaction(&tr, master)
		tr.ClientCode = ""
		tr.SupplierWholesale = ""
		tr.Subtotal = 0

		if master.JanPackInnerQty > 0 {
			tr.JanQuantity = yjQty / master.JanPackInnerQty
		}

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

	// 在庫が1以上ある品目について、ロット・期限情報を更新する
	if len(productCodesWithInventory) > 0 {
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

/**
 * @brief 指定された製品コード群に紐づくデッドストック（ロット・期限）情報を削除します。
 * @param tx SQLトランザクションオブジェクト
 * @param productCodes 削除対象の製品コードのスライス
 * @return error 処理中にエラーが発生した場合
 */
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
