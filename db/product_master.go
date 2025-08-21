// C:\Dev\WASABI\db\product_master.go

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// ▼▼▼ [修正点] カラム一覧から package_spec を削除 ▼▼▼
const SelectColumns = `
	product_code, yj_code, product_name, origin, kana_name, maker_name,
	usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
	flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
	flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
	jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
`

// ▲▲▲ 修正ここまで ▲▲▲

// ScanProductMaster maps a database row to a ProductMaster struct.
func ScanProductMaster(row interface{ Scan(...interface{}) error }) (*model.ProductMaster, error) {
	var m model.ProductMaster
	// ▼▼▼ [修正点] スキャン対象から &m.PackageSpec を削除 ▼▼▼
	err := row.Scan(
		&m.ProductCode, &m.YjCode, &m.ProductName, &m.Origin, &m.KanaName, &m.MakerName,
		&m.UsageClassification, &m.PackageForm, &m.YjUnitName, &m.YjPackUnitQty,
		&m.FlagPoison, &m.FlagDeleterious, &m.FlagNarcotic, &m.FlagPsychotropic,
		&m.FlagStimulant, &m.FlagStimulantRaw, &m.JanPackInnerQty, &m.JanUnitCode,
		&m.JanPackUnitQty, &m.NhiPrice, &m.PurchasePrice, &m.SupplierWholesale,
	)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateProductMasterInTx creates a new product master within a transaction.
func CreateProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	// ▼▼▼ [修正点] INSERT文から package_spec と対応するプレースホルダを削除 ▼▼▼
	const q = `INSERT INTO product_master (
		product_code, yj_code, product_name, origin, kana_name, maker_name,
		usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
		flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
		jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// ▲▲▲ 修正ここまで ▲▲▲

	// ▼▼▼ [修正点] Execの引数から rec.PackageSpec を削除 ▼▼▼
	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return fmt.Errorf("CreateProductMasterInTx failed: %w", err)
	}
	return nil
}

// UpsertProductMasterInTx updates a product master or inserts it if it doesn't exist.
func UpsertProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	// ▼▼▼ [修正点] UPSERT文から package_spec と対応するプレースホルダ・更新句を削除 ▼▼▼
	const q = `INSERT INTO product_master (
		product_code, yj_code, product_name, origin, kana_name, maker_name,
		usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
		flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
		jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(product_code) DO UPDATE SET
		yj_code=excluded.yj_code, product_name=excluded.product_name, origin=excluded.origin, 
		kana_name=excluded.kana_name, maker_name=excluded.maker_name, 
		usage_classification=excluded.usage_classification, package_form=excluded.package_form, 
		yj_unit_name=excluded.yj_unit_name, 
		yj_pack_unit_qty=excluded.yj_pack_unit_qty, flag_poison=excluded.flag_poison, 
		flag_deleterious=excluded.flag_deleterious, flag_narcotic=excluded.flag_narcotic, 
		flag_psychotropic=excluded.flag_psychotropic, flag_stimulant=excluded.flag_stimulant, 
		flag_stimulant_raw=excluded.flag_stimulant_raw, jan_pack_inner_qty=excluded.jan_pack_inner_qty, 
		jan_unit_code=excluded.jan_unit_code, jan_pack_unit_qty=excluded.jan_pack_unit_qty, 
		nhi_price=excluded.nhi_price, purchase_price=excluded.purchase_price, 
		supplier_wholesale=excluded.supplier_wholesale`
	// ▲▲▲ 修正ここまで ▲▲▲

	// ▼▼▼ [修正点] Execの引数から rec.PackageSpec を削除 ▼▼▼
	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return fmt.Errorf("UpsertProductMasterInTx failed: %w", err)
	}
	return nil
}

// GetProductMasterByCode は製品コードをキーに単一の製品マスターを取得します。
func GetProductMasterByCode(dbtx DBTX, code string) (*model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE product_code = ? LIMIT 1`
	m, err := ScanProductMaster(dbtx.QueryRow(q, code))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetProductMasterByCode failed: %w", err)
	}
	return m, nil
}

