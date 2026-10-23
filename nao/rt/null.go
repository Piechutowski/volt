package rt

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"reflect"
)

// Null is the representation of a nullable column: a value
// plus a validity bit, never a pointer. The zero value is NULL. JSON shows
// the value or null. A valid Null[[]byte] is bytes, a nil V empty bytes,
// as a plain []byte is (spec §columns).
type Null[T any] struct {
	V     T
	Valid bool
}

// Some wraps a value in a valid Null.
func Some[T any](v T) Null[T] {
	return Null[T]{V: v, Valid: true}
}

// Get returns the value and whether it is present.
func (n Null[T]) Get() (T, bool) { return n.V, n.Valid }

// Or returns the value, or def when NULL.
func (n Null[T]) Or(def T) T {
	if n.Valid {
		return n.V
	}
	return def
}

// Scan implements sql.Scanner through database/sql's rules via sql.Null,
// but converts a value of bool kind itself unless the type implements
// sql.Scanner, since database/sql scans an integer into a bool and not
// into a named bool (spec §types), and reads a valid []byte through
// Bytes, never nil (spec §columns).
func (n *Null[T]) Scan(src any) error {
	if b, ok := any(&n.V).(*[]byte); ok && src != nil {
		if err := bytesScan(b, src); err != nil {
			return err
		}
		n.Valid = true
		return nil
	}
	_, scanner := any(&n.V).(sql.Scanner)
	if v := reflect.ValueOf(&n.V).Elem(); src != nil && v.Kind() == reflect.Bool && !scanner {
		b, err := driver.Bool.ConvertValue(src)
		if err != nil {
			return err
		}
		v.SetBool(b.(bool))
		n.Valid = true
		return nil
	}
	var s sql.Null[T]
	if err := s.Scan(src); err != nil {
		return err
	}
	n.V, n.Valid = s.V, s.Valid
	return nil
}

// Value implements driver.Valuer: the value, or nil for NULL; a valid
// []byte binds through Bytes, nil as empty bytes (spec §columns).
func (n Null[T]) Value() (driver.Value, error) {
	if b, ok := any(n.V).([]byte); ok && n.Valid {
		return Bytes(b).Value()
	}
	return sql.Null[T]{V: n.V, Valid: n.Valid}.Value()
}

var jsonNull = []byte("null")

// MarshalJSON writes the value, or null; a valid []byte is base64, nil
// as "" (spec §columns).
func (n Null[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return jsonNull, nil
	}
	return json.Marshal(bytesWrap(n.V))
}

// UnmarshalJSON reads null as NULL and anything else as the value.
func (n *Null[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), jsonNull) {
		*n = Null[T]{}
		return nil
	}
	if err := json.Unmarshal(data, &n.V); err != nil {
		return err
	}
	n.Valid = true
	return nil
}
