// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\backorders.go

package db

import (
	"database/sql"
	"fmt"
	"wasabi/model"
)

// ▼▼▼【ここから修正】▼▼▼

/**
 * @brief 複数の発注残レコードをトランザクション内で登録します（INSERT）。
 * @param tx SQLトランザクションオブジェクト
 * @param backorders 登録する発注残レコードのスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 発注ごとに新しいレコードとしてデータベースに挿入します。
 */
func InsertBackordersInTx(tx *sql.Tx, backorders []model.Backorder) error {
	const q = `
		INSERT INTO backorders (
			order_date, yj_code, product_name, package_form, jan_pack_inner_qty, 
			yj_unit_name, order_quantity, remaining_quantity, wholesaler_code,
			yj_pack_unit_qty, jan_pack_unit_qty, jan_unit_code
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to prepare backorder insert statement: %w", err)
	}
	defer stmt.Close()

	for _, bo := range backorders {
		_, err := stmt.Exec(
			bo.OrderDate, bo.YjCode, bo.ProductName, bo.PackageForm, bo.JanPackInnerQty,
			bo.YjUnitName, bo.OrderQuantity, bo.RemainingQuantity, bo.WholesalerCode,
			bo.YjPackUnitQty, bo.JanPackUnitQty, bo.JanUnitCode,
		)
		if err != nil {
			return fmt.Errorf("failed to execute backorder insert for yj %s: %w", bo.YjCode, err)
		}
	}
	return nil
}

/**
 * @brief 納品データに基づいて発注残を消し込みます（FIFO: 先入れ先出し）。
 * @param conn データベース接続
 * @param deliveredItems 納品された品物のスライス
 * @return error 処理中にエラーが発生した場合
 * @details
 * 納品された各品物について、対応する発注残を古いものから順に消し込みます。
 * 発注残数量が0になったレコードは削除されます。
 */
func ReconcileBackorders(conn *sql.DB, deliveredItems []model.Backorder) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for reconciliation: %w", err)
	}
	defer tx.Rollback()

	for _, item := range deliveredItems {
		deliveryQty := item.YjQuantity

		rows, err := tx.Query(`
			SELECT id, remaining_quantity FROM backorders 
			WHERE yj_code = ? AND package_form = ? AND jan_pack_inner_qty = ? AND yj_unit_name = ?
			ORDER BY order_date, id`,
			item.YjCode, item.PackageForm, item.JanPackInnerQty, item.YjUnitName,
		)
		if err != nil {
			return fmt.Errorf("failed to query backorders for reconciliation: %w", err)
		}

		for rows.Next() {
			if deliveryQty <= 0 {
				break
			}
			var id int
			var remainingQty float64
			if err := rows.Scan(&id, &remainingQty); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan backorder row: %w", err)
			}

			if deliveryQty >= remainingQty {
				// 納品数で発注残が完全にカバーされる場合
				if _, err := tx.Exec(`DELETE FROM backorders WHERE id = ?`, id); err != nil {
					rows.Close()
					return fmt.Errorf("failed to delete reconciled backorder id %d: %w", id, err)
				}
				deliveryQty -= remainingQty
			} else {
				// 納品数の一部で発注残を減らす場合
				newRemaining := remainingQty - deliveryQty
				if _, err := tx.Exec(`UPDATE backorders SET remaining_quantity = ? WHERE id = ?`, newRemaining, id); err != nil {
					rows.Close()
					return fmt.Errorf("failed to update partially reconciled backorder id %d: %w", id, err)
				}
				deliveryQty = 0
			}
		}
		rows.Close()
	}
	return tx.Commit()
}

/**
 * @brief 全ての発注残を、集計で高速に参照できるマップ形式で取得します。
 * @param conn データベース接続
 * @return map[string]float64 包装ごとのキーを文字列にしたマップ
 * @return error 処理中にエラーが発生した場合
 * @details
 * キーは "YJコード|包装形態|内包装数量|YJ単位名" の形式で生成されます。
 * 在庫元帳の計算（GetStockLedger）で使われます。
 */
func GetAllBackordersMap(conn *sql.DB) (map[string]float64, error) {
	const q = `
		SELECT yj_code, package_form, jan_pack_inner_qty, yj_unit_name, SUM(remaining_quantity)
		FROM backorders
		GROUP BY yj_code, package_form, jan_pack_inner_qty, yj_unit_name`
	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query all backorders map: %w", err)
	}
	defer rows.Close()

	backordersMap := make(map[string]float64)
	for rows.Next() {
		var yjCode, packageForm, yjUnitName string
		var janPackInnerQty, totalRemaining float64
		if err := rows.Scan(&yjCode, &packageForm, &janPackInnerQty, &yjUnitName, &totalRemaining); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s|%s|%g|%s", yjCode, packageForm, janPackInnerQty, yjUnitName)
		backordersMap[key] = totalRemaining
	}
	return backordersMap, nil
}

/**
 * @brief 全ての発注残を画面表示用のリスト形式で取得します。
 * @param conn データベース接続
 * @return []model.Backorder 発注残レコードのスライス
 * @return error 処理中にエラーが発生した場合
 */
func GetAllBackordersList(conn *sql.DB) ([]model.Backorder, error) {
	const q = `
		SELECT
			id, order_date, yj_code, product_name, package_form, jan_pack_inner_qty, 
			yj_unit_name, order_quantity, remaining_quantity, wholesaler_code,
			yj_pack_unit_qty, jan_pack_unit_qty, jan_unit_code
		FROM backorders
		ORDER BY order_date, product_name, id
	`
	rows, err := conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query all backorders list: %w", err)
	}
	defer rows.Close()

	var backorders []model.Backorder
	for rows.Next() {
		var bo model.Backorder
		if err := rows.Scan(
			&bo.ID, &bo.OrderDate, &bo.YjCode, &bo.ProductName, &bo.PackageForm, &bo.JanPackInnerQty,
			&bo.YjUnitName, &bo.OrderQuantity, &bo.RemainingQuantity, &bo.WholesalerCode,
			&bo.YjPackUnitQty, &bo.JanPackUnitQty, &bo.JanUnitCode,
		); err != nil {
			return nil, err
		}
		backorders = append(backorders, bo)
	}
	return backorders, nil
}

/**
 * @brief 指定されたIDの発注残レコードをトランザクション内で削除します。
 * @param tx SQLトランザクションオブジェクト
 * @param id 削除対象のID
 * @return error 処理中にエラーが発生した場合
 */
func DeleteBackorderInTx(tx *sql.Tx, id int) error {
	const q = `DELETE FROM backorders WHERE id = ?`

	res, err := tx.Exec(q, id)
	if err != nil {
		return fmt.Errorf("failed to delete backorder for id %d: %w", id, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for backorder id %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no backorder found to delete for id %d", id)
	}
	return nil
}

// ▲▲▲【修正ここまで】▲▲▲
