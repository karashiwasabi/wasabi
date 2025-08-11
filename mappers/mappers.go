package mappers

import (
	"database/sql"
	"strconv"
	"wasabi/model"
	"wasabi/units"
)

func MapProductMasterToTransaction(ar *model.TransactionRecord, master *model.ProductMaster) {
	// Only set the unit price from the master's NHI price if the transaction
	// does not already have a unit price (e.g., from a DAT file).
	if ar.UnitPrice == 0 {
		ar.UnitPrice = master.NhiPrice
	}

	ar.YjCode = master.YjCode
	ar.ProductName = master.ProductName
	ar.KanaName = master.KanaName
	ar.MakerName = master.MakerName
	ar.UsageClassification = master.UsageClassification
	ar.PackageForm = master.PackageForm
	ar.PurchasePrice = master.PurchasePrice
	ar.SupplierWholesale = master.SupplierWholesale
	ar.YjPackUnitQty = master.YjPackUnitQty
	ar.JanPackUnitQty = master.JanPackUnitQty
	ar.JanPackInnerQty = master.JanPackInnerQty
	ar.FlagPoison = master.FlagPoison
	ar.FlagDeleterious = master.FlagDeleterious
	ar.FlagNarcotic = master.FlagNarcotic
	ar.FlagPsychotropic = master.FlagPsychotropic
	ar.FlagStimulant = master.FlagStimulant
	ar.FlagStimulantRaw = master.FlagStimulantRaw

	yjUnitName := units.ResolveName(master.YjUnitName)
	janUnitCodeStr := strconv.Itoa(master.JanUnitCode)
	var janUnitName string
	if janUnitCodeStr == "0" || janUnitCodeStr == "" {
		janUnitName = yjUnitName
	} else {
		janUnitName = units.ResolveName(janUnitCodeStr)
	}
	ar.JanUnitName = janUnitName
	ar.YjUnitName = yjUnitName
	ar.JanUnitCode = janUnitCodeStr

	tempJcshms := model.JCShms{
		JC037: master.PackageSpec,
		JC039: master.YjUnitName,
		JC044: master.YjPackUnitQty,
		JA006: sql.NullFloat64{Float64: master.JanPackInnerQty, Valid: true},
		JA008: sql.NullFloat64{Float64: master.JanPackUnitQty, Valid: true},
		JA007: sql.NullString{String: strconv.Itoa(master.JanUnitCode), Valid: true},
	}
	ar.PackageSpec = units.FormatPackageSpec(&tempJcshms)
}
