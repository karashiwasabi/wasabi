package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// =======================================================================
// 共通設定（ANSI前提）
// =======================================================================

const (
	// マスター・MAファイルのパス
	MasterDir       = "C:\\Dev\\WASABI\\SOU\\"
	JCSHMSFilePath  = MasterDir + "JCSHMS.CSV"
	JANCODEFilePath = MasterDir + "JANCODE.CSV"

	// MA0: マスター照合一致時のドッキング結果（どちらからも MA0 へ追加、既存内容は保持）
	// MA1: USAGEで master 照合に一致しなかったものを出力
	// MA2: DATで master 照合に一致しなかったものを出力
	MA0FilePath = MasterDir + "MA0.csv"
	MA1FilePath = MasterDir + "MA1.csv"
	MA2FilePath = MasterDir + "MA2.csv"

	// DATは固定長ファイル。各行は128バイトとする
	requiredDATBytes = 128

	// 出力先フォルダ
	OrganizedDATDir   = "C:\\Dev\\WASABI\\OrganizedDAT"
	OrganizedUSAGEDir = "C:\\Dev\\WASABI\\OrganizedUSAGE"
)

// DAT用固定長フィールド定義（フィールド0～13）
// 0: RecordTypeEtc = 3, 1: DeliveryFlag = 1, 2: Date = 8,
// 3: ReceiptNumber = 10, 4: LineNumber = 2, 5: Filler = 1,
// 6: ProductCode = 13, 7: ProductName = 40, 8: Quantity = 5,
// 9: UnitPrice = 9, 10: Subtotal = 9, 11: PackagingDrugPrice = 8,
// 12: ExpiryDate = 6, 13: LotNumber = 13
var datFieldLengths = []int{3, 1, 8, 10, 2, 1, 13, 40, 5, 9, 9, 8, 6, 13}

// =======================================================================
// 重複排除用グローバル変数
// =======================================================================

// DAT処理：重複キーは「卸コード_日付_伝票番号」
var duplicateDATMap = make(map[string]bool)

// USAGE処理：重複キーは、全項目を連結した文字列
var duplicateUSAGEMap = make(map[string]bool)

// MA0：どちらからも MA0 に出力する際、JANコードで重複が無いようグローバルチェック
var globalMa0Dup = make(map[string]bool)

// USAGE用：マスター照合に一致しなかった場合の重複出力を防ぐ（キー：JANコード）
var globalUsageMa1Dup = make(map[string]bool)

// =======================================================================
// 共通ユーティリティ（ANSI前提）
// =======================================================================

// substring: 文字列 s の start 位置から length 文字を切り出し、TrimSpace して返す
func substring(s string, start, length int) string {
	if start < 0 || start >= len(s) {
		return ""
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	return strings.TrimSpace(s[start:end])
}

// parseDateFromRecord: DATレコードのフィールド2（Date: YYYYMMDD）から先頭6文字（YYYYMM）を抽出する
func parseDateFromRecord(fields []string) (string, error) {
	if len(fields) < 3 {
		return "", fmt.Errorf("フィールド数不足")
	}
	dateField := fields[2]
	if len(dateField) < 6 {
		return "", fmt.Errorf("不正な日付フィールド: %s", dateField)
	}
	return dateField[:6], nil
}

// =======================================================================
// マスター読み込み（ANSI前提）
// =======================================================================

func loadJchmsMaster(filePath string) (map[string][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("JCSHMSファイル '%s' を開けませんでした: %w", filePath, err)
	}
	defer file.Close()
	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1
	masterMap := make(map[string][]string)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("JCSHMS CSV 読み込みエラー: %v", err)
			continue
		}
		if len(record) < 1 {
			continue
		}
		jan := strings.TrimSpace(record[0])
		if jan != "" {
			masterMap[jan] = record
		}
	}
	return masterMap, nil
}

func loadJancodeMaster(filePath string) (map[string][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("JANCODEファイル '%s' を開けませんでした: %w", filePath, err)
	}
	defer file.Close()
	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1
	masterMap := make(map[string][]string)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("JANCODE CSV 読み込みエラー: %v", err)
			continue
		}
		if len(record) < 2 {
			continue
		}
		jan := strings.TrimSpace(record[1])
		if jan != "" {
			masterMap[jan] = record
		}
	}
	return masterMap, nil
}

// =======================================================================
// DAT処理 (固定長, ANSI前提)
// =======================================================================

