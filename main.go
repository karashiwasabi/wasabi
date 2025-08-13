package main

import (
	"database/sql"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	_ "github.com/mattn/go-sqlite3"

	"wasabi/aggregation" // ★ aggregationパッケージのインポートを追加
	"wasabi/backup"
	"wasabi/dat"
	"wasabi/db"
	"wasabi/inout"
	"wasabi/inventory"
	"wasabi/loader"
	"wasabi/masteredit"
	"wasabi/reprocess"
	"wasabi/transaction"
	"wasabi/units"
	"wasabi/usage"
)

func main() {
	conn, err := sql.Open("sqlite3", "./wasabi.db")
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}

	defer conn.Close()

	if err := loader.InitDatabase(conn); err != nil {
		log.Fatalf("master data initialization failed: %v", err)
	}
	if _, err := units.LoadTANIFile("SOU/TANI.CSV"); err != nil {
		log.Fatalf("tani master init failed: %v", err)
	}
	log.Println("Master data loaded successfully.")

	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/dat/upload", dat.UploadDatHandler(conn))
	mux.HandleFunc("/api/usage/upload", usage.UploadUsageHandler(conn))
	mux.HandleFunc("/api/inout/save", inout.SaveInOutHandler(conn))
	mux.HandleFunc("/api/inventory/upload", inventory.UploadInventoryHandler(conn))
	mux.HandleFunc("/api/aggregation", aggregation.GetAggregationHandler(conn)) // ★ この行を追加

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
