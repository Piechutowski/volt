package rt

import "encoding/json"

// JSONList encodes a list parameter (spec §V10.3) as the JSON array a
// generated select binds to `json_each(:name)`: one static statement
// whatever the list length, prepared once (D66). A nil or empty slice
// is "[]", which matches no row. The element types a list parameter
// can carry — the integer, float, string and bool families — always
// marshal, so the error path is unreachable and the result is plain.
func JSONList[T any](v []T) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic("rt.JSONList: " + err.Error()) // unreachable for the admitted element types
	}
	return string(b)
}
