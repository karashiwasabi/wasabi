package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/mappers"
	"wasabi/model"
)

func SaveGuidedInventoryData(tx *sql.Tx, date string, yjCode string, allPackagings []model.ProductMaster, inventoryData map[string]float64, deadstockData []model.DeadStockRecord) error {
	var allProductCodes []string
	mastersMap := make(map[string]*model.ProductMaster)
	for _, pkg := range allPackagings {
		allProductCodes = append(allProductCodes, pkg.ProductCode)
		p := pkg
		mastersMap[pkg.ProductCode] = &p
	}

	if len(allProductCodes) > 0 {
		placeholders := strings.Repeat("?,", len(allProductCodes)-1) + "?"
		pastDeleteQuery := fmt.Sprintf(`DELETE FROM transaction_records WHERE flag = 0 AND transaction_date < ? AND jan_code IN (%s)`, placeholders)
		args := make([]interface{}, 0, len(allProductCodes)+1)
		args = append(args, date)
		for _, code := range allProductCodes {
			args = append(args, code)
		}
		if _, err := tx.Exec(pastDeleteQuery, args...); err != nil {
			return fmt.Errorf("failed to delete past inventory records: %w", err)
		}
	}

	if err := DeleteTransactionsByFlagAndDateAndCodes(tx, 0, date, allProductCodes); err != nil {
		return fmt.Errorf("failed to delete old inventory records for the same day: %w", err)
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

	receiptNumber := fmt.Sprintf("ADJ-%s-%s", date, yjCode)
	var productCodesWithInventory []string

	for i, productCode := range allProductCodes {
		master, ok := mastersMap[productCode]
		if !ok {
			continue
		}

		janQty := inventoryData[productCode]
		if janQty > 0 {
			productCodesWithInventory = append(productCodesWithInventory, productCode)
		}

		tr := model.TransactionRecord{
			TransactionDate: date,
			Flag:            0,
			ReceiptNumber:   receiptNumber,
			LineNumber:      fmt.Sprintf("%d", i+1),
			JanQuantity:     janQty,
			ProcessFlagMA:   "COMPLETE",
		}

		tr.YjQuantity = janQty * master.JanPackInnerQty
		mappers.MapProductMasterToTransaction(&tr, master)
		tr.ClientCode = ""
		tr.SupplierWholesale = ""

		// ▼▼▼【修正】Subtotalを計算する処理を追加 ▼▼▼
		tr.Subtotal = tr.YjQuantity * tr.UnitPrice
		// ▲▲▲【修正ここまで】▲▲▲

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

		if err := DeleteDeadStockByProductCodesInTx(tx, productCodesWithInventory); err != nil {
			return fmt.Errorf("failed to delete old dead stock records: %w", err)
		}
		if err := SaveDeadStockListInTx(tx, relevantDeadstockData); err != nil {
			return fmt.Errorf("failed to upsert new dead stock records: %w", err)
		}
	}

	return nil
}
