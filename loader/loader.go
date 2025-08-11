package loader

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Defines which columns in the master CSVs should be treated as numeric types.
var tableSchemas = map[string]map[int]string{
	"jcshms": {
		44: "real", // JC044
		50: "real", // JC050 (NHI Price)
		61: "int", 62: "int", 63: "int", 64: "int", 65: "int", 66: "int",
	},
	// ▼▼▼ 修正箇所 ▼▼▼
	// 正しい列番号 (JA006は7列目、JA008は9列目) に修正
	"jancode": {
		7: "real", // JA006
		9: "real", // JA008
	},
	// ▲▲▲ 修正箇所 ▲▲▲
}

// InitDatabase creates the schema and loads master data from CSV files.
func InitDatabase(db *sql.DB) error {
	if err := applySchema(db); err != nil {
		return fmt.Errorf("failed to apply schema.sql: %w", err)
	}
	// JANCODE.CSVはヘッダーがあるため、スキップするように修正
	if err := loadCSV(db, "SOU/JCSHMS.CSV", "jcshms", 125, false); err != nil {
		return fmt.Errorf("failed to load JCSHMS.CSV: %w", err)
	}
	if err := loadCSV(db, "SOU/JANCODE.CSV", "jancode", 30, true); err != nil {
		return fmt.Errorf("failed to load JANCODE.CSV: %w", err)
	}
	return nil
}

func applySchema(db *sql.DB) error {
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = db.Exec(string(schema))
	return err
}

// ヘッダーをスキップするための bool 型引数 `skipHeader` を追加
func loadCSV(db *sql.DB, filepath, tablename string, columns int, skipHeader bool) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(transform.NewReader(f, japanese.ShiftJIS.NewDecoder()))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	// skipHeaderがtrueの場合、ファイルの最初の行を読み飛ばす
	if skipHeader {
		if _, err := r.Read(); err != nil && err != io.EOF {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	placeholders := strings.Repeat("?,", columns-1) + "?"
	stmt, err := tx.Prepare(fmt.Sprintf("INSERT OR REPLACE INTO %s VALUES (%s)", tablename, placeholders))
	if err != nil {
		return err
	}
	defer stmt.Close()

	schema := tableSchemas[tablename]

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < columns {
			continue
		}

		args := make([]interface{}, columns)
		for i, val := range row[:columns] {
			// キーは1から始まる列番号なので i+1
			if colType, ok := schema[i+1]; ok {
				trimmedVal := strings.TrimSpace(val)
				switch colType {
				case "real":
					num, _ := strconv.ParseFloat(trimmedVal, 64)
					args[i] = num
				case "int":
					num, _ := strconv.ParseInt(trimmedVal, 10, 64)
					args[i] = num
				}
			} else {
				args[i] = val
			}
		}

		if _, err := stmt.Exec(args...); err != nil {
			continue
		}
	}
	return tx.Commit()
}
