package itest

import (
	"database/sql/driver"
	"fmt"
)

// Money is a declared column type (spec §types): cents, stored as int.
type Money int64

// Scan implements sql.Scanner.
func (m *Money) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*m = Money(v)
	case nil:
		*m = 0
	default:
		return fmt.Errorf("money: cannot scan %T", src)
	}
	return nil
}

// Value implements driver.Valuer.
func (m Money) Value() (driver.Value, error) { return int64(m), nil }
