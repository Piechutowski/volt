// Package pay is the package-qualified declared type of the integration
// fixture (spec §types): a Go type reached through a from path.
package pay

import (
	"database/sql/driver"
	"fmt"
)

// Cents is a declared column type (spec §types): an amount in cents,
// stored as int.
type Cents int

// Scan implements sql.Scanner on the pointer (spec §types).
func (c *Cents) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*c = Cents(v)
	case nil:
		*c = 0
	default:
		return fmt.Errorf("pay: cannot scan %T", src)
	}
	return nil
}

// Value implements driver.Valuer on the value (spec §types).
func (c Cents) Value() (driver.Value, error) { return int64(c), nil }
