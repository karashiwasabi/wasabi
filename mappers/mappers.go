package mappers

import (
	"database/sql"
	"strconv"
	"wasabi/model"
	"wasabi/units"
)

// MapProductMasterToTransaction は、ProductMaster構造体の情報からTransactionRecord構造体へ必要なデータをコピーします。
func MapProductMasterToTransaction(ar *model.TransactionRecord, master *model.ProductMaster) {
	// 取引記録に単価が設定されていない場合、マスターの薬価をデフォルト値として使用します。
	if ar.UnitPrice == 0 {
		ar.UnitPrice = master.NhiPrice
	}

	// product_master から transaction_records へ共通して存在する情報をコピーします。
	ar.JanCode = master.ProductCode
	ar.YjCode = master.YjCode
	// WASABIではproduct_nameに規格が含まれることが期待されるため、tkrのロジックを応用します。
	if master.Specification != "" {
		ar.ProductName = master.ProductName + " " + master.Specification
	} else {
		ar.ProductName = master.ProductName
	}
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

	// 単位名の解決ロジック (tkrのロジックを流用)
	ar.YjUnitName = units.ResolveName(master.YjUnitName)
	ar.JanUnitCode = strconv.Itoa(master.JanUnitCode)
	if master.JanUnitCode == 0 {
		ar.JanUnitName = ar.YjUnitName
	} else {
		ar.JanUnitName = units.ResolveName(ar.JanUnitCode)
	}

	// 包装仕様文字列の生成ロジック (tkrのロジックを流用)
	// この処理のために、一時的にJCShms構造体の形式にデータを当てはめます。
	tempJcshms := model.JCShms{
		JC037: master.PackageForm,
		JC039: master.YjUnitName,
		JC044: master.YjPackUnitQty,
		JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
		JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
		JA007: sql.NullString{String: strconv.Itoa(master.JanUnitCode), Valid: true},
	}
	ar.PackageSpec = units.FormatPackageSpec(&tempJcshms)
}
