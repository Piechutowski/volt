package rt

import (
	"errors"
	"fmt"
)

// Migration is the statement form of one migration (spec
// §migration-tooling): what volt migration sql prints and
// volt_migrations.go carries. The runner applies it.
type Migration struct {
	Name string
	// Up and Down are the statements of each direction; Down is nil
	// when the migration is irreversible (spec §migrations).
	Up, Down []string
	// NoTransaction runs the statements outside any transaction, one
	// by one, with the ledger write after the last, and disables the
	// foreign-key bracket (spec §migrations).
	NoTransaction bool
	// ForeignKeysOff brackets the migration with PRAGMA foreign_keys =
	// OFF, PRAGMA foreign_key_check before the commit and PRAGMA
	// foreign_keys = ON after (spec §migrations).
	ForeignKeysOff bool
}

// MigrationStatus is one row of the runner's status (spec
// §migration-tooling): a migration of the package, or a ledger row no
// migration has.
type MigrationStatus struct {
	Name      string
	Applied   bool
	AppliedAt string // RFC 3339 UTC; "" while pending
	// OutOfOrder marks a pending migration that sorts before an applied
	// one (spec §migration-ledger).
	OutOfOrder bool
	// Note is "not in the package" for a ledger row whose name no
	// migration has, and "" otherwise.
	Note string
}

// MigrationError is a migration that did not run (spec
// §migration-tooling): the statement SQLite refused, counted
// from one, with its text, or, at Index zero, a failed foreign-key
// check or a down asked of an irreversible migration, which Cause
// says. Any other error of the runner's up and down is the database.
type MigrationError struct {
	Name      string
	Direction string // "up" or "down"
	Index     int
	SQL       string
	Cause     error
}

func (e *MigrationError) Error() string {
	if e.Index == 0 {
		return fmt.Sprintf("migration %s: %v", e.Name, e.Cause)
	}
	return fmt.Sprintf("migration %s: statement %d: %v\n  %s", e.Name, e.Index, e.Cause, e.SQL)
}

// Unwrap returns the cause, SQLite's error for a refused statement.
func (e *MigrationError) Unwrap() error { return e.Cause }

// ErrIrreversible is the cause of the MigrationError the runner's down
// returns for a migration without down statements (spec
// §migration-ledger).
var ErrIrreversible = errors.New("irreversible, with no down to run")
