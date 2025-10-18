// C:\Users\wasab\OneDrive\デスクトップ\WASABI\loader\loader.go
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

// ▼▼▼【ここから修正】▼▼▼
var tableSchemas = map[string]map[int]string{
	"jcshms": {
		45:  "real", // JC044
		50:  "real", // JC049
		51:  "real", // JC050
		62:  "int",  // JC061
		63:  "int",  // JC062
		64:  "int",  // JC063
		65:  "int",  // JC064
		66:  "int",  // JC065
		67:  "int",  // JC066
		125: "real", // JC124
	},
	"jancode": {
		7: "real",
		9: "real",
	},
}

// ▲▲▲【修正ここまで】▲▲▲

// InitDatabase creates the schema and loads master data from CSV files.
func InitDatabase(db *sql.DB) error {
	if err := applySchema(db); err != nil {
		return fmt.Errorf("failed to apply schema.sql: %w", err)
	}
	if err := LoadCSV(db, "SOU/JCSHMS.CSV", "jcshms", 125, false); err != nil {
		return fmt.Errorf("failed to load JCSHMS.CSV: %w", err)
	}
	if err := LoadCSV(db, "SOU/JANCODE.CSV", "jancode", 30, true); err != nil {
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

func LoadCSV(db *sql.DB, filepath, tablename string, columns int, skipHeader bool) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(transform.NewReader(f, japanese.ShiftJIS.NewDecoder()))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

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
		for i := 0; i < columns; i++ {
			val := row[i]
			// CSVの列番号は0から始まるので、スキーマのキー(1から始まる)に合わせるために+1する
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
