package mastermanager

import (
	"database/sql"
	"fmt"
	"strconv"
	"wasabi/db"
	"wasabi/model"
)

// FindOrCreate は、マスターの特定と作成に関する全てのロジックを集約した関数です。
// 適切なマスターを返すことを保証し、処理中に新しいマスターが作成された場合はメモリマップも更新します。
func FindOrCreate(
	tx *sql.Tx,
	janCode string,
	productName string,
	mastersMap map[string]*model.ProductMaster,
	jcshmsMap map[string]*model.JCShms,
) (*model.ProductMaster, error) {

	// 1. マスター特定用のキーを生成（JANがあればJAN、なければ製品名から合成）
	key := janCode
	isSyntheticKey := false
	if key == "" || key == "0000000000000" {
		key = fmt.Sprintf("9999999999999%s", productName)
		isSyntheticKey = true
	}

	// 2. まずメモリ上のマップを確認
	if master, ok := mastersMap[key]; ok {
		return master, nil // 発見した場合は即座に返す
	}

	// 3. メモリにない場合、JCSHMS経由での作成を試みる (JANコードが存在する場合のみ)
	if !isSyntheticKey {
		if jcshms, ok := jcshmsMap[janCode]; ok && jcshms.JC018 != "" {
			// JCSHMSに情報があった場合、正式なマスターを作成
			yjCode := jcshms.JC009
			if yjCode == "" {
				// YJコードがなければ新規採番
				newYj, err := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
				if err != nil {
					return nil, fmt.Errorf("failed to get next sequence for jcshms master: %w", err)
				}
				yjCode = newYj
			}

			// JCSHMSデータからProductMasterInputを作成
			input := createMasterInputFromJcshms(janCode, yjCode, jcshms)
			if err := db.CreateProductMasterInTx(tx, input); err != nil {
				return nil, fmt.Errorf("failed to create master from jcshms: %w", err)
			}

			// DB登録後、メモリマップに反映するためのモデルを作成
			newMaster := createMasterModelFromInput(input)
			mastersMap[key] = &newMaster // メモリマップを更新
			return &newMaster, nil
		}
	}

	// 4. JCSHMSに情報がなければ、仮マスターを作成する
	newYj, err := db.NextSequenceInTx(tx, "MA2Y", "MA2Y", 8)
	if err != nil {
		return nil, fmt.Errorf("failed to get next sequence for provisional master: %w", err)
	}

	provisionalInput := model.ProductMasterInput{
		ProductCode: key,
		YjCode:      newYj,
		ProductName: productName,
		Origin:      "PROVISIONAL",
	}
	if err := db.CreateProductMasterInTx(tx, provisionalInput); err != nil {
		return nil, fmt.Errorf("failed to create provisional master: %w", err)
	}

	// DB登録後、メモリマップに反映
	newMaster := createMasterModelFromInput(provisionalInput)
	mastersMap[key] = &newMaster // メモリマップを更新
	return &newMaster, nil
}

// createMasterInputFromJcshms はJCSHMSのデータからDB登録用のProductMasterInputを作成するヘルパー関数です。
func createMasterInputFromJcshms(jan, yj string, jcshms *model.JCShms) model.ProductMasterInput {
	var nhiPrice float64
	if jcshms.JC044 > 0 {
		nhiPrice = jcshms.JC050 / jcshms.JC044
	}

	// ▼▼▼ 修正箇所 ▼▼▼
	// jcshms.JA007.String (文字列) を正しく整数に変換する
	janUnitCodeVal, _ := strconv.Atoi(jcshms.JA007.String)
	// ▲▲▲ 修正箇所 ▲▲▲

	return model.ProductMasterInput{
		ProductCode:         jan,
		YjCode:              yj,
		ProductName:         jcshms.JC018,
		Origin:              "JCSHMS",
		KanaName:            jcshms.JC022,
		MakerName:           jcshms.JC030,
		UsageClassification: jcshms.JC013,
		PackageForm:         jcshms.JC037,
		PackageSpec:         jcshms.JC037,
		YjUnitName:          jcshms.JC039,
		YjPackUnitQty:       jcshms.JC044,
		FlagPoison:          jcshms.JC061,
		FlagDeleterious:     jcshms.JC062,
		FlagNarcotic:        jcshms.JC063,
		FlagPsychotropic:    jcshms.JC064,
		FlagStimulant:       jcshms.JC065,
		FlagStimulantRaw:    jcshms.JC066,
		JanPackInnerQty:     jcshms.JA006.Float64,
		JanUnitCode:         janUnitCodeVal, // 修正した値を正しくセットする
		JanPackUnitQty:      jcshms.JA008.Float64,
		NhiPrice:            nhiPrice,
	}
}

// createMasterModelFromInput はDB登録用のInputからメモリマップ格納用のProductMasterを作成するヘルパー関数です。
func createMasterModelFromInput(input model.ProductMasterInput) model.ProductMaster {
	return model.ProductMaster{
		ProductCode:         input.ProductCode,
		YjCode:              input.YjCode,
		ProductName:         input.ProductName,
		Origin:              input.Origin,
		KanaName:            input.KanaName,
		MakerName:           input.MakerName,
		UsageClassification: input.UsageClassification,
		PackageForm:         input.PackageForm,
		PackageSpec:         input.PackageSpec,
		YjUnitName:          input.YjUnitName,
		YjPackUnitQty:       input.YjPackUnitQty,
		FlagPoison:          input.FlagPoison,
		FlagDeleterious:     input.FlagDeleterious,
		FlagNarcotic:        input.FlagNarcotic,
		FlagPsychotropic:    input.FlagPsychotropic,
		FlagStimulant:       input.FlagStimulant,
		FlagStimulantRaw:    input.FlagStimulantRaw,
		JanPackInnerQty:     input.JanPackInnerQty,
		JanUnitCode:         input.JanUnitCode,
		JanPackUnitQty:      input.JanPackUnitQty,
		NhiPrice:            input.NhiPrice,
		PurchasePrice:       input.PurchasePrice,
		SupplierWholesale:   input.SupplierWholesale,
	}
}
