package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/model"
)

// SelectColumns は、product_masterテーブルから全列を取得するためのSQLスニペットです。
const SelectColumns = `
	product_code, yj_code, gs1_code, product_name, kana_name, maker_name,
	specification, usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
	jan_pack_inner_qty, jan_unit_code, jan_pack_unit_qty, origin,
	nhi_price, purchase_price,
	flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant, flag_stimulant_raw,
	is_order_stopped, supplier_wholesale,
	group_code, shelf_number, category, user_notes
`

// ScanProductMaster は、データベースの行データから model.ProductMaster 構造体に値を割り当てます。
func ScanProductMaster(row interface{ Scan(...interface{}) error }) (*model.ProductMaster, error) {
	var m model.ProductMaster
	err := row.Scan(
		// 基本情報
		&m.ProductCode, &m.YjCode, &m.Gs1Code, &m.ProductName, &m.KanaName, &m.MakerName,
		// 製品仕様情報
		&m.Specification, &m.UsageClassification, &m.PackageForm, &m.YjUnitName, &m.YjPackUnitQty,
		&m.JanPackInnerQty, &m.JanUnitCode, &m.JanPackUnitQty, &m.Origin,
		// 価格情報
		&m.NhiPrice, &m.PurchasePrice,
		// 管理フラグ・情報
		&m.FlagPoison, &m.FlagDeleterious, &m.FlagNarcotic, &m.FlagPsychotropic, &m.FlagStimulant, &m.FlagStimulantRaw,
		&m.IsOrderStopped, &m.SupplierWholesale,
		// ユーザー定義項目
		&m.GroupCode, &m.ShelfNumber, &m.Category, &m.UserNotes,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpsertProductMasterInTx は、製品マスターレコードをトランザクション内でUPSERTします。
func UpsertProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	const q = `INSERT INTO product_master (
		product_code, yj_code, gs1_code, product_name, kana_name, maker_name,
		specification, usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
		jan_pack_inner_qty, jan_unit_code, jan_pack_unit_qty, origin,
		nhi_price, purchase_price,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic, flag_stimulant, flag_stimulant_raw,
		is_order_stopped, supplier_wholesale,
		group_code, shelf_number, category, user_notes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(product_code) DO UPDATE SET
		yj_code=excluded.yj_code, gs1_code=excluded.gs1_code, product_name=excluded.product_name, kana_name=excluded.kana_name, maker_name=excluded.maker_name,
		specification=excluded.specification, usage_classification=excluded.usage_classification, package_form=excluded.package_form, yj_unit_name=excluded.yj_unit_name, yj_pack_unit_qty=excluded.yj_pack_unit_qty,
		jan_pack_inner_qty=excluded.jan_pack_inner_qty, jan_unit_code=excluded.jan_unit_code, jan_pack_unit_qty=excluded.jan_pack_unit_qty, origin=excluded.origin,
		nhi_price=excluded.nhi_price, purchase_price=excluded.purchase_price,
		flag_poison=excluded.flag_poison, flag_deleterious=excluded.flag_deleterious, flag_narcotic=excluded.flag_narcotic, flag_psychotropic=excluded.flag_psychotropic, flag_stimulant=excluded.flag_stimulant, flag_stimulant_raw=excluded.flag_stimulant_raw,
		is_order_stopped=excluded.is_order_stopped, supplier_wholesale=excluded.supplier_wholesale,
		group_code=excluded.group_code, shelf_number=excluded.shelf_number, category=excluded.category, user_notes=excluded.user_notes
	`

	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.Gs1Code, rec.ProductName, rec.KanaName, rec.MakerName,
		rec.Specification, rec.UsageClassification, rec.PackageForm, rec.YjUnitName, rec.YjPackUnitQty,
		rec.JanPackInnerQty, rec.JanUnitCode, rec.JanPackUnitQty, rec.Origin,
		rec.NhiPrice, rec.PurchasePrice,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic, rec.FlagStimulant, rec.FlagStimulantRaw,
		rec.IsOrderStopped, rec.SupplierWholesale,
		rec.GroupCode, rec.ShelfNumber, rec.Category, rec.UserNotes,
	)
	if err != nil {
		return fmt.Errorf("UpsertProductMasterInTx failed: %w", err)
	}
	return nil
}

// GetProductMastersByCodesMap は、複数の製品コードをキーに製品マスターをマップ形式で取得します。
func GetProductMastersByCodesMap(dbtx DBTX, codes []string) (map[string]*model.ProductMaster, error) {
	if len(codes) == 0 {
		return make(map[string]*model.ProductMaster), nil
	}
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE product_code IN (?` + strings.Repeat(",?", len(codes)-1) + `)`

	args := make([]interface{}, len(codes))
	for i, code := range codes {
		args[i] = code
	}

	rows, err := dbtx.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query for masters by codes failed: %w", err)
	}
	defer rows.Close()

	mastersMap := make(map[string]*model.ProductMaster)
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		mastersMap[m.ProductCode] = m
	}
	return mastersMap, nil
}

// GetProductMasterByCode は、単一の製品コードをキーに製品マスターを取得します。
func GetProductMasterByCode(dbtx DBTX, code string) (*model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE product_code = ?`
	row := dbtx.QueryRow(q, code)
	m, err := ScanProductMaster(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetProductMasterByCode failed for code %s: %w", code, err)
	}
	return m, nil
}

// GetAllProductMasters は、product_masterテーブルの全レコードを取得します。
func GetAllProductMasters(dbtx DBTX) ([]*model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master ORDER BY kana_name`

	rows, err := dbtx.Query(q)
	if err != nil {
		return nil, fmt.Errorf("GetAllProductMasters query failed: %w", err)
	}
	defer rows.Close()

	var masters []*model.ProductMaster
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, m)
	}
	return masters, nil
}

// GetProductMastersByYjCode は、YJコードをキーに製品マスターを取得します。
func GetProductMastersByYjCode(dbtx DBTX, yjCode string) ([]*model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE yj_code = ? ORDER BY product_code`
	rows, err := dbtx.Query(q, yjCode)
	if err != nil {
		return nil, fmt.Errorf("query for masters by yj code failed: %w", err)
	}
	defer rows.Close()

	var masters []*model.ProductMaster
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, m)
	}
	return masters, nil
}

// UpdatePricesAndSuppliersInTx は、納入価と採用卸を一括更新します。
func UpdatePricesAndSuppliersInTx(tx *sql.Tx, updates []model.PriceUpdate) error {
	const q = `UPDATE product_master SET purchase_price = ?, supplier_wholesale = ? WHERE product_code = ?`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("UpdatePricesAndSuppliersInTx failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		_, err := stmt.Exec(u.NewPurchasePrice, u.NewSupplier, u.ProductCode)
		if err != nil {
			// 1件のエラーで全体を止めずに、エラーを返しつつも処理を継続する（ロールバックは呼び出し元に任せる）
			return fmt.Errorf("UpdatePricesAndSuppliersInTx failed for product %s: %w", u.ProductCode, err)
		}
	}
	return nil
}

// ClearAllProductMasters は、product_masterテーブルの全レコードを削除します。
func ClearAllProductMasters(tx *sql.Tx) error {
	const q = `DELETE FROM product_master`
	_, err := tx.Exec(q)
	if err != nil {
		return fmt.Errorf("ClearAllProductMasters failed: %w", err)
	}
	return nil
}
