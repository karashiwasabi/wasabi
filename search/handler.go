// C:\Users\wasab\OneDrive\デスクトップ\WASABI\search\handler.go

package search

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"wasabi/db"
)

/**
 * @brief 製品名・カナ名でJCSHMSマスターを検索するAPIハンドラ (/api/products/search)
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * クエリパラメータ "q" で検索キーワードを受け取ります。キーワードは2文字以上必要です。
 * 製品検索モーダルなどで使用されます。
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
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * クエリパラメータ "q" で検索キーワードを受け取ります。キーワードは2文字以上必要です。
 * JCSHMS由来でない、手動登録されたマスターも検索対象に含みます。
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
 * @param conn データベース接続
 * @return http.HandlerFunc HTTPリクエストを処理するハンドラ関数
 * @details
 * クエリパラメータ "yj_code" でYJコードを受け取ります。
 * 「棚卸調整」画面などで、同一YJコードの包装バリエーションを全て取得するために使用されます。
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
