// C:\Users\wasab\OneDrive\デスクトップ\WASABI\units\units.go

package units

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"wasabi/model"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

var internalMap map[string]string
var reverseMap map[string]string

// FormatPackageSpecは、JCSHMSのデータから仕様通りの包装文字列を生成します。
func FormatPackageSpec(jcshms *model.JCShms) string {
	if jcshms == nil {
		return ""
	}

	yjUnitName := ResolveName(jcshms.JC039)
	pkg := fmt.Sprintf("%s %g%s", jcshms.JC037, jcshms.JC044, yjUnitName)

	if jcshms.JA006.Valid && jcshms.JA008.Valid && jcshms.JA008.Float64 != 0 {
		resolveJanUnitName := func(code string) string {
			if code != "0" && code != "" {
				return ResolveName(code)
			}
			return "" // 0か空の場合は単位を省略
		}

		janUnitName := resolveJanUnitName(jcshms.JA007.String)

		pkg += fmt.Sprintf(" (%g%s×%g%s)",
			jcshms.JA006.Float64,
			yjUnitName,
			jcshms.JA008.Float64,
			janUnitName,
		)
	}
	return pkg
}

// ▼▼▼ [修正点] 「簡易包装」を生成する新しい関数を末尾に追加 ▼▼▼
// FormatSimplePackageSpec は、「包装形態 + 内包装数量 + YJ単位名」の簡易的な包装文字列を生成します。
func FormatSimplePackageSpec(jcshms *model.JCShms) string {
	if jcshms == nil {
		return ""
	}

	// 内包装数量が存在し、0より大きい場合のみ文字列を組み立てる
	if jcshms.JA006.Valid && jcshms.JA006.Float64 > 0 {
		yjUnitName := ResolveName(jcshms.JC039)
		return fmt.Sprintf("%s %g%s", jcshms.JC037, jcshms.JA006.Float64, yjUnitName)
	}

	// 条件に合わない場合は、包装形態のみを返すなどのフォールバック
	// ここでは、より情報量が多い詳細包装を返す
	return FormatPackageSpec(jcshms)
}

// ▲▲▲ 修正ここまで ▲▲▲

// (LoadTANIFile, ResolveName, ResolveCode functions are unchanged)
func LoadTANIFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("LoadTANIFile: open %s: %w", path, err)
	}
	defer file.Close()

	decoder := japanese.ShiftJIS.NewDecoder()
	reader := csv.NewReader(transform.NewReader(file, decoder))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	m := make(map[string]string)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("LoadTANIFile: read %s: %w", path, err)
		}
		if len(record) < 2 {
			continue
		}
		code := record[0]
		name := record[1]
		m[code] = name
	}
	internalMap = m

	reverseMap = make(map[string]string)
	for code, name := range internalMap {
		reverseMap[name] = code
	}

	return m, nil
}

func ResolveName(code string) string {
	if internalMap == nil {
		return code
	}
	if name, ok := internalMap[code]; ok {
		return name
	}
	return code
}

func ResolveCode(name string) string {
	if reverseMap == nil {
		return ""
	}
	if code, ok := reverseMap[name]; ok {
		return code
	}
	return ""
}
