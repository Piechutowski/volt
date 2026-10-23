package rt

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONList encodes a list parameter (spec §predicates) as the JSON
// array a generated select binds to `json_each(?)`: one static statement
// whatever the list length, prepared once. An element that implements
// driver.Valuer is encoded as its driver value, the form the column
// holds. A nil or empty slice is "[]", which matches no row.
func JSONList[T any](v []T) string {
	if len(v) == 0 {
		return "[]"
	}
	vals := make([]any, len(v))
	for i, x := range v {
		vals[i] = x
		if dv, ok := any(x).(driver.Valuer); ok {
			val, err := dv.Value()
			if err != nil {
				panic("rt.JSONList: " + err.Error())
			}
			vals[i] = val
		}
	}
	b, err := json.Marshal(vals)
	if err != nil {
		panic("rt.JSONList: " + err.Error())
	}
	return string(b)
}
