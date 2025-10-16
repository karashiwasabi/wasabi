// C:\Users\wasab\OneDrive\デスクトップ\WASABI\mappers\view.go

package mappers

import (
	"database/sql"
	"fmt"
	"wasabi/model"
	"wasabi/units"
)

// ToProductMasterView は、*model.ProductMaster を画面表示用の model.ProductMasterView に変換します。
// 包装仕様文字列の生成など、表示に必要な計算処理を集約します。
func ToProductMasterView(master *model.ProductMaster) model.ProductMasterView {
	if master == nil {
		return model.ProductMasterView{}
	}

	// 包装仕様の文字列を生成するために一時的な構造体にデータを詰め替える
	tempJcshms := model.JCShms{
		JC037: master.PackageForm,
		JC039: master.YjUnitName,
		JC044: master.YjPackUnitQty,
		JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
		JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
		JA007: sql.NullString{String: fmt.Sprintf("%d", master.JanUnitCode), Valid: true},
	}

	// JAN単位名を解決する
	var janUnitName string
	if master.JanUnitCode == 0 {
		janUnitName = master.YjUnitName
	} else {
		janUnitName = units.ResolveName(fmt.Sprintf("%d", master.JanUnitCode))
	}

	return model.ProductMasterView{
		ProductMaster:        *master,
		FormattedPackageSpec: units.FormatPackageSpec(&tempJcshms),
		JanUnitName:          janUnitName,
	}
}
