// C:\Users\wasab\OneDrive\デスクトップ\WASABI\valuation\handler.go

package valuation

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"wasabi/db"
	"wasabi/model"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// (GetValuationHandler と ExportValuationHandler は変更ありません)
func GetValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		} //
		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		} //
		results, err := db.GetInventoryValuation(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get inventory valuation: "+err.Error(), http.StatusInternalServerError)
			return
		} //
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results) //
	}
}

func ExportValuationHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		} //
		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		} //
		results, err := db.GetInventoryValuation(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get inventory valuation for export: "+err.Error(), http.StatusInternalServerError)
			return
		} //
		f := excelize.NewFile() //
		sheetName := "在庫評価一覧"
		index, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(index)
		f.DeleteSheet("Sheet1") //
		headers := []string{"剤型", "製品名", "包装", "在庫数", "YJ単位", "薬価金額", "納入価金額"}
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F0F0F0"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		}) //
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, h)
			f.SetCellStyle(sheetName, cell, cell, headerStyle) //
		}
		currencyStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 2}) //
		ucMap := map[string]string{"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"}
		rowNum := 2
		var grandTotalNhi, grandTotalPurchase float64 //
		for _, group := range results {
			ucName := ucMap[strings.TrimSpace(group.UsageClassification)]
			if ucName == "" {
				ucName = group.UsageClassification
			} //
			for _, row := range group.DetailRows {
				stockStr := fmt.Sprintf("%.2f", row.Stock)
				f.SetCellValue(sheetName, "A"+strconv.Itoa(rowNum), ucName)
				f.SetCellValue(sheetName, "B"+strconv.Itoa(rowNum), row.ProductName)        //
				f.SetCellValue(sheetName, "C"+strconv.Itoa(rowNum), row.PackageSpec)        //
				f.SetCellValue(sheetName, "D"+strconv.Itoa(rowNum), stockStr)               //
				f.SetCellValue(sheetName, "E"+strconv.Itoa(rowNum), row.YjUnitName)         //
				f.SetCellValue(sheetName, "F"+strconv.Itoa(rowNum), row.TotalNhiValue)      //
				f.SetCellValue(sheetName, "G"+strconv.Itoa(rowNum), row.TotalPurchaseValue) //
				rowNum++
			}
			grandTotalNhi += group.TotalNhiValue           //
			grandTotalPurchase += group.TotalPurchaseValue //
		}
		f.SetCellValue(sheetName, "E"+strconv.Itoa(rowNum+1), "総合計")                                  //
		f.SetCellValue(sheetName, "F"+strconv.Itoa(rowNum+1), grandTotalNhi)                          //
		f.SetCellValue(sheetName, "G"+strconv.Itoa(rowNum+1), grandTotalPurchase)                     //
		totalStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})                //
		f.SetCellStyle(sheetName, "E"+strconv.Itoa(rowNum+1), "G"+strconv.Itoa(rowNum+1), totalStyle) //
		f.SetCellStyle(sheetName, "F2", "G"+strconv.Itoa(rowNum+1), currencyStyle)                    //
		fileName := fmt.Sprintf("在庫評価一覧_%s.xlsx", filters.Date)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName) //
		if err := f.Write(w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} //
	}
}

