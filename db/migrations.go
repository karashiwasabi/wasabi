// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\migrations.go

package db

import (
	"database/sql"
	"fmt"
	"log"
)

/**
 * @brief データベースのマイグレーション（スキーマの更新）を適用します。
 * @param conn データベース接続
 * @return error 処理中にエラーが発生した場合
 * @details
 * アプリケーションの起動時に呼び出され、不足しているインデックスなどを追加します。
 * 各SQL文は `IF NOT EXISTS` を使用しているため、何度実行しても安全です。
 */
func ApplyMigrations(conn *sql.DB) error {
	migrations := []string{
		// パフォーマンス改善のためのインデックス
		`CREATE INDEX IF NOT EXISTS idx_transactions_receipt_number ON transaction_records (receipt_number);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_process_flag_ma ON transaction_records (process_flag_ma);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_flag_date ON transaction_records (flag, transaction_date);`,
	}

	log.Println("Applying database migrations...")
	for _, migration := range migrations {
		if _, err := conn.Exec(migration); err != nil {
			return fmt.Errorf("failed to apply migration (%s): %w", migration, err)
		}
	}
	log.Println("Database migrations applied successfully.")
	return nil
}
