package models

import (
	"database/sql/driver"
	"fmt"

	"github.com/Piechutowski/volt/nao/rt"
)

// Visibility is a declared column type stored as string, its values the
// constants below (spec §types).
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
)

// VisibilityKnown refuses a value outside the constants of Visibility
// (spec §validators).
func VisibilityKnown(v Visibility) error {
	switch v {
	case VisibilityPublic, VisibilityUnlisted:
		return nil
	}
	return fmt.Errorf("%q is not a known visibility", string(v))
}

// VisibilityKnownNull is VisibilityKnown for a nullable column, which NULL
// passes (spec §validators).
func VisibilityKnownNull(v rt.Null[Visibility]) error {
	if !v.Valid {
		return nil
	}
	return VisibilityKnown(v.V)
}

// Tier is a declared column type stored as int, an alias of its storage
// type (spec §types).
type Tier = int

// Cents is a declared column type stored as int whose Go declaration
// spells int64, so it implements Scanner and Valuer (spec §types).
type Cents int64

// Scan implements sql.Scanner on the pointer (spec §types).
func (c *Cents) Scan(src any) error {
	v, ok := src.(int64)
	if !ok {
		return fmt.Errorf("cents: cannot scan %T", src)
	}
	*c = Cents(v)
	return nil
}

// Value implements driver.Valuer on the value (spec §types).
func (c Cents) Value() (driver.Value, error) { return int64(c), nil }