// GetProductMastersByCodesMap は複数の製品コードをキーに製品マスターをマップで取得します。
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

// GetEditableProductMasters fetches all non-JCSHMS product masters for the edit screen.
func GetEditableProductMasters(conn *sql.DB) ([]model.ProductMasterView, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE origin != 'JCSHMS' ORDER BY kana_name`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("GetEditableProductMasters failed: %w", err)
	}
	defer rows.Close()

	var mastersView []model.ProductMasterView
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}

		// ▼▼▼ 修正点 ▼▼▼
		tempJcshms := model.JCShms{
			JC037: m.PackageForm,
			JC039: m.YjUnitName,
			JC044: m.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
		}
		formattedSpec := units.FormatPackageSpec(&tempJcshms)
		// ▲▲▲ 修正ここまで ▲▲▲

		mastersView = append(mastersView, model.ProductMasterView{
			ProductMaster:        *m,
			FormattedPackageSpec: formattedSpec,
		})
	}
	return mastersView, nil
}

// ▼▼▼ [修正点] 引数を conn *sql.DB から dbtx DBTX に変更 ▼▼▼
// GetAllProductMasters retrieves all product master records.
func GetAllProductMasters(dbtx DBTX) ([]*model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master 
		ORDER BY
			CASE
				WHEN usage_classification = '1' OR usage_classification = '内' THEN 1
				WHEN usage_classification = '2' OR usage_classification = '外' THEN 2
				WHEN usage_classification = '3' OR usage_classification = '歯' THEN 3
				WHEN usage_classification = '4' OR usage_classification = '注' THEN 4
				WHEN usage_classification = '5' OR usage_classification = '機' THEN 5
				WHEN usage_classification = '6' OR usage_classification = '他' THEN 6
				ELSE 7
			END,
			kana_name`

	rows, err := dbtx.Query(q) // conn.Query から dbtx.Query に変更
	if err != nil {
		return nil, fmt.Errorf("GetAllProductMasters failed: %w", err)
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

// ▲▲▲ 修正ここまで ▲▲▲

// GetProductMastersByYjCode はYJコードに紐づく全ての製品マスターを表示用のViewモデルとして取得します。
func GetProductMastersByYjCode(dbtx DBTX, yjCode string) ([]model.ProductMasterView, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE yj_code = ? ORDER BY product_name`
	rows, err := dbtx.Query(q, yjCode)
	if err != nil {
		return nil, fmt.Errorf("GetProductMastersByYjCode for %s failed: %w", yjCode, err)
	}
	defer rows.Close()

	var mastersView []model.ProductMasterView
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}

		// ▼▼▼ [修正点] 組み立て包装の生成に必要なデータを全て渡すように修正 ▼▼▼
		tempJcshms := model.JCShms{
			JC037: m.PackageForm,
			JC039: m.YjUnitName,
			JC044: m.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
		}
		// ▲▲▲ 修正ここまで ▲▲▲
		formattedSpec := units.FormatPackageSpec(&tempJcshms)

		mastersView = append(mastersView, model.ProductMasterView{
			ProductMaster:        *m,
			FormattedPackageSpec: formattedSpec,
		})
	}
	return mastersView, nil
}

// UpdatePricesAndSuppliersInTx は複数の製品の納入価格と主要卸を一括で更新します。
func UpdatePricesAndSuppliersInTx(tx *sql.Tx, updates []model.PriceUpdate) error {
	const q = `UPDATE product_master SET purchase_price = ?, supplier_wholesale = ? WHERE product_code = ?`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare price update statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		if _, err := stmt.Exec(u.NewPurchasePrice, u.NewSupplier, u.ProductCode); err != nil {
			return fmt.Errorf("failed to update price for product %s: %w", u.ProductCode, err)
		}
	}
	return nil
}
