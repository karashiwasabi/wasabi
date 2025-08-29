package valuation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"

	"github.com/xuri/excelize/v2"
)

// GetValuationHandler は在庫評価レポートのデータを返します
func GetValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		}

		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		}

		results, err := db.GetInventoryValuation(conn, filters)

		if err != nil {
			http.Error(w, "Failed to get inventory valuation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// ExportValuationHandler は在庫評価レポートをExcelファイルとしてエクスポートします。
func ExportValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		}

		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		}

		results, err := db.GetInventoryValuation(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get inventory valuation for export: "+err.Error(), http.StatusInternalServerError)
			return
		}

		f := excelize.NewFile()
		sheetName := "在庫評価一覧"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1")

		// ヘッダーを書き込み
		headers := []string{"剤型", "製品名", "包装", "在庫数", "YJ単位", "薬価金額", "納入価金額"}
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F0F0F0"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
			f.SetCellStyle(sheetName, cell, cell, headerStyle)
		}

		// スタイル設定
		currencyStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 2}) // 桁区切り
		ucMap := map[string]string{"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"}
		rowNum := 2
		var grandTotalNhi, grandTotalPurchase float64

		for _, group := range results {
			ucName := ucMap[strings.TrimSpace(group.UsageClassification)]
			if ucName == "" {
				ucName = group.UsageClassification
			}

			// 剤型ごとの詳細行
			for _, row := range group.DetailRows {
				stockStr := fmt.Sprintf("%.2f", row.Stock)
				f.SetCellValue(sheetName, "A"+strconv.Itoa(rowNum), ucName)
				f.SetCellValue(sheetName, "B"+strconv.Itoa(rowNum), row.ProductName)
				f.SetCellValue(sheetName, "C"+strconv.Itoa(rowNum), row.PackageSpec)
				f.SetCellValue(sheetName, "D"+strconv.Itoa(rowNum), stockStr)
				f.SetCellValue(sheetName, "E"+strconv.Itoa(rowNum), row.YjUnitName)
				f.SetCellValue(sheetName, "F"+strconv.Itoa(rowNum), row.TotalNhiValue)
				f.SetCellValue(sheetName, "G"+strconv.Itoa(rowNum), row.TotalPurchaseValue)
				rowNum++
			}
			grandTotalNhi += group.TotalNhiValue
			grandTotalPurchase += group.TotalPurchaseValue
		}

		// 総合計
		f.SetCellValue(sheetName, "E"+strconv.Itoa(rowNum+1), "総合計")
		f.SetCellValue(sheetName, "F"+strconv.Itoa(rowNum+1), grandTotalNhi)
		f.SetCellValue(sheetName, "G"+strconv.Itoa(rowNum+1), grandTotalPurchase)
		totalStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		f.SetCellStyle(sheetName, "E"+strconv.Itoa(rowNum+1), "G"+strconv.Itoa(rowNum+1), totalStyle)
		f.SetCellStyle(sheetName, "F2", "G"+strconv.Itoa(rowNum+1), currencyStyle)

		// ファイル名を設定してダウンロード
		fileName := fmt.Sprintf("在庫評価一覧_%s.xlsx", filters.Date)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		if err := f.Write(w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
