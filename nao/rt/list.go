package rt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// List is a list parameter as JSONList encodes it: the JSON text it
// binds as, or the error that stops the statement binding it, so a list
// Volt cannot bind fails its statement like any other argument instead
// of panicking where the statement is built (spec §predicates).
type List struct {
	text   string
	err    error
	absent bool
}

// Value is the list's JSON text, NULL for an absent optional list, or
// the error that stops it binding.
func (l List) Value() (driver.Value, error) {
	switch {
	case l.err != nil:
		return nil, l.err
	case l.absent:
		return nil, nil
	}
	return l.text, nil
}

// String is the list's JSON text, NULL for an absent optional list, or
// its error.
func (l List) String() string {
	switch {
	case l.err != nil:
		return l.err.Error()
	case l.absent:
		return "NULL"
	}
	return l.text
}

// JSONList encodes a list parameter (spec §predicates) as the JSON
// array a generated select binds to `json_each(?)`: one static statement
// whatever the list length, prepared once. An element that implements
// driver.Valuer is encoded as its driver value, the form the column
// holds, and a time.Time, or an element whose driver value is one, as its
// TimeText, the text the column holds (spec §go-types). A nil or empty
// slice is "[]", which matches no row. An element that cannot be encoded,
// such as a time outside TimeText's years, makes the List an error the
// statement returns.
func JSONList[T any](v []T) List {
	if len(v) == 0 {
		return List{text: "[]"}
	}
	vals := make([]any, len(v))
	for i, x := range v {
		vals[i] = x
		if s, ok, err := timeBound(x); ok || err != nil {
			if err != nil {
				return List{err: fmt.Errorf("rt: list item %d: %w", i, err)}
			}
			vals[i] = s
			continue
		}
		if dv, ok := any(x).(driver.Valuer); ok {
			val, err := dv.Value()
			if err != nil {
				return List{err: fmt.Errorf("rt: list item %d: %w", i, err)}
			}
			vals[i] = val
		}
	}
	b, err := json.Marshal(vals)
	if err != nil {
		return List{err: fmt.Errorf("rt: list: %w", err)}
	}
	return List{text: string(b)}
}

// JSONListOptional encodes an optional list parameter (spec
// §predicates): NULL when the list is absent, which makes every term
// that reads it true, and JSONList of it when present, so a present
// empty list is "[]" and matches no row.
func JSONListOptional[T any](v Null[[]T]) List {
	if !v.Valid {
		return List{absent: true}
	}
	return JSONList(v.V)
}
