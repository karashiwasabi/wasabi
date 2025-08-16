package loader

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

// ReloadJcshmsHandler handles the request to reload JCSHMS and JANCODE master files.
func ReloadJcshmsHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Attempting to reload JCSHMS and JANCODE master files...")

		if err := LoadCSV(conn, "SOU/JCSHMS.CSV", "jcshms", 125, false); err != nil {
			log.Printf("Error reloading JCSHMS.CSV: %v", err)
			http.Error(w, "Failed to reload JCSHMS.CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := LoadCSV(conn, "SOU/JANCODE.CSV", "jancode", 30, true); err != nil {
			log.Printf("Error reloading JANCODE.CSV: %v", err)
			http.Error(w, "Failed to reload JANCODE.CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("JCSHMS and JANCODE master files reloaded successfully.")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "JCSHMS・JANCODEマスターの再読み込みが完了しました。",
		})
	}
}
