package rt

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// Bytes is a plain []byte as generated code and the runtime bind, scan
// and render it (spec §columns rule 2): nil and empty are one value,
// empty bytes, and a plain []byte is never NULL. Every plain []byte the
// dynamic layer binds, a Raw argument and an After key among them, binds
// through Bytes; a declared type stored as []byte binds through its own
// driver.Valuer instead.
type Bytes []byte

// Value binds the bytes, nil as empty bytes, never NULL (spec §columns
// rule 2).
func (b Bytes) Value() (driver.Value, error) {
	if b == nil {
		return []byte{}, nil
	}
	return []byte(b), nil
}

// Scan reads a value through database/sql's rules into bytes that are
// never nil, empty bytes a driver hands back as nil among them; NULL is
// an error, as it is for every NOT NULL column (spec §columns rule 2).
func (b *Bytes) Scan(src any) error {
	return bytesScan((*[]byte)(b), src)
}

// MarshalJSON writes the bytes as base64, nil as "" like empty bytes
// (spec §columns rule 2).
func (b Bytes) MarshalJSON() ([]byte, error) {
	if b == nil {
		b = Bytes{}
	}
	return json.Marshal([]byte(b))
}

var errBytesNull = errors.New("rt: converting NULL to []byte is unsupported")

func bytesScan(dst *[]byte, src any) error {
	var n sql.Null[[]byte]
	if err := n.Scan(src); err != nil {
		return err
	}
	if !n.Valid {
		return errBytesNull
	}
	if n.V == nil {
		n.V = []byte{}
	}
	*dst = n.V
	return nil
}

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
