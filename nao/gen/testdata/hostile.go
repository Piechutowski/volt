package models

import "fmt"

// CoreAuditKind is a declared column type stored as string, its values the
// constants below (spec §types).
type CoreAuditKind string

const (
	// CoreAuditKindReadWrite is a value with a space.
	CoreAuditKindReadWrite CoreAuditKind = "read write"
	CoreAuditKindAdmin     CoreAuditKind = "admin"
)

// CoreAuditKindKnown refuses a value outside the constants of CoreAuditKind
// (spec §validators).
func CoreAuditKindKnown(v CoreAuditKind) error {
	switch v {
	case CoreAuditKindReadWrite, CoreAuditKindAdmin:
		return nil
	}
	return fmt.Errorf("%q is not a known core audit kind", string(v))
}
