package parsers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// ParsedInventoryDataはファイル全体の構造体です
type ParsedInventoryData struct {
	Date    string
	Records []model.UnifiedInputRecord
}

// trimQuotesは文字列から空白とシングルクォートを除去します
func trimQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), "'")
}

// ParseInventoryFileは棚卸ファイルを解析し、UnifiedInputRecordのスライスを返します
func ParseInventoryFile(r io.Reader) (*ParsedInventoryData, error) {
	decoder := japanese.ShiftJIS.NewDecoder()
	reader := csv.NewReader(transform.NewReader(r, decoder))
	reader.FieldsPerRecord = -1

	var result ParsedInventoryData
	var dataRecords []model.UnifiedInputRecord

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv read all error: %w", err)
	}

	for _, row := range records {
		if len(row) == 0 {
			continue
		}

		rowType := strings.TrimSpace(row[0])
		switch rowType {
		case "H":
			if len(row) > 4 {
				result.Date = trimQuotes(row[4])
			}
		case "R1":
			if len(row) > 45 {
				innerPackQty, _ := strconv.ParseFloat(strings.TrimSpace(row[17]), 64)
				physicalJanQty, _ := strconv.ParseFloat(strings.TrimSpace(row[21]), 64)

				dataRecords = append(dataRecords, model.UnifiedInputRecord{
					ProductName:     trimQuotes(row[12]),
					YjUnitName:      trimQuotes(row[16]),
					JanPackInnerQty: innerPackQty,   // 18列目を格納
					JanQuantity:     physicalJanQty, // 22列目を格納
					YjCode:          trimQuotes(row[42]),
					JanCode:         trimQuotes(row[45]),
				})
			}
		}
	}
	result.Records = dataRecords
	return &result, nil
}
