package db

import (
	"database/sql"
	"fmt"
	"wasabi/model"
)

// UpsertBackordersInTx は複数の発注残レコードを登録または更新します。
func UpsertBackordersInTx(tx *sql.Tx, backorders []model.Backorder) error {
	const q = `
		INSERT INTO backorders (yj_code, package_form, jan_pack_inner_qty, yj_unit_name, order_date, yj_quantity, product_name)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(yj_code, package_form, jan_pack_inner_qty, yj_unit_name) DO UPDATE SET
			yj_quantity = yj_quantity + excluded.yj_quantity,
			order_date = excluded.order_date
	`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare backorder upsert statement: %w", err)
	}
	defer stmt.Close()

	for _, bo := range backorders {
		_, err := stmt.Exec(bo.YjCode, bo.PackageForm, bo.JanPackInnerQty, bo.YjUnitName, bo.OrderDate, bo.YjQuantity, bo.ProductName)
		if err != nil {
			return fmt.Errorf("failed to execute backorder upsert for yj %s: %w", bo.YjCode, err)
		}
	}
	return nil
}

// ReconcileBackorders は納品データに基づいて発注残を消し込みます。
func ReconcileBackorders(conn *sql.DB, deliveredItems []model.Backorder) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for reconciliation: %w", err)
	}
	defer tx.Rollback()

	for _, item := range deliveredItems {
		var currentBackorderQty float64
		err := tx.QueryRow(`
			SELECT yj_quantity FROM backorders 
			WHERE yj_code = ? AND package_form = ? AND jan_pack_inner_qty = ? AND yj_unit_name = ?`,
			item.YjCode, item.PackageForm, item.JanPackInnerQty, item.YjUnitName,
		).Scan(&currentBackorderQty)

		if err != nil {
			if err == sql.ErrNoRows {
				continue // 発注残がなければスキップ
			}
			return fmt.Errorf("failed to query backorder for yj %s: %w", item.YjCode, err)
		}

		newQty := currentBackorderQty - item.YjQuantity
		if newQty <= 0 {
			_, err := tx.Exec(`
				DELETE FROM backorders 
				WHERE yj_code = ? AND package_form = ? AND jan_pack_inner_qty = ? AND yj_unit_name = ?`,
				item.YjCode, item.PackageForm, item.JanPackInnerQty, item.YjUnitName,
			)
			if err != nil {
				return fmt.Errorf("failed to delete completed backorder for yj %s: %w", item.YjCode, err)
			}
		} else {
			_, err := tx.Exec(`
				UPDATE backorders SET yj_quantity = ? 
				WHERE yj_code = ? AND package_form = ? AND jan_pack_inner_qty = ? AND yj_unit_name = ?`,
				newQty, item.YjCode, item.PackageForm, item.JanPackInnerQty, item.YjUnitName,
			)
			if err != nil {
				return fmt.Errorf("failed to update backorder for yj %s: %w", item.YjCode, err)
			}
		}
	}
	return tx.Commit()
}

// GetAllBackordersMap は全ての発注残を、集計で使いやすいキーのマップ形式で取得します。
func GetAllBackordersMap(conn *sql.DB) (map[string]float64, error) {
	rows, err := conn.Query("SELECT yj_code, package_form, jan_pack_inner_qty, yj_unit_name, yj_quantity FROM backorders")
	if err != nil {
		return nil, fmt.Errorf("failed to query all backorders: %w", err)
	}
	defer rows.Close()

	backordersMap := make(map[string]float64)
	for rows.Next() {
		var bo model.Backorder
		var qty float64
		if err := rows.Scan(&bo.YjCode, &bo.PackageForm, &bo.JanPackInnerQty, &bo.YjUnitName, &qty); err != nil {
			return nil, err
		}
		// 集計ロジックで使うための文字列キーをここで生成
		key := fmt.Sprintf("%s|%s|%g|%s", bo.YjCode, bo.PackageForm, bo.JanPackInnerQty, bo.YjUnitName)
		backordersMap[key] = qty
	}
	return backordersMap, nil
}

// GetAllBackordersList は全ての発注残をリスト形式で取得します。
func GetAllBackordersList(conn *sql.DB) ([]model.Backorder, error) {
	// ▼▼▼ [修正点] product_masterとJOINして包装情報を取得するクエリに変更 ▼▼▼
	const q = `
		SELECT
			b.yj_code, b.package_form, b.jan_pack_inner_qty, b.yj_unit_name,
			b.order_date, b.yj_quantity, b.product_name,
			pm.yj_pack_unit_qty, pm.jan_pack_unit_qty, pm.jan_unit_code
		FROM backorders AS b
		LEFT JOIN product_master AS pm ON b.yj_code = pm.yj_code
		GROUP BY b.yj_code, b.package_form, b.jan_pack_inner_qty, b.yj_unit_name
		ORDER BY b.order_date, b.product_name
	`
	rows, err := conn.Query(q)
	// ▲▲▲ 修正ここまで ▲▲▲
	if err != nil {
		return nil, fmt.Errorf("failed to query all backorders list: %w", err)
	}
	defer rows.Close()

	var backorders []model.Backorder
	for rows.Next() {
		var bo model.Backorder
		// ▼▼▼ [修正点] 追加したフィールドをスキャン対象に含める ▼▼▼
		if err := rows.Scan(
			&bo.YjCode, &bo.PackageForm, &bo.JanPackInnerQty, &bo.YjUnitName,
			&bo.OrderDate, &bo.YjQuantity, &bo.ProductName,
			&bo.YjPackUnitQty, &bo.JanPackUnitQty, &bo.JanUnitCode,
		); err != nil {
			// ▲▲▲ 修正ここまで ▲▲▲
			return nil, err
		}
		backorders = append(backorders, bo)
	}
	return backorders, nil
}

// ▼▼▼ [修正点] 以下の関数をファイル末尾に追加 ▼▼▼
// DeleteBackorderInTx は指定されたキーの発注残レコードを削除します。
func DeleteBackorderInTx(tx *sql.Tx, backorder model.Backorder) error {
	const q = `DELETE FROM backorders 
				WHERE yj_code = ? AND package_form = ? AND jan_pack_inner_qty = ? AND yj_unit_name = ?`

	res, err := tx.Exec(q, backorder.YjCode, backorder.PackageForm, backorder.JanPackInnerQty, backorder.YjUnitName)
	if err != nil {
		return fmt.Errorf("failed to delete backorder for yj %s: %w", backorder.YjCode, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for backorder yj %s: %w", backorder.YjCode, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no backorder found to delete for yj %s with specified package", backorder.YjCode)
	}
	return nil
}

// ▲▲▲ 修正ここまで ▲▲▲