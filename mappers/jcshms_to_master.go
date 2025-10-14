// C:\Users\wasab\OneDrive\デスクトップ\WASABI\mappers\jcshms_to_master.go
package mappers

import (
	"strconv"
	"strings"
	"wasabi/model"
)

// JcshmsToProductMasterInput はJCSHMSのレコードをProductMasterInputに変換します。
func JcshmsToProductMasterInput(jcshms *model.JCShms, janCode string) model.ProductMasterInput {
	return model.ProductMasterInput{
		ProductCode: janCode,
		YjCode:      jcshms.JC009,
		// ▼▼▼【ここが修正箇所】▼▼▼
		Gs1Code: jcshms.JC122,
		// ▲▲▲【修正ここまで】▲▲▲
		ProductName:         strings.TrimSpace(jcshms.JC018), // JC018: 商品名
		KanaName:            strings.TrimSpace(jcshms.JC022), // JC022: 商品名カナ
		MakerName:           strings.TrimSpace(jcshms.JC030),
		Specification:       strings.TrimSpace(jcshms.JC020), // JC020: 規格容量
		UsageClassification: strings.TrimSpace(jcshms.JC013),
		PackageForm:         strings.TrimSpace(jcshms.JC037),
		YjUnitName:          strings.TrimSpace(jcshms.JC039),
		YjPackUnitQty:       jcshms.JC044,
		JanPackInnerQty:     jcshms.JA006.Float64,
		JanUnitCode:         parseInt(jcshms.JA007.String),
		JanPackUnitQty:      jcshms.JA008.Float64,
		Origin:              "JCSHMS",
		NhiPrice:            jcshms.JC050,
		FlagPoison:          jcshms.JC061,
		FlagDeleterious:     jcshms.JC062,
		FlagNarcotic:        jcshms.JC063,
		FlagPsychotropic:    jcshms.JC064,
		FlagStimulant:       jcshms.JC065,
		FlagStimulantRaw:    jcshms.JC066,
		IsOrderStopped:      0, // デフォルトは発注可
	}
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}
