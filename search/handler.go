// C:\Users\wasab\OneDrive\デスクトップ\WASABI\search\handler.go
package search

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db" // ▼▼▼【ここに追加】▼▼▼
	"wasabi/mappers"
)

/**
 * @brief 製品名・カナ名でJCSHMSマスターを検索するAPIハンドラ (/api/products/search)
 */
func SearchJcshmsByNameHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if len(query) < 2 {
			http.Error(w, "Query must be at least 2 characters", http.StatusBadRequest)
			return
		}
		results, err := db.SearchJcshmsByName(conn, query)
		if err != nil {
			http.Error(w, "Failed to search products", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

/**
 * @brief 製品名・カナ名で製品マスター全体を検索するAPIハンドラ (/api/masters/search_all)
 */
func SearchAllMastersHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if len(query) < 2 {
			http.Error(w, "Query must be at least 2 characters", http.StatusBadRequest)
			return
		}
		results, err := db.SearchAllProductMastersByName(conn, query)
		if err != nil {
			http.Error(w, "Failed to search masters", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

/**
 * @brief YJコードに紐づく製品マスターのリストを取得するAPIハンドラ (/api/masters/by_yj_code)
 */
func GetMastersByYjCodeHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		yjCode := r.URL.Query().Get("yj_code")
		if yjCode == "" {
			http.Error(w, "yj_code parameter is required", http.StatusBadRequest)
			return
		}
		results, err := db.GetProductMastersByYjCode(conn, yjCode)
		if err != nil {
			http.Error(w, "Failed to get masters by yj_code", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetProductByGS1Handler はGS1コードを元に製品情報を検索し、製品マスター全体を返します。
func GetProductByGS1Handler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs1Code := r.URL.Query().Get("gs1_code")
		if gs1Code == "" {
			http.Error(w, "gs1_code is required", http.StatusBadRequest)
			return
		}

		master, err := db.GetProductMasterByGS1Code(conn, gs1Code)
		if err != nil {
			http.Error(w, "Failed to get product by gs1 code: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if master == nil {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}

		// ▼▼▼【ここから修正】共通変換関数を使用 ▼▼▼
		masterView := mappers.ToProductMasterView(master)
		// ▲▲▲【修正ここまで】▲▲▲

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masterView)
	}
}
