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
	"wasabi/backorder"
	"wasabi/backup"
	"wasabi/config"
	"wasabi/dat"
	"wasabi/db"
	"wasabi/deadstock"
	"wasabi/guidedinventory"
	"wasabi/inout"
	"wasabi/inventory"
	"wasabi/loader"
	"wasabi/masteredit"
	"wasabi/medrec"
	"wasabi/orders"
	"wasabi/precomp"
	"wasabi/pricing"
	"wasabi/product"
	"wasabi/reprocess"
	"wasabi/returns"
	"wasabi/settings"
	"wasabi/stock"
	"wasabi/transaction"
	"wasabi/units"
	"wasabi/usage"
	"wasabi/valuation"
)

func main() {
	conn, err := sql.Open("sqlite3", "./wasabi.db")
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	conn.Exec("PRAGMA journal_mode = WAL;")
	conn.Exec("PRAGMA busy_timeout = 5000;")
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
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
	mux.HandleFunc("/api/valuation", valuation.GetValuationHandler(conn))
	mux.HandleFunc("/api/valuation/export", valuation.ExportValuationHandler(conn))
	mux.HandleFunc("/api/dat/upload", dat.UploadDatHandler(conn))
	mux.HandleFunc("/api/usage/upload", usage.UploadUsageHandler(conn))
	mux.HandleFunc("/api/inout/save", inout.SaveInOutHandler(conn))
	mux.HandleFunc("/api/inventory/upload", inventory.UploadInventoryHandler(conn))
	mux.HandleFunc("/api/inventory/list", inventory.ListInventoryProductsHandler(conn))
	mux.HandleFunc("/api/inventory/save_manual", inventory.SaveManualInventoryHandler(conn))
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
	// ▼▼▼【ここに追加】▼▼▼
	mux.HandleFunc("/api/pricing/backup_export", pricing.BackupExportHandler(conn))
	// ▲▲▲【追加ここまで】▲▲▲
	mux.HandleFunc("/api/transactions/reprocess", reprocess.ProcessTransactionsHandler(conn))
	mux.HandleFunc("/api/deadstock/list", deadstock.GetDeadStockHandler(conn))
	mux.HandleFunc("/api/deadstock/save", deadstock.SaveDeadStockHandler(conn))
	mux.HandleFunc("/api/deadstock/import", deadstock.ImportDeadStockHandler(conn))
	mux.HandleFunc("/api/settings/get", settings.GetSettingsHandler(conn))
	mux.HandleFunc("/api/settings/save", settings.SaveSettingsHandler(conn))
	mux.HandleFunc("/api/settings/wholesalers", settings.WholesalersHandler(conn))
	mux.HandleFunc("/api/settings/wholesalers/", settings.WholesalersHandler(conn))
	mux.HandleFunc("/api/transactions/clear_all", settings.ClearTransactionsHandler(conn))
	// ▼▼▼【ここに追加】▼▼▼
	mux.HandleFunc("/api/masters/clear_all", settings.ClearMastersHandler(conn))
	// ▲▲▲【追加ここまで】▲▲▲
	mux.HandleFunc("/api/masters/search_all", db.SearchAllMastersHandler(conn))
	mux.HandleFunc("/api/precomp/save", precomp.SavePrecompHandler(conn))
	mux.HandleFunc("/api/precomp/load", precomp.LoadPrecompHandler(conn))
	mux.HandleFunc("/api/precomp/clear", precomp.ClearPrecompHandler(conn))
	mux.HandleFunc("/api/precomp/export", precomp.ExportPrecompHandler(conn))
	mux.HandleFunc("/api/precomp/import", precomp.ImportPrecompHandler(conn))
	// ▼▼▼【ここに追加】▼▼▼
	mux.HandleFunc("/api/precomp/import_all", precomp.BulkImportPrecompHandler(conn))
	// ▲▲▲【追加ここまで】▲▲▲
	mux.HandleFunc("/api/precomp/export_all", precomp.ExportAllPrecompHandler(conn))
	mux.HandleFunc("/api/orders/candidates", orders.GenerateOrderCandidatesHandler(conn))
	mux.HandleFunc("/api/orders/place", orders.PlaceOrderHandler(conn))
	mux.HandleFunc("/api/returns/candidates", returns.GenerateReturnCandidatesHandler(conn))
	mux.HandleFunc("/api/backorders", backorder.GetBackordersHandler(conn))
	mux.HandleFunc("/api/backorders/delete", backorder.DeleteBackorderHandler(conn))
	mux.HandleFunc("/api/backorders/bulk_delete", backorder.BulkDeleteBackordersHandler(conn))
	mux.HandleFunc("/api/masters/reload_jcshms", loader.CreateMasterUpdateHandler(conn))
	mux.HandleFunc("/api/pricing/export", pricing.GetExportDataHandler(conn))
	mux.HandleFunc("/api/pricing/upload", pricing.UploadQuotesHandler(conn))
	mux.HandleFunc("/api/pricing/update", pricing.BulkUpdateHandler(conn))
	mux.HandleFunc("/api/pricing/all_masters", pricing.GetAllMastersForPricingHandler(conn))
	// ▼▼▼ Add this line ▼▼▼
	mux.HandleFunc("/api/pricing/direct_import", pricing.DirectImportHandler(conn))
	// ▲▲▲ Addition complete ▲▲▲
	mux.HandleFunc("/api/masters/by_yj_code", db.GetMastersByYjCodeHandler(conn))
	mux.HandleFunc("/api/stock/current", stock.GetCurrentStockHandler(conn))
	mux.HandleFunc("/api/stock/all_current", stock.GetAllCurrentStockHandler(conn))
	mux.HandleFunc("/api/medrec/download", medrec.DownloadHandler(conn))
	mux.HandleFunc("/api/products/search_filtered", product.SearchProductsHandler(conn))
	mux.HandleFunc("/api/inventory/adjust/data", guidedinventory.GetInventoryDataHandler(conn))
	mux.HandleFunc("/api/inventory/adjust/save", guidedinventory.SaveInventoryDataHandler(conn))

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
