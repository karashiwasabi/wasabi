package db

import "database/sql"

// DBTXは、*sql.DBと*sql.Txの両方が満たすことができるインターフェースです。
// これにより、同じ関数をトランザクション内でも外でも実行できます。
type DBTX interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}