func processDATFile(
	file io.Reader, fileName string,
	jchmsMaster, jancodeMaster map[string][]string,
	orgOutWriters map[string]*csv.Writer,
	ma0Writer io.Writer, ma2Writer io.Writer,
) error {
	scanner := bufio.NewScanner(file)
	var currentWholesaleCode string // S行から取得する卸コード
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		// S行: 先頭が "S" の行から卸コード取得
		if strings.HasPrefix(line, "S") {
			if len(line) >= 13 {
				currentWholesaleCode = substring(line, 3, 10)
			} else {
				currentWholesaleCode = ""
			}
			continue
		}

		// D行: 先頭が "D" の行のみ処理
		if !strings.HasPrefix(line, "D") {
			continue
		}

		// 行長が requiredDATBytes 未満ならスペースでパディング
		if len(line) < requiredDATBytes {
			line = line + strings.Repeat(" ", requiredDATBytes-len(line))
		}

		// 固定長フィールド抽出
		fields := make([]string, len(datFieldLengths))
		pos := 0
		for i, flen := range datFieldLengths {
			if pos+flen > len(line) {
				fields[i] = ""
			} else {
				fields[i] = substring(line, pos, flen)
			}
			pos += flen
		}

		// OrganizedDAT 用レコード: 「卸コード + fields」
		orgRecord := make([]string, 0, 15)
		orgRecord = append(orgRecord, currentWholesaleCode)
		orgRecord = append(orgRecord, fields...)

		// 重複キーは「卸コード_日付_伝票番号」
		dupKey := currentWholesaleCode + "_" + fields[2] + "_" + fields[3]
		if duplicateDATMap[dupKey] {
			continue
		}
		duplicateDATMap[dupKey] = true

		// OrganizedDAT出力： 日付フィールド (fields[2]) の先頭6文字を月キーとして抽出
		if len(fields[2]) < 6 {
			log.Printf("警告: 行 %d の日付フィールドが不正: '%s'", lineNumber, fields[2])
			continue
		}
		monthKey, err := parseDateFromRecord(fields)
		if err != nil {
			log.Printf("日付抽出エラー: %v", err)
			continue
		}
		writer, ok := orgOutWriters[monthKey]
		if !ok {
			outFileName := fmt.Sprintf("OrganizedDAT%s.csv", monthKey)
			outFilePath := filepath.Join(OrganizedDATDir, outFileName)
			if _, err := os.Stat(OrganizedDATDir); os.IsNotExist(err) {
				if mkErr := os.MkdirAll(OrganizedDATDir, os.ModePerm); mkErr != nil {
					log.Printf("OrganizedDATフォルダ作成エラー: %v", mkErr)
					continue
				}
			}
			outFile, err := os.Create(outFilePath)
			if err != nil {
				log.Printf("OrganizedDATファイル作成エラー: %v", err)
				continue
			}
			writer = csv.NewWriter(outFile)
			orgOutWriters[monthKey] = writer
		}
		if err := writer.Write(orgRecord); err != nil {
			log.Printf("OrganizedDAT書き出しエラー: %v", err)
		}

		// マスター照合: JANコードは fields[6] を利用
		janCode := fields[6]
		jchmsRec, ok1 := jchmsMaster[janCode]
		jancodeRec, ok2 := jancodeMaster[janCode]
		if ok1 && ok2 {
			// 一致した場合 → MA0 に出力（追記モード、globalMa0Dup により重複排除）
			if !globalMa0Dup[janCode] {
				globalMa0Dup[janCode] = true
				dockingRecord := append([]string{}, jchmsRec...)
				dockingRecord = append(dockingRecord, jancodeRec...)
				wr := csv.NewWriter(ma0Writer)
				if err := wr.Write(dockingRecord); err != nil {
					log.Printf("DAT MA0書き出しエラー: %v", err)
				}
				wr.Flush()
			}
		} else {
			// 不一致の場合 → DAT は MA2 に出力 (ここは新規作成/上書きで出力)
			wr := csv.NewWriter(ma2Writer)
			if err := wr.Write(orgRecord); err != nil {
				log.Printf("DAT MA2書き出しエラー: %v", err)
			}
			wr.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("DATファイルスキャンエラー: %w", err)
	}
	return nil
}

// =======================================================================
// USAGE処理 (CSV形式, ANSI前提)
// =======================================================================

