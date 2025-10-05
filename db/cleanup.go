// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\cleanup.go

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"wasabi/model"
)

// GetCleanupCandidates は整理対象となる製品マスターのリストを取得します。
func GetCleanupCandidates(conn *sql.DB) ([]*model.ProductMaster, error) {
	// 1. 全製品の現在の理論在庫を取得
	stockMap, err := GetAllCurrentStockMap(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get all current stock: %w", err)
	}

	// 在庫がゼロの製品コードをリストアップ
	var zeroStockProductCodes []string
	allProductCodes, err := getAllProductCodes(conn)
	if err != nil {
		return nil, err
	}
	for _, pc := range allProductCodes {
		if stock, ok := stockMap[pc]; !ok || stock == 0 {
			zeroStockProductCodes = append(zeroStockProductCodes, pc)
		}
	}

	if len(zeroStockProductCodes) == 0 {
		return []*model.ProductMaster{}, nil
	}

	// 2. 在庫ゼロの製品について、過去3ヶ月の取引履歴を確認
	cutoffDate := time.Now().AddDate(0, -3, 0).Format("20060102")

	placeholders := strings.Repeat("?,", len(zeroStockProductCodes)-1) + "?"
	query := fmt.Sprintf(`
		SELECT DISTINCT jan_code FROM transaction_records
		WHERE flag IN (1, 2, 3, 11, 12)
		AND transaction_date >= ?
		AND jan_code IN (%s)
	`, placeholders)

	args := make([]interface{}, 0, len(zeroStockProductCodes)+1)
	args = append(args, cutoffDate)
	for _, pc := range zeroStockProductCodes {
		args = append(args, pc)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent transactions: %w", err)
	}
	defer rows.Close()

	// 期間内に動きがあった製品をマップに記録
	movedProducts := make(map[string]bool)
	for rows.Next() {
		var productCode string
		if err := rows.Scan(&productCode); err != nil {
			return nil, err
		}
		movedProducts[productCode] = true
	}

	// 3. 動きがなかった製品コードのみを抽出
	var finalCandidateCodes []string
	for _, pc := range zeroStockProductCodes {
		if !movedProducts[pc] {
			finalCandidateCodes = append(finalCandidateCodes, pc)
		}
	}

	if len(finalCandidateCodes) == 0 {
		return []*model.ProductMaster{}, nil
	}

	// 4. 最終候補のマスター情報を取得して返す
	mastersMap, err := GetProductMastersByCodesMap(conn, finalCandidateCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get final candidate masters: %w", err)
	}

	var result []*model.ProductMaster
	for _, code := range finalCandidateCodes {
		if master, ok := mastersMap[code]; ok {
			result = append(result, master)
		}
	}
	return result, nil
}

// DeleteMastersByCodesInTx は指定された製品コードのマスターを削除します。
func DeleteMastersByCodesInTx(tx *sql.Tx, productCodes []string) (int64, error) {
	if len(productCodes) == 0 {
		return 0, nil
	}
	placeholders := strings.Repeat("?,", len(productCodes)-1) + "?"
	query := fmt.Sprintf("DELETE FROM product_master WHERE product_code IN (%s)", placeholders)

	args := make([]interface{}, len(productCodes))
	for i, code := range productCodes {
		args[i] = code
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete masters: %w", err)
	}
	return res.RowsAffected()
}

// getAllProductCodes は product_master から全ての製品コードを取得するヘルパー関数です。
func getAllProductCodes(conn *sql.DB) ([]string, error) {
	rows, err := conn.Query("SELECT product_code FROM product_master")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}