// ExportValuationPDFHandler は在庫評価レポートをPDFファイルとしてエクスポートします。
func ExportValuationPDFHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filters := model.ValuationFilters{
			Date:                q.Get("date"),
			KanaName:            q.Get("kanaName"),
			UsageClassification: q.Get("dosageForm"),
		} //

		if filters.Date == "" {
			http.Error(w, "Date parameter is required", http.StatusBadRequest)
			return
		} //

		results, err := db.GetInventoryValuation(conn, filters)
		if err != nil {
			http.Error(w, "Failed to get inventory valuation for export: "+err.Error(), http.StatusInternalServerError)
			return
		} //

		pdf := gofpdf.New("P", "mm", "A4", "") //
		pdf.AddPage()                          //

		pdf.SetLineWidth(0.2)                   //
		fontPath := "SOU/ipaexg.ttf"            //
		pdf.AddUTF8Font("ipaexg", "", fontPath) //

		pdf.SetFont("ipaexg", "", 14)                           //
		pdf.Cell(0, 10, fmt.Sprintf("%s 在庫評価一覧", filters.Date)) //
		pdf.Ln(15)                                              //

		pdf.SetFont("ipaexg", "", 9) //

		const (
			pageWidth          = 210.0
			pageHeight         = 297.0
			leftMargin         = 10.0
			topMargin          = 10.0
			rightMargin        = 10.0
			bottomMargin       = 15.0
			contentWidth       = pageWidth - leftMargin - rightMargin
			productNameWidth   = 70.0
			packageSpecWidth   = 55.0
			stockWidth         = 25.0
			nhiPriceWidth      = 20.0
			purchasePriceWidth = 20.0
			lineHt             = 5.0
		)
		pdf.SetMargins(leftMargin, topMargin, rightMargin)
		pdf.SetAutoPageBreak(false, bottomMargin) // 自動改ページは無効化

		// ヘッダーを描画する関数
		drawHeader := func() {
			pdf.SetFont("ipaexg", "", 9)
			pdf.SetFillColor(240, 240, 240)                                          //
			pdf.CellFormat(productNameWidth, 7, "製品名", "1", 0, "C", true, 0, "")     //
			pdf.CellFormat(packageSpecWidth, 7, "包装", "1", 0, "C", true, 0, "")      //
			pdf.CellFormat(stockWidth, 7, "在庫数", "1", 0, "C", true, 0, "")           //
			pdf.CellFormat(nhiPriceWidth, 7, "薬価金額", "1", 0, "C", true, 0, "")       //
			pdf.CellFormat(purchasePriceWidth, 7, "納入価金額", "1", 0, "C", true, 0, "") //
			pdf.Ln(7)
		}
		drawHeader()

		var grandTotalNhi float64                                                              //
		var grandTotalPurchase float64                                                         //
		ucMap := map[string]string{"1": "内", "2": "外", "3": "歯", "4": "注", "5": "機", "6": "他"} //

		for _, group := range results {
			// グループヘッダーの高さを見積もり、改ページチェック
			groupHeaderHeight := 8.0
			if pdf.GetY()+groupHeaderHeight > (pageHeight - bottomMargin) {
				pdf.AddPage()
				drawHeader()
			}

			ucName := ucMap[strings.TrimSpace(group.UsageClassification)]
			if ucName == "" {
				ucName = group.UsageClassification
			} //

			pdf.SetFont("ipaexg", "", 10)                                                                          //
			pdf.SetFillColor(220, 220, 220)                                                                        //
			pdf.CellFormat(contentWidth, groupHeaderHeight, fmt.Sprintf("■ %s", ucName), "1", 1, "L", true, 0, "") //
			pdf.SetFont("ipaexg", "", 9)

			for _, row := range group.DetailRows { //
				// 複数行になる可能性のあるテキストを行リストに分割
				productNameLines := pdf.SplitLines([]byte(row.ProductName), productNameWidth-2) //
				packageSpecLines := pdf.SplitLines([]byte(row.PackageSpec), packageSpecWidth-2) //

				// 必要な行数を計算 (製品名と包装で多い方に合わせる)
				lineCount := len(productNameLines)
				if len(packageSpecLines) > lineCount {
					lineCount = len(packageSpecLines)
				} //

				// この行に必要な高さを計算 (最低でも7mmは確保)
				requiredHeight := float64(lineCount) * lineHt
				if requiredHeight < 7.0 {
					requiredHeight = 7.0
				} //

				// 改ページが必要かチェック
				if pdf.GetY()+requiredHeight > (pageHeight - bottomMargin) { //
					pdf.AddPage()
					drawHeader() //
				}

				startX := pdf.GetX()
				startY := pdf.GetY()

				// 各セルをCellFormatで描画 (罫線も指定)
				// 製品名 (左・右罫線)
				pdf.Rect(startX, startY, productNameWidth, requiredHeight, "D") // 外枠を描画
				pdf.SetX(startX + 1)                                            // 少し内側から描画開始
				pdf.MultiCell(productNameWidth-2, lineHt, row.ProductName, "", "L", false)
				pdf.SetXY(startX+productNameWidth, startY) // 次のセルの開始位置へ

				// 包装 (右罫線のみ、Rectで描画済みなので不要)
				pdf.Rect(startX+productNameWidth, startY, packageSpecWidth, requiredHeight, "D")
				pdf.SetX(startX + productNameWidth + 1)
				pdf.MultiCell(packageSpecWidth-2, lineHt, row.PackageSpec, "", "L", false)
				pdf.SetXY(startX+productNameWidth+packageSpecWidth, startY)

				// 在庫数 (右罫線 + 中央揃え)
				stockText := fmt.Sprintf("%.2f %s", row.Stock, row.YjUnitName)                     //
				pdf.CellFormat(stockWidth, requiredHeight, stockText, "BR", 0, "RM", false, 0, "") //

				// 薬価金額 (右罫線 + 右揃え)
				nhiText := fmt.Sprintf("%s", formatCurrency(row.TotalNhiValue))                    //
				pdf.CellFormat(nhiPriceWidth, requiredHeight, nhiText, "BR", 0, "R", false, 0, "") //

				// 納入価金額 (右罫線 + 右揃え)
				purchaseText := fmt.Sprintf("円%s", formatCurrency(row.TotalPurchaseValue))                   //
				pdf.CellFormat(purchasePriceWidth, requiredHeight, purchaseText, "BR", 1, "R", false, 0, "") //

				// 描画後のY座標を更新 (SetXYやCellFormat(...,1,...)で自動更新されるが念のため)
				// pdf.SetY(startY + requiredHeight)
			}
			grandTotalNhi += group.TotalNhiValue           //
			grandTotalPurchase += group.TotalPurchaseValue //
		}

		// 総合計の行の改ページチェック
		totalHeight := 8.0
		if pdf.GetY()+totalHeight > (pageHeight - bottomMargin) {
			pdf.AddPage()
			drawHeader()
		}

		pdf.SetFont("ipaexg", "", 10)                                                                                //
		pdf.SetFillColor(240, 240, 240)                                                                              //
		pdf.CellFormat(productNameWidth+packageSpecWidth+stockWidth, 8, "総合計", "1", 0, "R", true, 0, "")             //
		pdf.CellFormat(nhiPriceWidth, 8, fmt.Sprintf("%s", formatCurrency(grandTotalNhi)), "1", 0, "R", true, 0, "") //
		// ▼▼▼ 修正箇所 ▼▼▼
		pdf.CellFormat(purchasePriceWidth, 8, "円"+formatCurrency(grandTotalPurchase), "1", 1, "R", true, 0, "") //
		// ▲▲▲ 修正ここまで ▲▲▲

		var buffer bytes.Buffer
		if err := pdf.Output(&buffer); err != nil {
			http.Error(w, "PDFの生成に失敗しました: "+err.Error(), http.StatusInternalServerError)
			return
		} //

		fileName := fmt.Sprintf("在庫評価一覧_%s.pdf", filters.Date)                                  //
		w.Header().Set("Content-Type", "application/pdf")                                       //
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName)) //
		w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))                     //

		if _, err := buffer.WriteTo(w); err != nil {
			http.Error(w, "PDFの送信に失敗しました: "+err.Error(), http.StatusInternalServerError)
		} //
	}
}

func formatCurrency(value float64) string {
	s := strconv.FormatFloat(value, 'f', 0, 64)
	if value < 0 {
		s = s[1:]
	} //
	n := len(s)
	if n <= 3 {
		if value < 0 {
			return "-" + s
		}
		return s //
	}
	start := (n-1)%3 + 1
	var parts []string
	parts = append(parts, s[:start])
	for i := start; i < n; i += 3 {
		parts = append(parts, s[i:i+3])
	} //
	result := strings.Join(parts, ",") //
	if value < 0 {
		return "-" + result
	} //
	return result
}
