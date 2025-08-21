// C:\Dev\WASABI\mappers\mappers.go

package mappers

import (
	"database/sql"
	"strconv"
	"wasabi/model"
	"wasabi/units"
)

func MapProductMasterToTransaction(ar *model.TransactionRecord, master *model.ProductMaster) {
	if ar.UnitPrice == 0 {
		ar.UnitPrice = master.NhiPrice
	}

	ar.JanCode = master.ProductCode

	ar.YjCode = master.YjCode
	ar.ProductName = master.ProductName
	// (中略: 他のフィールドのマッピング)
	// ...
	ar.YjUnitName = units.ResolveName(master.YjUnitName)
	ar.JanUnitCode = strconv.Itoa(master.JanUnitCode)

	// ▼▼▼ ここからが修正の核心部分です ▼▼▼
	// FormatPackageSpecに必要なデータを全て持つ一時的なJCShms構造体を作成します。
	tempJcshms := model.JCShms{
		JC037: master.PackageForm,
		JC039: master.YjUnitName,
		JC044: master.YjPackUnitQty,
		JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
		JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
		JA007: sql.NullString{String: strconv.Itoa(master.JanUnitCode), Valid: true},
	}
	// 完全にデータが揃った構造体を渡して、包装仕様を生成します。
	ar.PackageSpec = units.FormatPackageSpec(&tempJcshms)
	// ▲▲▲ 修正ここまで ▲▲▲
}
