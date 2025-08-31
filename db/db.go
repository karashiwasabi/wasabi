// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\db.go

package db

import "database/sql"

// DBTXは、*sql.DB（データベース接続プール）と*sql.Tx（トランザクション）の両方が
// 満たすことができるインターフェースです。
//
// このインターフェースを関数の引数として使用することで、同じデータベース操作のロジックを
// トランザクションの内外で再利用できます。これにより、コードの重複が削減され、
// メンテナンス性が向上します。
type DBTX interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}
