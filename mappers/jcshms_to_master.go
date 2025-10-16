// C:\Users\wasab\OneDrive\デスクトップ\WASABI\mappers\jcshms_to_master.go
package mappers

import (
	"strconv"
	"strings"
	"wasabi/model"
)

// JcshmsToProductMasterInput はJCSHMSのレコードをProductMasterInputに変換します。
func JcshmsToProductMasterInput(jcshms *model.JCShms, janCode string) model.ProductMasterInput {
	// ▼▼▼【ここから修正】薬価を単価に変換するロジックを修正 ▼▼▼
	var unitNhiPrice float64
	// JC044(YJ包装単位薬価数量)が0より大きい場合のみ計算
	if jcshms.JC044 > 0 {
		// JC050(包装薬価) / JC044(YJ包装単位薬価数量) でYJ単位あたりの単価を算出
		unitNhiPrice = jcshms.JC050 / jcshms.JC044
	}
	// ▲▲▲【修正ここまで】▲▲▲

	return model.ProductMasterInput{
		ProductCode:         janCode,
		YjCode:              jcshms.JC009,
		Gs1Code:             jcshms.JC122,
		ProductName:         strings.TrimSpace(jcshms.JC018),
		KanaName:            strings.TrimSpace(jcshms.JC022),
		MakerName:           strings.TrimSpace(jcshms.JC030),
		Specification:       strings.TrimSpace(jcshms.JC020),
		UsageClassification: strings.TrimSpace(jcshms.JC013),
		PackageForm:         strings.TrimSpace(jcshms.JC037),
		YjUnitName:          strings.TrimSpace(jcshms.JC039),
		YjPackUnitQty:       jcshms.JC044,
		JanPackInnerQty:     jcshms.JA006.Float64,
		JanUnitCode:         parseInt(jcshms.JA007.String),
		JanPackUnitQty:      jcshms.JA008.Float64,
		Origin:              "JCSHMS",
		NhiPrice:            unitNhiPrice, // 修正した単価をセット
		FlagPoison:          jcshms.JC061,
		FlagDeleterious:     jcshms.JC062,
		FlagNarcotic:        jcshms.JC063,
		FlagPsychotropic:    jcshms.JC064,
		FlagStimulant:       jcshms.JC065,
		FlagStimulantRaw:    jcshms.JC066,
		IsOrderStopped:      0,
	}
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}
