package units

import (
	"encoding/json"
	"net/http"
)

// GetTaniMapHandlerは、ロード済みの単位マップを返します。
func GetTaniMapHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if internalMap == nil {
			json.NewEncoder(w).Encode(make(map[string]string))
			return
		}
		json.NewEncoder(w).Encode(internalMap)
	}
}
