// C:\Users\wasab\OneDrive\デスクトップ\WASABI\db\jcshms.go

package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"wasabi/model"
)

/**
 * @brief 複数のJANコードに対応するJCSHMSおよびJANCODEマスター情報を一括で取得し、マップ形式で返します。
 * @param tx SQLトランザクションオブジェクト
 * @param jans 検索対象のJANコードのスライス
 * @return map[string]*model.JCShms JANコードをキーとしたJCSHMS情報のマップ
 * @return error 処理中にエラーが発生した場合
 * @details
 * N+1問題を避けるため、IN句を用いてjcshmsテーブルとjancodeテーブルにそれぞれ1回ずつクエリを発行します。
 * これにより、多数のJANコードに対するマスター情報を効率的に取得できます。
 * DATファイルの処理などで、未知のJANコードの詳細情報をまとめて取得する際に使用されます。
 */
func GetJcshmsByCodesMap(tx *sql.Tx, jans []string) (map[string]*model.JCShms, error) {
	if len(jans) == 0 {
		return make(map[string]*model.JCShms), nil
	}

	results := make(map[string]*model.JCShms)
	args := make([]interface{}, len(jans))
	for i, jan := range jans {
		args[i] = jan
		// 後続の処理でnilポインタエラーが発生しないように、マップのエントリを初期化
		results[jan] = &model.JCShms{}
	}

	inClause := `(?` + strings.Repeat(",?", len(jans)-1) + `)`

	// jcshmsテーブルへのクエリ
	q1 := `SELECT JC000, JC009, JC013, JC018, JC022, JC030, JC037, JC039, JC044, JC050,
	              JC061, JC062, JC063, JC064, JC065, JC066
	       FROM jcshms WHERE JC000 IN ` + inClause
	rows1, err := tx.Query(q1, args...)
	if err != nil {
		return nil, fmt.Errorf("jcshms bulk search failed: %w", err)
	}
	defer rows1.Close()

	for rows1.Next() {
		var jan string
		var jcshmsPart model.JCShms
		var jc050 sql.NullString

		if err := rows1.Scan(&jan, &jcshmsPart.JC009, &jcshmsPart.JC013, &jcshmsPart.JC018, &jcshmsPart.JC022, &jcshmsPart.JC030,
			&jcshmsPart.JC037, &jcshmsPart.JC039, &jcshmsPart.JC044, &jc050,
			&jcshmsPart.JC061, &jcshmsPart.JC062, &jcshmsPart.JC063, &jcshmsPart.JC064, &jcshmsPart.JC065, &jcshmsPart.JC066,
		); err != nil {
			return nil, err
		}

		res := results[jan]
		res.JC009, res.JC013, res.JC018, res.JC022 = jcshmsPart.JC009, jcshmsPart.JC013, jcshmsPart.JC018, jcshmsPart.JC022
		res.JC030, res.JC037, res.JC039 = jcshmsPart.JC030, jcshmsPart.JC037, jcshmsPart.JC039
		res.JC044 = jcshmsPart.JC044
		res.JC061, res.JC062, res.JC063, res.JC064, res.JC065, res.JC066 = jcshmsPart.JC061, jcshmsPart.JC062, jcshmsPart.JC063, jcshmsPart.JC064, jcshmsPart.JC065, jcshmsPart.JC066

		val, err := strconv.ParseFloat(jc050.String, 64)
		if err != nil {
			res.JC050 = 0
		} else {
			res.JC050 = val
		}
	}

	// jancodeテーブルへのクエリ
	q2 := `SELECT JA001, JA006, JA007, JA008 FROM jancode WHERE JA001 IN ` + inClause
	rows2, err := tx.Query(q2, args...)
	if err != nil {
		return nil, fmt.Errorf("jancode bulk search failed: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var jan string
		var jaPart struct {
			JA006 sql.NullFloat64
			JA007 sql.NullString
			JA008 sql.NullFloat64
		}
		if err := rows2.Scan(&jan, &jaPart.JA006, &jaPart.JA007, &jaPart.JA008); err != nil {
			return nil, err
		}
		results[jan].JA006 = jaPart.JA006
		results[jan].JA007 = jaPart.JA007
		results[jan].JA008 = jaPart.JA008
	}

	return results, nil
}
