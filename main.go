// C:\Dev\WASABI\main.go

package main

import (
	"database/sql"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	_ "github.com/mattn/go-sqlite3"

	"wasabi/aggregation"
	"wasabi/backorder" // backorderパッケージをインポートリストに追加
	"wasabi/backup"
	"wasabi/config" // settingsの前に移動
	"wasabi/dat"
	"wasabi/db"
	"wasabi/deadstock"
	"wasabi/inout"
	"wasabi/inventory"
	"wasabi/loader"
	"wasabi/masteredit" // ▼▼▼ [修正点] 追加 ▼▼▼
	"wasabi/orders"
	"wasabi/precomp"
	"wasabi/reprocess"
	"wasabi/settings" // settingsを追加
	"wasabi/transaction"
	"wasabi/units"
	"wasabi/usage"
	"wasabi/valuation" // valuationパッケージをインポートリストに追加
)

func main() {
	conn, err := sql.Open("sqlite3", "./wasabi.db")
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	conn.Exec("PRAGMA journal_mode = WAL;")
	conn.Exec("PRAGMA busy_timeout = 5000;")
	// ▼▼▼ [修正点] 接続数を2から1に戻す ▼▼▼
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	// ▲▲▲ 修正ここまで ▲▲▲
	defer conn.Close()

	if _, err := config.LoadConfig(); err != nil {
		log.Printf("WARN: Could not load config.json: %v", err)
	}

	if err := loader.InitDatabase(conn); err != nil {
		log.Fatalf("master data initialization failed: %v", err)
	}
	if _, err := units.LoadTANIFile("SOU/TANI.CSV"); err != nil {
		log.Fatalf("tani master init failed: %v", err)
	}
	log.Println("Master data loaded successfully.")

	mux := http.NewServeMux()

	// API Endpoints
	// ▼▼▼ [修正点] 以下の1行を新しく追加 ▼▼▼
	mux.HandleFunc("/api/valuation", valuation.GetValuationHandler(conn))
	// ▲▲▲ 修正ここまで ▲▲▲
	mux.HandleFunc("/api/dat/upload", dat.UploadDatHandler(conn))
	mux.HandleFunc("/api/usage/upload", usage.UploadUsageHandler(conn))
	mux.HandleFunc("/api/inout/save", inout.SaveInOutHandler(conn))
	mux.HandleFunc("/api/inventory/upload", inventory.UploadInventoryHandler(conn))
	// ▼▼▼ [修正点] 手入力棚卸用のAPIを追加 ▼▼▼
	mux.HandleFunc("/api/inventory/list", inventory.ListInventoryProductsHandler(conn))
	mux.HandleFunc("/api/inventory/save_manual", inventory.SaveManualInventoryHandler(conn))
	// ▲▲▲ 修正ここまで ▲▲▲
	mux.HandleFunc("/api/aggregation", aggregation.GetAggregationHandler(conn))
	mux.HandleFunc("/api/clients", db.GetAllClientsHandler(conn))
	mux.HandleFunc("/api/products/search", db.SearchJcshmsByNameHandler(conn))
	mux.HandleFunc("/api/units/map", units.GetTaniMapHandler())
	mux.HandleFunc("/api/receipts", transaction.GetReceiptsHandler(conn))
	mux.HandleFunc("/api/transaction/", transaction.GetTransactionHandler(conn))
	mux.HandleFunc("/api/transaction/delete/", transaction.DeleteTransactionHandler(conn))
	mux.HandleFunc("/api/masters/editable", masteredit.GetEditableMastersHandler(conn))
	mux.HandleFunc("/api/master/update", masteredit.UpdateMasterHandler(conn))
	mux.HandleFunc("/api/clients/export", backup.ExportClientsHandler(conn))
	mux.HandleFunc("/api/clients/import", backup.ImportClientsHandler(conn))
	mux.HandleFunc("/api/products/export", backup.ExportProductsHandler(conn))
	mux.HandleFunc("/api/products/import", backup.ImportProductsHandler(conn))
	mux.HandleFunc("/api/transactions/reprocess", reprocess.ReProcessTransactionsHandler(conn))
	mux.HandleFunc("/api/deadstock/list", deadstock.GetDeadStockHandler(conn))
	mux.HandleFunc("/api/deadstock/save", deadstock.SaveDeadStockHandler(conn))
	mux.HandleFunc("/api/settings/get", settings.GetSettingsHandler(conn))
	mux.HandleFunc("/api/settings/save", settings.SaveSettingsHandler(conn))
	// ▼▼▼ 以下2行を追加 ▼▼▼
	mux.HandleFunc("/api/settings/wholesalers", settings.WholesalersHandler(conn))
	mux.HandleFunc("/api/settings/wholesalers/", settings.WholesalersHandler(conn))
	// ▲▲▲ 追加ここまで ▲▲▲
	//mux.HandleFunc("/api/medrec/download", medrec.DownloadHandler(conn))        // ▼▼▼ [修正点] 追加 ▼▼▼
	mux.HandleFunc("/api/masters/search_all", db.SearchAllMastersHandler(conn)) // 予製用の製品検索
	mux.HandleFunc("/api/precomp/save", precomp.SavePrecompHandler(conn))       // 予製データの保存
	mux.HandleFunc("/api/precomp/load", precomp.LoadPrecompHandler(conn))       // 予製データの呼び出し
	mux.HandleFunc("/api/precomp/clear", precomp.ClearPrecompHandler(conn))
	mux.HandleFunc("/api/orders/candidates", orders.GenerateOrderCandidatesHandler(conn))
	// ▼▼▼ [修正点] 以下の2行を新しく追加 ▼▼▼
	mux.HandleFunc("/api/orders/place", orders.PlaceOrderHandler(conn))
	mux.HandleFunc("/api/backorders", backorder.GetBackordersHandler(conn))
	// ▲▲▲ 修正ここまで ▲▲▲
	// ▼▼▼ [修正点] 以下の1行を新しく追加 ▼▼▼
	mux.HandleFunc("/api/masters/reload_jcshms", loader.ReloadJcshmsHandler(conn))
	// ▲▲▲ 修正ここまで ▲▲▲

	// Serve Frontend
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	port := ":8080"
	log.Printf("Server starting on http://localhost%s", port)
	go openBrowser("http://localhost" + port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default: // linux, etc.
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}
