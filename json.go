package volt

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response with the right Content-Type.
// Copy-paste and customize freely — like nao's rt.JSON, it is a
// convenience, not a framework.
func JSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}
