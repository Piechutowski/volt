package rt

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// Bytes is the value of a []byte column (spec §columns): bytes held in a
// string, so nothing can change them and a row, a changeset and an
// effect share them freely. It has one empty value, which binds as empty
// bytes and never NULL. Bytes(b) copies b in and Bytes() copies it out.
// Every plain []byte the dynamic layer binds, a Raw argument and an
// After key among them, binds through Bytes; a declared type stored as
// []byte binds through its own driver.Valuer instead.
type Bytes string

// Bytes is a copy of the bytes, never nil.
func (b Bytes) Bytes() []byte { return []byte(b) }

// Value binds the bytes, empty as empty bytes, never NULL (spec
// §columns).
func (b Bytes) Value() (driver.Value, error) { return []byte(b), nil }

// Scan reads a value through database/sql's rules into bytes of its own;
// NULL is an error, as it is for every NOT NULL column (spec §columns).
func (b *Bytes) Scan(src any) error {
	var n sql.Null[string]
	if err := n.Scan(src); err != nil {
		return err
	}
	if !n.Valid {
		return errBytesNull
	}
	*b = Bytes(n.V)
	return nil
}

// MarshalJSON writes the bytes as base64, as encoding/json writes a
// []byte, and empty as "" (spec §columns).
func (b Bytes) MarshalJSON() ([]byte, error) { return json.Marshal([]byte(b)) }

// UnmarshalJSON reads base64 as encoding/json reads a []byte; null leaves
// b as it was, as it leaves every value.
func (b *Bytes) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), jsonNull) {
		return nil
	}
	var v []byte
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = Bytes(v)
	return nil
}

var errBytesNull = errors.New("rt: converting NULL to bytes is unsupported")

func bytesWrap(v any) any {
	if b, ok := v.([]byte); ok {
		return Bytes(b)
	}
	return v
}

func argsAppend(args []any, vals ...any) []any {
	for _, v := range vals {
		args = append(args, bytesWrap(v))
	}
	return args
}
