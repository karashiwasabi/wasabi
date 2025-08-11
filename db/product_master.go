package db

import (
	"database/sql"
	"fmt"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// selectColumns is a reusable string for selecting all columns from the product_master table.
const selectColumns = `
	product_code, yj_code, product_name, origin, kana_name, maker_name,
	usage_classification, package_form, package_spec, yj_unit_name, yj_pack_unit_qty,
	flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
	flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
	jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
`

// scanProductMaster maps a database row to a ProductMaster struct.
func scanProductMaster(row interface{ Scan(...interface{}) error }) (*model.ProductMaster, error) {
	var m model.ProductMaster
	err := row.Scan(
		&m.ProductCode, &m.YjCode, &m.ProductName, &m.Origin, &m.KanaName, &m.MakerName,
		&m.UsageClassification, &m.PackageForm, &m.PackageSpec, &m.YjUnitName, &m.YjPackUnitQty,
		&m.FlagPoison, &m.FlagDeleterious, &m.FlagNarcotic, &m.FlagPsychotropic,
		&m.FlagStimulant, &m.FlagStimulantRaw, &m.JanPackInnerQty, &m.JanUnitCode,
		&m.JanPackUnitQty, &m.NhiPrice, &m.PurchasePrice, &m.SupplierWholesale,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateProductMasterInTx creates a new product master within a transaction.
func CreateProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	const q = `INSERT INTO product_master (
		product_code, yj_code, product_name, origin, kana_name, maker_name,
		usage_classification, package_form, package_spec, yj_unit_name, yj_pack_unit_qty,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
		flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
		jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)` // Corrected to 23 placeholders

	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	if err != nil {
		return fmt.Errorf("CreateProductMasterInTx failed: %w", err)
	}
	return nil
}

// UpsertProductMasterInTx updates a product master or inserts it if it doesn't exist.
func UpsertProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	const q = `INSERT INTO product_master (
		product_code, yj_code, product_name, origin, kana_name, maker_name,
		usage_classification, package_form, package_spec, yj_unit_name, yj_pack_unit_qty,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
		flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
		jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) -- Corrected to 23 placeholders
	ON CONFLICT(product_code) DO UPDATE SET
		yj_code=excluded.yj_code, product_name=excluded.product_name, origin=excluded.origin, 
		kana_name=excluded.kana_name, maker_name=excluded.maker_name, 
		usage_classification=excluded.usage_classification, package_form=excluded.package_form, 
		package_spec=excluded.package_spec, yj_unit_name=excluded.yj_unit_name, 
		yj_pack_unit_qty=excluded.yj_pack_unit_qty, flag_poison=excluded.flag_poison, 
		flag_deleterious=excluded.flag_deleterious, flag_narcotic=excluded.flag_narcotic, 
		flag_psychotropic=excluded.flag_psychotropic, flag_stimulant=excluded.flag_stimulant, 
		flag_stimulant_raw=excluded.flag_stimulant_raw, jan_pack_inner_qty=excluded.jan_pack_inner_qty, 
		jan_unit_code=excluded.jan_unit_code, jan_pack_unit_qty=excluded.jan_pack_unit_qty, 
		nhi_price=excluded.nhi_price, purchase_price=excluded.purchase_price, 
		supplier_wholesale=excluded.supplier_wholesale` // Corrected to include the last column

	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.PackageSpec, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	if err != nil {
		return fmt.Errorf("UpsertProductMasterInTx failed: %w", err)
	}
	return nil
}

// GetProductMasterByCode は製品コードをキーに単一の製品マスターを取得します。
func GetProductMasterByCode(conn *sql.DB, code string) (*model.ProductMaster, error) {
	q := `SELECT ` + selectColumns + ` FROM product_master WHERE product_code = ? LIMIT 1`
	m, err := scanProductMaster(conn.QueryRow(q, code))
	if err == sql.ErrNoRows {
		return nil, nil // 見つからない場合はエラーではなくnilを返す
	}
	if err != nil {
		return nil, fmt.Errorf("GetProductMasterByCode failed: %w", err)
	}
	return m, nil
}

// GetProductMastersByCodesMap は複数の製品コードをキーに製品マスターをマップで取得します。
func GetProductMastersByCodesMap(conn *sql.DB, codes []string) (map[string]*model.ProductMaster, error) {
	if len(codes) == 0 {
		return make(map[string]*model.ProductMaster), nil
	}
	q := `SELECT ` + selectColumns + ` FROM product_master WHERE product_code IN (?` + strings.Repeat(",?", len(codes)-1) + `)`

	args := make([]interface{}, len(codes))
	for i, code := range codes {
		args[i] = code
	}

	rows, err := conn.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query for masters by codes failed: %w", err)
	}
	defer rows.Close()

	mastersMap := make(map[string]*model.ProductMaster)
	for rows.Next() {
		m, err := scanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		mastersMap[m.ProductCode] = m
	}
	return mastersMap, nil
}

// GetEditableProductMasters fetches all non-JCSHMS product masters for the edit screen.
func GetEditableProductMasters(conn *sql.DB) ([]model.ProductMasterView, error) {
	q := `SELECT ` + selectColumns + ` FROM product_master WHERE origin != 'JCSHMS' ORDER BY kana_name`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("GetEditableProductMasters failed: %w", err)
	}
	defer rows.Close()

	var mastersView []model.ProductMasterView
	for rows.Next() {
		m, err := scanProductMaster(rows)
		if err != nil {
			return nil, err
		}

		// Create a temporary JCShms-like struct to use the existing formatting function
		tempJcshms := model.JCShms{
			JC037: m.PackageSpec,
			JC039: m.YjUnitName,
			JC044: m.YjPackUnitQty,
			JA006: sql.NullFloat64{Float64: m.JanPackInnerQty, Valid: true},
			JA008: sql.NullFloat64{Float64: m.JanPackUnitQty, Valid: true},
			JA007: sql.NullString{String: fmt.Sprintf("%d", m.JanUnitCode), Valid: true},
		}
		formattedSpec := units.FormatPackageSpec(&tempJcshms)

		mastersView = append(mastersView, model.ProductMasterView{
			ProductMaster:        *m,
			FormattedPackageSpec: formattedSpec,
		})
	}
	return mastersView, nil
}

// GetAllProductMasters retrieves all product master records.
func GetAllProductMasters(conn *sql.DB) ([]*model.ProductMaster, error) {
	q := `SELECT ` + selectColumns + ` FROM product_master ORDER BY kana_name`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("GetAllProductMasters failed: %w", err)
	}
	defer rows.Close()

	var masters []*model.ProductMaster
	for rows.Next() {
		m, err := scanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, m)
	}
	return masters, nil
}