// USAGEファイルはCSV形式（例:
// 20250501,2229001F1053,4987858210040,アストミン錠１０ｍｇ,45.00,16)
// 重複キーは各レコード（全項目連結）
// マスター照合に一致する場合は MA0 に出力（追記）し、一致しなければ USAGEでは MA1 に出力する
func processUSAGEFile(
	file io.Reader, fileName string,
	jchmsMaster, jancodeMaster map[string][]string,
	orgOutWriters map[string]*csv.Writer,
	ma1Writer io.Writer,
) error {

	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("USAGE CSV 読み込みエラー: %v", err)
			continue
		}

		dupKey := strings.Join(record, ",")
		if duplicateUSAGEMap[dupKey] {
			continue
		}
		duplicateUSAGEMap[dupKey] = true

		// OrganizedUSAGE 出力： Usage Date (record[0]) の先頭6文字を月キーとする
		if len(record) < 1 {
			log.Printf("USAGEレコード不足: %v", record)
			continue
		}
		monthKey := ""
		if len(record[0]) >= 6 {
			monthKey = record[0][:6]
		} else {
			log.Printf("不正なUsage Date: %s", record[0])
			continue
		}
		writer, ok := orgOutWriters[monthKey]
		if !ok {
			outFileName := fmt.Sprintf("OrganizedUSAGE%s.csv", monthKey)
			outFilePath := filepath.Join(OrganizedUSAGEDir, outFileName)
			if _, err := os.Stat(OrganizedUSAGEDir); os.IsNotExist(err) {
				if mkErr := os.MkdirAll(OrganizedUSAGEDir, os.ModePerm); mkErr != nil {
					log.Printf("OrganizedUSAGEフォルダ作成エラー: %v", mkErr)
					continue
				}
			}
			outFile, err := os.Create(outFilePath)
			if err != nil {
				log.Printf("OrganizedUSAGEファイル作成エラー: %v", err)
				continue
			}
			writer = csv.NewWriter(outFile)
			orgOutWriters[monthKey] = writer
		}
		if err := writer.Write(record); err != nil {
			log.Printf("OrganizedUSAGE書き出しエラー: %v", err)
		}

		// マスター照合: USAGEでは JAN Code は record[2] とする
		if len(record) < 3 {
			log.Printf("USAGEレコード項目不足: %v", record)
			continue
		}
		janCode := strings.TrimSpace(record[2])
		jchmsRec, ok1 := jchmsMaster[janCode]
		jancodeRec, ok2 := jancodeMaster[janCode]
		if ok1 && ok2 {
			// マスター照合一致 → MA0 に追記 (globalMa0Dup により重複排除)
			if !globalMa0Dup[janCode] {
				globalMa0Dup[janCode] = true
				dockingRecord := append([]string{}, jchmsRec...)
				dockingRecord = append(dockingRecord, jancodeRec...)
				// MA0 は追記モードで既存のファイルに出力
				ma0AppendFile, err := os.OpenFile(MA0FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Printf("USAGE MA0ファイルオープンエラー: %v", err)
				} else {
					wr := csv.NewWriter(ma0AppendFile)
					if err := wr.Write(dockingRecord); err != nil {
						log.Printf("USAGE MA0書き出しエラー: %v", err)
					}
					wr.Flush()
					ma0AppendFile.Close()
				}
			}
		} else {
			// マスター照合不一致 → USAGE は MA1 に出力 (globalUsageMa1Dup により重複排除)
			if !globalUsageMa1Dup[janCode] {
				globalUsageMa1Dup[janCode] = true
				ma1AppendFile, err := os.OpenFile(MA1FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Printf("USAGE MA1ファイルオープンエラー: %v", err)
				} else {
					wr := csv.NewWriter(ma1AppendFile)
					if err := wr.Write(record); err != nil {
						log.Printf("USAGE MA1書き出しエラー: %v", err)
					}
					wr.Flush()
					ma1AppendFile.Close()
				}
			}
		}
	}
	return nil
}

// =======================================================================
// HTTPアップロードハンドラー
// =======================================================================

func uploadDatHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "フォームパースエラー: "+err.Error(), http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "DATファイルがアップロードされていません", http.StatusBadRequest)
		return
	}

	jchmsMaster, err := loadJchmsMaster(JCSHMSFilePath)
	if err != nil {
		http.Error(w, "JCSHMS読み込みエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jancodeMaster, err := loadJancodeMaster(JANCODEFilePath)
	if err != nil {
		http.Error(w, "JANCODE読み込みエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// OrganizedDAT 出力用の月別マップ
	orgOutWriters := make(map[string]*csv.Writer)

	// MA0 は追記モードで開く
	ma0File, err := os.OpenFile(MA0FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "MA0ファイルオープンエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer ma0File.Close()

	// MA2 は新規作成／上書き
	ma2File, err := os.Create(MA2FilePath)
	if err != nil {
		http.Error(w, "MA2ファイル作成エラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer ma2File.Close()
	ma2Writer := csv.NewWriter(ma2File)
	defer ma2Writer.Flush()

	// DATファイル処理
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("DATファイルオープンエラー (%s): %v", fileHeader.Filename, err)
			continue
		}
		if perr := processDATFile(file, fileHeader.Filename, jchmsMaster, jancodeMaster, orgOutWriters, ma0File, ma2File); perr != nil {
			log.Printf("DATファイル処理エラー (%s): %v", fileHeader.Filename, perr)
		}
		file.Close()
	}

	// OrganizedDAT各月ファイルのFlush
	for month, writer := range orgOutWriters {
		writer.Flush()
		log.Printf("OrganizedDAT %s の書き出し完了", month)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("DATファイルの処理が完了しました"))
}

func uploadUsageHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "フォームパースエラー: "+err.Error(), http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "USAGEファイルがアップロードされていません", http.StatusBadRequest)
		return
	}

	jchmsMaster, err := loadJchmsMaster(JCSHMSFilePath)
	if err != nil {
		http.Error(w, "JCSHMS読み込みエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jancodeMaster, err := loadJancodeMaster(JANCODEFilePath)
	if err != nil {
		http.Error(w, "JANCODE読み込みエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// OrganizedUSAGE 出力用の月別マップ
	orgOutWriters := make(map[string]*csv.Writer)

	// MA0 は追記モードで開く（DAT, USAGE 両方で共通）
	ma0File, err := os.OpenFile(MA0FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "MA0ファイルオープンエラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer ma0File.Close()

	// MA1 は新規作成／上書き
	ma1File, err := os.Create(MA1FilePath)
	if err != nil {
		http.Error(w, "MA1ファイル作成エラー: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer ma1File.Close()
	ma1Writer := csv.NewWriter(ma1File)
	defer ma1Writer.Flush()

	// USAGEファイル処理
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("USAGEファイルオープンエラー (%s): %v", fileHeader.Filename, err)
			continue
		}
		if perr := processUSAGEFile(file, fileHeader.Filename, jchmsMaster, jancodeMaster, orgOutWriters, ma1File); perr != nil {
			log.Printf("USAGEファイル処理エラー (%s): %v", fileHeader.Filename, perr)
		}
		file.Close()
	}

	// OrganizedUSAGE各月ファイルのFlush
	for month, writer := range orgOutWriters {
		writer.Flush()
		log.Printf("OrganizedUSAGE %s の書き出し完了", month)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("USAGEファイルの処理が完了しました"))
}

// =======================================================================
// HTML UI (共通: DAT/USAGEアップロード)
// =======================================================================

func serveHome(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>DAT/USAGEファイルアップロード</title>
  <style>
    .drop-area { border: 2px dashed #ccc; border-radius: 20px; width: 300px; height: 200px; padding: 20px; text-align: center; font-family: Arial, sans-serif; margin: 20px auto; }
    #result { width: 80%; margin: 20px auto; border: 1px solid #ccc; padding: 10px; font-family: Arial, sans-serif; white-space: pre-wrap; }
    button { margin: 10px; }
  </style>
</head>
<body>
  <h2>DATファイルアップロード</h2>
  <div id="dat-drop-area" class="drop-area">
    <p>DATファイルをドラッグ＆ドロップまたはボタンから選択</p>
    <button id="dat-uploadBtn">ファイル選択</button>
    <input type="file" id="dat-fileInput" style="display:none;" multiple accept=".dat">
  </div>
  <h2>USAGEファイルアップロード</h2>
  <div id="usage-drop-area" class="drop-area">
    <p>USAGEファイルをドラッグ＆ドロップまたはボタンから選択</p>
    <button id="usage-uploadBtn">ファイル選択</button>
    <input type="file" id="usage-fileInput" style="display:none;" multiple accept=".csv,.dat">
  </div>
  <div id="result">結果がここに表示されます</div>
  <script>
    function setupUploader(dropAreaId, buttonId, fileInputId, endpoint) {
      const fileInput = document.getElementById(fileInputId);
      const dropArea = document.getElementById(dropAreaId);
      const resultDiv = document.getElementById('result');
      ['dragenter','dragover','dragleave','drop'].forEach(eventName => {
        dropArea.addEventListener(eventName, e => { e.preventDefault(); e.stopPropagation(); });
      });
      dropArea.addEventListener('drop', e => {
        const files = e.dataTransfer.files;
        handleFiles(files, endpoint, resultDiv);
      });
      document.getElementById(buttonId).addEventListener('click', () => { fileInput.click(); });
      fileInput.addEventListener('change', e => { handleFiles(e.target.files, endpoint, resultDiv); });
    }
    function handleFiles(files, endpoint, resultDiv) {
      const formData = new FormData();
      for (let i = 0; i < files.length; i++) {
        formData.append("file", files[i]);
      }
      fetch(endpoint, { method:"POST", body: formData })
      .then(response => response.text())
      .then(data => { resultDiv.textContent = data; })
      .catch(error => { resultDiv.textContent = "アップロードエラー: " + error; });
    }
    setupUploader("dat-drop-area", "dat-uploadBtn", "dat-fileInput", "/uploadDat");
    setupUploader("usage-drop-area", "usage-uploadBtn", "usage-fileInput", "/uploadUsage");
  </script>
</body>
</html>
	`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// =======================================================================
// main
// =======================================================================

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/uploadDat", uploadDatHandler)     // DAT処理
	http.HandleFunc("/uploadUsage", uploadUsageHandler) // USAGE処理
	log.Println("サーバー起動: http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
