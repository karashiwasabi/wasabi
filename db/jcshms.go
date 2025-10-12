package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"wasabi/model" //
)

/**
 * @brief 複数のJANコードに対応するJCSHMSおよびJANCODEマスター情報を一括で取得し、マップ形式で返します。
 * @param tx SQLトランザクションオブジェクト
 * @param jans 検索対象のJANコードのスライス
 * @return map[string]*model.JCShms JANコードをキーとしたJCSHMS情報のマップ
 * @return error 処理中にエラーが発生した場合
 */
func GetJcshmsByCodesMap(tx *sql.Tx, jans []string) (map[string]*model.JCShms, error) {
	if len(jans) == 0 {
		return make(map[string]*model.JCShms), nil
	}

	results := make(map[string]*model.JCShms)
	args := make([]interface{}, len(jans))
	for i, jan := range jans {
		args[i] = jan
		results[jan] = &model.JCShms{}
	}

	inClause := `(?` + strings.Repeat(",?", len(jans)-1) + `)`

	// JCSHMSテーブルへのクエリ (JC020:規格, JC122:GS1コード を追加)
	q1 := `SELECT JC000, JC009, JC013, JC018, JC020, JC022, JC030, JC037, JC039, JC044, JC050,
	              JC061, JC062, JC063, JC064, JC065, JC066, JC122
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

		if err := rows1.Scan(&jan, &jcshmsPart.JC009, &jcshmsPart.JC013, &jcshmsPart.JC018, &jcshmsPart.JC020, &jcshmsPart.JC022, &jcshmsPart.JC030,
			&jcshmsPart.JC037, &jcshmsPart.JC039, &jcshmsPart.JC044, &jc050,
			&jcshmsPart.JC061, &jcshmsPart.JC062, &jcshmsPart.JC063, &jcshmsPart.JC064, &jcshmsPart.JC065, &jcshmsPart.JC066, &jcshmsPart.JC122,
		); err != nil {
			return nil, err
		}

		res := results[jan]
		res.JC009, res.JC013, res.JC018, res.JC020, res.JC022 = jcshmsPart.JC009, jcshmsPart.JC013, jcshmsPart.JC018, jcshmsPart.JC020, jcshmsPart.JC022
		res.JC030, res.JC037, res.JC039 = jcshmsPart.JC030, jcshmsPart.JC037, jcshmsPart.JC039
		res.JC044 = jcshmsPart.JC044
		res.JC061, res.JC062, res.JC063, res.JC064, res.JC065, res.JC066 = jcshmsPart.JC061, jcshmsPart.JC062, jcshmsPart.JC063, jcshmsPart.JC064, jcshmsPart.JC065, jcshmsPart.JC066
		res.JC122 = jcshmsPart.JC122

		val, err := strconv.ParseFloat(jc050.String, 64)
		if err != nil {
			res.JC050 = 0
		} else {
			res.JC050 = val
		}
	}

	// jancodeテーブルへのクエリ (変更なし)
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

/**
 * @brief 単一のJANコードに対応するJCSHMSおよびJANCODEマスター情報を取得します。
 * @param tx SQLトランザクションオブジェクト
 * @param jan 検索対象のJANコード
 * @return *model.JCShms JCSHMS情報
 * @return error 処理中にエラーが発生した場合
 */
func GetJcshmsRecordByJan(tx *sql.Tx, jan string) (*model.JCShms, error) {
	jcshms := &model.JCShms{}
	var jc050 sql.NullString

	// JCSHMSテーブルへのクエリ
	q1 := `SELECT JC009, JC013, JC018, JC020, JC022, JC030, JC037, JC039, JC044, JC050,
				  JC061, JC062, JC063, JC064, JC065, JC066, JC122
		   FROM jcshms WHERE JC000 = ?`
	err := tx.QueryRow(q1, jan).Scan(
		&jcshms.JC009, &jcshms.JC013, &jcshms.JC018, &jcshms.JC020, &jcshms.JC022, &jcshms.JC030,
		&jcshms.JC037, &jcshms.JC039, &jcshms.JC044, &jc050,
		&jcshms.JC061, &jcshms.JC062, &jcshms.JC063, &jcshms.JC064, &jcshms.JC065, &jcshms.JC066, &jcshms.JC122,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("jcshms single search failed for jan %s: %w", jan, err)
	}

	val, err := strconv.ParseFloat(jc050.String, 64)
	if err != nil {
		jcshms.JC050 = 0
	} else {
		jcshms.JC050 = val
	}

	// jancodeテーブルへのクエリ
	q2 := `SELECT JA006, JA007, JA008 FROM jancode WHERE JA001 = ?`
	err = tx.QueryRow(q2, jan).Scan(&jcshms.JA006, &jcshms.JA007, &jcshms.JA008)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("jancode single search failed for jan %s: %w", jan, err)
	}

	return jcshms, nil
}

/**
 * @brief 単一のGS1コードに対応するJCSHMSおよびJANCODEマスター情報を取得します。
 * @param tx SQLトランザクションオブジェクト
 * @param gs1Code 検索対象のGS1コード
 * @return *model.JCShms JCSHMS情報
 * @return string 見つかったJANコード
 * @return error 処理中にエラーが発生した場合
 */
func GetJcshmsRecordByGS1(tx *sql.Tx, gs1Code string) (*model.JCShms, string, error) {
	jcshms := &model.JCShms{}
	var jc050 sql.NullString
	var janCode string

	// JCSHMSテーブルへのクエリ (JC122で検索)
	q1 := `SELECT JC000, JC009, JC013, JC018, JC020, JC022, JC030, JC037, JC039, JC044, JC050,
				  JC061, JC062, JC063, JC064, JC065, JC066, JC122
		   FROM jcshms WHERE JC122 = ?`
	err := tx.QueryRow(q1, gs1Code).Scan(
		&janCode, &jcshms.JC009, &jcshms.JC013, &jcshms.JC018, &jcshms.JC020, &jcshms.JC022, &jcshms.JC030,
		&jcshms.JC037, &jcshms.JC039, &jcshms.JC044, &jc050,
		&jcshms.JC061, &jcshms.JC062, &jcshms.JC063, &jcshms.JC064, &jcshms.JC065, &jcshms.JC066, &jcshms.JC122,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("jcshms single search by gs1 failed for gs1 %s: %w", gs1Code, err)
	}

	val, err := strconv.ParseFloat(jc050.String, 64)
	if err != nil {
		jcshms.JC050 = 0
	} else {
		jcshms.JC050 = val
	}

	// jancodeテーブルへのクエリ (取得したjanCodeを使用)
	q2 := `SELECT JA006, JA007, JA008 FROM jancode WHERE JA001 = ?`
	err = tx.QueryRow(q2, janCode).Scan(&jcshms.JA006, &jcshms.JA007, &jcshms.JA008)
	if err != nil && err != sql.ErrNoRows {
		return nil, "", fmt.Errorf("jancode single search failed for jan %s: %w", janCode, err)
	}

	return jcshms, janCode, nil
}
