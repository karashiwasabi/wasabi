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
	ar.KanaName = master.KanaName
	ar.UsageClassification = master.UsageClassification
	ar.PackageForm = master.PackageForm
	ar.MakerName = master.MakerName
	ar.JanPackInnerQty = master.JanPackInnerQty
	ar.JanPackUnitQty = master.JanPackUnitQty
	ar.YjPackUnitQty = master.YjPackUnitQty
	ar.PurchasePrice = master.PurchasePrice
	ar.SupplierWholesale = master.SupplierWholesale
	ar.FlagPoison = master.FlagPoison
	ar.FlagDeleterious = master.FlagDeleterious
	ar.FlagNarcotic = master.FlagNarcotic
	ar.FlagPsychotropic = master.FlagPsychotropic
	ar.FlagStimulant = master.FlagStimulant
	ar.FlagStimulantRaw = master.FlagStimulantRaw

	// ▼▼▼ [修正点] YJ単位名とJAN単位名の設定ロジックをここに集約・修正 ▼▼▼
	ar.YjUnitName = units.ResolveName(master.YjUnitName)
	ar.JanUnitCode = strconv.Itoa(master.JanUnitCode)

	// ご指摘のロジックを実装
	if master.JanUnitCode == 0 {
		ar.JanUnitName = ar.YjUnitName // 0の場合はYJ単位名を引き継ぐ
	} else {
		ar.JanUnitName = units.ResolveName(ar.JanUnitCode) // 0以外はコードを名称に変換
	}
	// ▲▲▲ 修正ここまで ▲▲▲

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
}
