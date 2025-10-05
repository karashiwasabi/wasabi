package mastermanager

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"
)

// FindOrCreate は、マスターの特定と作成に関する全てのロジックを集約した関数です。
func FindOrCreate(
	tx *sql.Tx,
	janCode string,
	productName string,
	mastersMap map[string]*model.ProductMaster,
	jcshmsMap map[string]*model.JCShms,
) (*model.ProductMaster, error) {

	key := janCode
	isSyntheticKey := false
	if key == "" || key == "0000000000000" {
		key = fmt.Sprintf("9999999999999%s", productName)
		isSyntheticKey = true
	}

	// 1. まずメモリ上のキャッシュ（マップ）を確認
	if master, ok := mastersMap[key]; ok {
		return master, nil
	}

	// 2. 次にデータベースを検索 (tkrのロジック)
	existingMaster, err := db.GetProductMasterByCode(tx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing master %s: %w", key, err)
	}
	// もしDBに存在すれば、それを使用する
	if existingMaster != nil {
		mastersMap[key] = existingMaster // メモリマップにも追加して次回以降の検索を高速化
		return existingMaster, nil
	}

	// 3. JCSHMSマスターから作成を試みる
	if !isSyntheticKey {
		if jcshms, ok := jcshmsMap[janCode]; ok && jcshms.JC018 != "" {
			// ヘルパー関数を呼び出して、新しいDB構造に基づいたデータを作成
			input := createMasterInputFromJcshms(janCode, jcshms)

			if err := db.UpsertProductMasterInTx(tx, input); err != nil {
				return nil, fmt.Errorf("failed to create master from jcshms: %w", err)
			}
			newMaster := createMasterModelFromInput(input)
			mastersMap[key] = &newMaster
			return &newMaster, nil
		}
	}

	// 4. メモリにもDBにもJCSHMSにも存在しない場合、新しい仮マスターを作成
	newYj, err := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
	if err != nil {
		return nil, fmt.Errorf("failed to get next sequence for provisional master: %w", err)
	}

	provisionalInput := model.ProductMasterInput{
		ProductCode: key,
		YjCode:      newYj,
		ProductName: productName, // DATファイルから読み取った名前を基本製品名として設定
		Origin:      "PROVISIONAL",
	}

	if err := db.UpsertProductMasterInTx(tx, provisionalInput); err != nil {
		return nil, fmt.Errorf("failed to create provisional master: %w", err)
	}

	newMaster := createMasterModelFromInput(provisionalInput)
	mastersMap[key] = &newMaster
	return &newMaster, nil
}

// createMasterInputFromJcshms はJCSHMSのデータからDB登録用のProductMasterInputを作成します。
func createMasterInputFromJcshms(jan string, jcshms *model.JCShms) model.ProductMasterInput {
	var nhiPrice float64
	if jcshms.JC044 > 0 {
		nhiPrice = jcshms.JC050 / jcshms.JC044
	}
	janUnitCodeVal, _ := strconv.Atoi(jcshms.JA007.String)

	input := model.ProductMasterInput{
		ProductCode:         jan,
		YjCode:              jcshms.JC009,
		ProductName:         strings.TrimSpace(jcshms.JC018), // 基本製品名 (JC018)
		Specification:       strings.TrimSpace(jcshms.JC020), // 規格 (JC020)
		Gs1Code:             jcshms.JC122,                    // GS1コード (JC122)
		Origin:              "JCSHMS",
		KanaName:            jcshms.JC022,
		MakerName:           jcshms.JC030,
		UsageClassification: jcshms.JC013,
		PackageForm:         jcshms.JC037,
		YjUnitName:          jcshms.JC039,
		YjPackUnitQty:       jcshms.JC044,
		NhiPrice:            nhiPrice,
		FlagPoison:          jcshms.JC061,
		FlagDeleterious:     jcshms.JC062,
		FlagNarcotic:        jcshms.JC063,
		FlagPsychotropic:    jcshms.JC064,
		FlagStimulant:       jcshms.JC065,
		FlagStimulantRaw:    jcshms.JC066,
		JanPackInnerQty:     jcshms.JA006.Float64,
		JanUnitCode:         janUnitCodeVal,
		JanPackUnitQty:      jcshms.JA008.Float64,
	}
	return input
}

// createMasterModelFromInput はDB登録用のInputからメモリマップ格納用のProductMasterを作成します。
func createMasterModelFromInput(input model.ProductMasterInput) model.ProductMaster {
	return model.ProductMaster{
		ProductCode:         input.ProductCode,
		YjCode:              input.YjCode,
		Gs1Code:             input.Gs1Code,
		ProductName:         input.ProductName,
		KanaName:            input.KanaName,
		MakerName:           input.MakerName,
		Specification:       input.Specification,
		UsageClassification: input.UsageClassification,
		PackageForm:         input.PackageForm,
		YjUnitName:          input.YjUnitName,
		YjPackUnitQty:       input.YjPackUnitQty,
		JanPackInnerQty:     input.JanPackInnerQty,
		JanUnitCode:         input.JanUnitCode,
		JanPackUnitQty:      input.JanPackUnitQty,
		Origin:              input.Origin,
		NhiPrice:            input.NhiPrice,
		PurchasePrice:       input.PurchasePrice,
		FlagPoison:          input.FlagPoison,
		FlagDeleterious:     input.FlagDeleterious,
		FlagNarcotic:        input.FlagNarcotic,
		FlagPsychotropic:    input.FlagPsychotropic,
		FlagStimulant:       input.FlagStimulant,
		FlagStimulantRaw:    input.FlagStimulantRaw,
		IsOrderStopped:      input.IsOrderStopped,
		SupplierWholesale:   input.SupplierWholesale,
		GroupCode:           input.GroupCode,
		ShelfNumber:         input.ShelfNumber,
		Category:            input.Category,
		UserNotes:           input.UserNotes,
	}
}
