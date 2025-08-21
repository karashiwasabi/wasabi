// C:\Dev\WASABI\mastermanager\mastermanager.go

package mastermanager

import (
	"database/sql"
	"fmt"
	"strconv"
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

	if master, ok := mastersMap[key]; ok {
		return master, nil
	}

	// ▼▼▼ [修正点] YJコードの有無で処理を分岐させるロジックを追加 ▼▼▼
	if !isSyntheticKey {
		if jcshms, ok := jcshmsMap[janCode]; ok && jcshms.JC018 != "" {
			// JCSHMSに情報があった場合
			if jcshms.JC009 != "" {
				// ケース1: YJコードが "ある" -> 通常通り正式マスターを作成
				input := createMasterInputFromJcshms(janCode, jcshms.JC009, jcshms)
				if err := db.CreateProductMasterInTx(tx, input); err != nil {
					return nil, fmt.Errorf("failed to create master from jcshms: %w", err)
				}
				newMaster := createMasterModelFromInput(input)
				mastersMap[key] = &newMaster
				return &newMaster, nil
			}
			// ケース2: YJコードが "ない" -> このifブロックを抜け、下の仮登録ルートに進む
		}
	}
	// ▲▲▲ 修正ここまで ▲▲▲

	// 仮登録ルート (JCSHMSにない、またはJCSHMSにあってもYJがない製品がここに来る)
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

	// ▼▼▼ [修正点] YJコードなしJCSHMS品の場合、剤型を「他」に上書きする ▼▼▼
	if !isSyntheticKey {
		if jcshms, ok := jcshmsMap[janCode]; ok && jcshms.JC009 == "" {
			provisionalInput.UsageClassification = "他"
		}
	}
	// ▲▲▲ 修正ここまで ▲▲▲

	if err := db.CreateProductMasterInTx(tx, provisionalInput); err != nil {
		return nil, fmt.Errorf("failed to create provisional master: %w", err)
	}

	newMaster := createMasterModelFromInput(provisionalInput)
	mastersMap[key] = &newMaster
	return &newMaster, nil
}

// createMasterInputFromJcshms はJCSHMSのデータからDB登録用のProductMasterInputを作成するヘルパー関数です。
func createMasterInputFromJcshms(jan, yj string, jcshms *model.JCShms) model.ProductMasterInput {
	var nhiPrice float64
	if jcshms.JC044 > 0 {
		nhiPrice = jcshms.JC050 / jcshms.JC044
	}
	janUnitCodeVal, _ := strconv.Atoi(jcshms.JA007.String)
	return model.ProductMasterInput{
		ProductCode:         jan,
		YjCode:              yj,
		ProductName:         jcshms.JC018,
		Origin:              "JCSHMS",
		KanaName:            jcshms.JC022,
		MakerName:           jcshms.JC030,
		UsageClassification: jcshms.JC013,
		PackageForm:         jcshms.JC037,
		YjUnitName:          jcshms.JC039,
		YjPackUnitQty:       jcshms.JC044,
		FlagPoison:          jcshms.JC061,
		FlagDeleterious:     jcshms.JC062,
		FlagNarcotic:        jcshms.JC063,
		FlagPsychotropic:    jcshms.JC064,
		FlagStimulant:       jcshms.JC065,
		FlagStimulantRaw:    jcshms.JC066,
		JanPackInnerQty:     jcshms.JA006.Float64,
		JanUnitCode:         janUnitCodeVal,
		JanPackUnitQty:      jcshms.JA008.Float64,
		NhiPrice:            nhiPrice,
	}
}

// createMasterModelFromInput はDB登録用のInputからメモリマップ格納用のProductMasterを作成するヘルパー関数です。
func createMasterModelFromInput(input model.ProductMasterInput) model.ProductMaster {
	return model.ProductMaster(input)
}