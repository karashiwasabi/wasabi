// C:\Users\wasab\OneDrive\デス\WASABI\db\product_master.go

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"wasabi/model"
	"wasabi/units"
)

// SelectColumns は product_master テーブルからレコードを取得する際の標準的なカラムリストです。
// この定数を使用することで、クエリ全体でカラムの順序と内容の一貫性を保ちます。
const SelectColumns = `
	product_code, yj_code, product_name, origin, kana_name, maker_name,
	usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
	flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
	flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
	jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
`

/**
 * @brief データベースの行データから model.ProductMaster 構造体に値をスキャンします。
 * @param row スキャン対象の行 (*sql.Row または *sql.Rows)
 * @return *model.ProductMaster スキャン結果のポインタ
 * @return error スキャン中にエラーが発生した場合
 * @details
 * SelectColumns定数で定義された順序でカラムをスキャンします。
 */
func ScanProductMaster(row interface{ Scan(...interface{}) error }) (*model.ProductMaster, error) {
	var m model.ProductMaster
	err := row.Scan(
		&m.ProductCode, &m.YjCode, &m.ProductName, &m.Origin, &m.KanaName, &m.MakerName,
		&m.UsageClassification, &m.PackageForm, &m.YjUnitName, &m.YjPackUnitQty,
		&m.FlagPoison, &m.FlagDeleterious, &m.FlagNarcotic, &m.FlagPsychotropic,
		&m.FlagStimulant, &m.FlagStimulantRaw, &m.JanPackInnerQty, &m.JanUnitCode,
		&m.JanPackUnitQty, &m.NhiPrice, &m.PurchasePrice, &m.SupplierWholesale,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

/**
 * @brief 新しい製品マスターレコードをトランザクション内で作成します。
 * @param tx SQLトランザクションオブジェクト
 * @param rec 登録する製品マスターの入力データ
 * @return error 処理中にエラーが発生した場合
 */
func CreateProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
	const q = `INSERT INTO product_master (
		product_code, yj_code, product_name, origin, kana_name, maker_name,
		usage_classification, package_form, yj_unit_name, yj_pack_unit_qty,
		flag_poison, flag_deleterious, flag_narcotic, flag_psychotropic,
		flag_stimulant, flag_stimulant_raw, jan_pack_inner_qty, jan_unit_code,
		jan_pack_unit_qty, nhi_price, purchase_price, supplier_wholesale
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	if err != nil {
		return fmt.Errorf("CreateProductMasterInTx failed: %w", err)
	}
	return nil
}

/**
 * @brief 製品マスターレコードをトランザクション内でUPSERT（挿入または更新）します。
 * @param tx SQLトランザクションオブジェクト
 * @param rec 登録・更新する製品マスターの入力データ
 * @return error 処理中にエラーが発生した場合
 * @details
 * product_codeが競合した場合は、INSERTの代わりにUPDATEが実行されます。
 */
func UpsertProductMasterInTx(tx *sql.Tx, rec model.ProductMasterInput) error {
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

	_, err := tx.Exec(q,
		rec.ProductCode, rec.YjCode, rec.ProductName, rec.Origin, rec.KanaName, rec.MakerName,
		rec.UsageClassification, rec.PackageForm, rec.YjUnitName, rec.YjPackUnitQty,
		rec.FlagPoison, rec.FlagDeleterious, rec.FlagNarcotic, rec.FlagPsychotropic,
		rec.FlagStimulant, rec.FlagStimulantRaw, rec.JanPackInnerQty, rec.JanUnitCode,
		rec.JanPackUnitQty, rec.NhiPrice, rec.PurchasePrice, rec.SupplierWholesale,
	)
	if err != nil {
		return fmt.Errorf("UpsertProductMasterInTx failed: %w", err)
	}
	return nil
}

/**
 * @brief 製品コードをキーに単一の製品マスターを取得します。
 * @param dbtx DBTXインターフェース（*sql.DB または *sql.Tx）
 * @param code 検索対象の製品コード(JANコード)
 * @return *model.ProductMaster 取得した製品マスター。見つからない場合はnil。
 * @return error 処理中にエラーが発生した場合
 */
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

/**
 * @brief 複数の製品コードをキーに製品マスターをマップ形式で取得します。
 * @param dbtx DBTXインターフェース
 * @param codes 検索対象の製品コードのスライス
 * @return map[string]*model.ProductMaster 製品コードをキーとした製品マスターのマップ
 * @return error 処理中にエラーが発生した場合
 * @details
 * N+1問題を避けるためIN句で一括取得します。
 */
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

/**
 * @brief 編集可能な製品マスター（JCSHMS由来でないもの）を全て取得します。
 * @param conn データベース接続
 * @return []model.ProductMaster 製品マスタースライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 「マスター」画面での表示や、製品マスターのエクスポート機能で使用されます。
 */
func GetEditableProductMasters(conn *sql.DB) ([]model.ProductMaster, error) {
	q := `SELECT ` + SelectColumns + ` FROM product_master WHERE origin != 'JCSHMS' ORDER BY kana_name`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("GetEditableProductMasters failed: %w", err)
	}
	defer rows.Close()

	var masters []model.ProductMaster
	for rows.Next() {
		m, err := ScanProductMaster(rows)
		if err != nil {
			return nil, err
		}
		masters = append(masters, *m)
	}
	return masters, nil
}

/**
 * @brief 全ての製品マスターレコードを取得します。
 * @param dbtx DBTXインターフェース
 * @return []*model.ProductMaster 製品マスターのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 剤型区分とカナ名でソートされた順序で返します。
 */
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

	rows, err := dbtx.Query(q)
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

/**
 * @brief YJコードに紐づく全ての製品マスターを表示用のViewモデルとして取得します。
 * @param dbtx DBTXインターフェース
 * @param yjCode 検索対象のYJコード
 * @return []model.ProductMasterView 画面表示用の製品マスタースライス
 * @return error 処理中にエラーが発生した場合
 */
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

		tempJcshms := model.JCShms{
			JC037: m.PackageForm,
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

/**
 * @brief 複数の製品の納入価格と主要卸を一括で更新します。
 * @param tx SQLトランザクションオブジェクト
 * @param updates 更新内容のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 「価格更新」画面で使用されます。
 */
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

/**
 * @brief product_masterテーブルの全レコードを削除します。
 * @param conn データベース接続
 * @return error 処理中にエラーが発生した場合
 * @details
 * 関連するMA2Yのシーケンスもリセットします。
 */
func ClearAllProductMasters(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction for clearing masters: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM product_master`); err != nil {
		return fmt.Errorf("failed to execute delete from product_master: %w", err)
	}

	// MA2Yのシーケンスもリセットする
	if _, err := tx.Exec(`UPDATE code_sequences SET last_no = 0 WHERE name = 'MA2Y'`); err != nil {
		log.Printf("Could not reset sequence for MA2Y (this is normal if table was empty): %v", err)
	}

	return tx.Commit()
}
