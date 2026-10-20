// The migration runtime (spec §migration-ledger, §migration-tooling):
// the ledger table, and up, down and status over the statement form of
// a package's migrations. The driver is the caller's, as everywhere in
// rt.
package rt

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration is the statement form of one migration (spec
// §migration-tooling rule 3): what volt migration sql prints and
// volt_migrations.go carries.
type Migration struct {
	Name string
	// Up and Down are the statements of each direction; Down is nil
	// when the migration is irreversible (spec §migrations rule 12).
	Up, Down []string
	// NoTransaction runs the statements outside any transaction, one
	// by one, with the ledger write after the last (spec §migrations
	// rule 13).
	NoTransaction bool
	// ForeignKeysOff brackets the migration with PRAGMA foreign_keys =
	// OFF, PRAGMA foreign_key_check before the commit and PRAGMA
	// foreign_keys = ON after (spec §migrations rule 13).
	ForeignKeysOff bool
}

// MigrationStatus is one row of MigrateStatus (spec §migration-tooling
// rule 3): a migration of the package, or a ledger row no migration
// has.
type MigrationStatus struct {
	Name      string
	Applied   bool
	AppliedAt string // RFC 3339 UTC; "" while pending
	// OutOfOrder marks a pending migration that sorts before an applied
	// one (spec §migration-ledger rule 2).
	OutOfOrder bool
	// Note is "not in the package" for a ledger row whose name no
	// migration has, and "" otherwise.
	Note string
}

const (
	ledgerDDL    = "CREATE TABLE IF NOT EXISTS volt_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)"
	ledgerSelect = "SELECT name, applied_at FROM volt_migrations"
	ledgerInsert = "INSERT INTO volt_migrations (name, applied_at) VALUES (?, ?)"
	ledgerDelete = "DELETE FROM volt_migrations WHERE name = ?"
)

// MigrateUp applies every pending migration in name order and returns
// the names applied, in order (spec §migration-ledger rule 2). It stops
// at the first failure, which names the migration, the statement and
// its index, and returns what was applied before it.
func MigrateUp(ctx context.Context, db *sql.DB, migrations []Migration) (applied []string, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	done, err := ledgerRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	pending := make([]Migration, 0, len(migrations))
	for _, m := range migrations {
		if _, ok := done[m.Name]; !ok {
			pending = append(pending, m)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].Name < pending[j].Name })
	for _, m := range pending {
		at := time.Now().UTC().Format(time.RFC3339)
		if err := migrationRun(ctx, conn, m, m.Up, ledgerInsert, m.Name, at); err != nil {
			return applied, err
		}
		applied = append(applied, m.Name)
	}
	return applied, nil
}

// MigrateDown reverts the applied migration with the greatest name by
// running its down statements and deleting its row, and returns its
// name (spec §migration-ledger rule 3). An irreversible migration, a
// ledger row no migration has and an empty ledger are errors.
func MigrateDown(ctx context.Context, db *sql.DB, migrations []Migration) (name string, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	done, err := ledgerRead(ctx, conn)
	if err != nil {
		return "", err
	}
	last := ""
	for n := range done {
		if n > last {
			last = n
		}
	}
	if last == "" {
		return "", fmt.Errorf("rt.MigrateDown: no migration is applied")
	}
	var m *Migration
	for i := range migrations {
		if migrations[i].Name == last {
			m = &migrations[i]
			break
		}
	}
	switch {
	case m == nil:
		return "", fmt.Errorf("rt.MigrateDown: migration %s is applied but not in the package", last)
	case m.Down == nil:
		return "", fmt.Errorf("rt.MigrateDown: migration %s is irreversible", last)
	}
	if err := migrationRun(ctx, conn, *m, m.Down, ledgerDelete, m.Name); err != nil {
		return "", err
	}
	return last, nil
}

// MigrateStatus lists every migration of the package and every ledger
// row, in name order, with whether and when each was applied and
// whether a pending one is out of order (spec §migration-tooling rule
// 3).
func MigrateStatus(ctx context.Context, db *sql.DB, migrations []Migration) ([]MigrationStatus, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	done, err := ledgerRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(migrations))
	names := make([]string, 0, len(migrations)+len(done))
	for _, m := range migrations {
		if !known[m.Name] {
			known[m.Name] = true
			names = append(names, m.Name)
		}
	}
	last := ""
	for n := range done {
		if !known[n] {
			names = append(names, n)
		}
		if n > last {
			last = n
		}
	}
	sort.Strings(names)
	out := make([]MigrationStatus, 0, len(names))
	for _, n := range names {
		at, applied := done[n]
		s := MigrationStatus{Name: n, Applied: applied, AppliedAt: at}
		if !applied && n < last {
			s.OutOfOrder = true
		}
		if !known[n] {
			s.Note = "not in the package"
		}
		out = append(out, s)
	}
	return out, nil
}

func ledgerRead(ctx context.Context, conn *sql.Conn) (map[string]string, error) {
	if _, err := conn.ExecContext(ctx, ledgerDDL); err != nil {
		return nil, fmt.Errorf("rt: creating volt_migrations: %w", err)
	}
	rows, err := conn.QueryContext(ctx, ledgerSelect)
	if err != nil {
		return nil, fmt.Errorf("rt: reading volt_migrations: %w", err)
	}
	defer rows.Close()
	done := map[string]string{}
	for rows.Next() {
		var name, at string
		if err := rows.Scan(&name, &at); err != nil {
			return nil, err
		}
		done[name] = at
	}
	return done, rows.Err()
}

// migrationRun runs one direction of m on conn and then the ledger
// statement: in one transaction, or statement by statement with
// NoTransaction, inside the foreign-key bracket with ForeignKeysOff
// (spec §migrations rule 13).
func migrationRun(ctx context.Context, conn *sql.Conn, m Migration, stmts []string, ledger string, args ...any) (err error) {
	if m.ForeignKeysOff {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("migration %s: PRAGMA foreign_keys = OFF: %w", m.Name, err)
		}
		defer func() {
			if _, e := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); e != nil && err == nil {
				err = fmt.Errorf("migration %s: PRAGMA foreign_keys = ON: %w", m.Name, e)
			}
		}()
	}
	if m.NoTransaction {
		for i, s := range stmts {
			if _, err := conn.ExecContext(ctx, s); err != nil {
				return statementError(m.Name, i, s, err)
			}
		}
		if m.ForeignKeysOff {
			if err := foreignKeyCheck(ctx, conn, m.Name); err != nil {
				return err
			}
		}
		if _, err := conn.ExecContext(ctx, ledger, args...); err != nil {
			return fmt.Errorf("migration %s: ledger: %w", m.Name, err)
		}
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %s: %w", m.Name, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for i, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return statementError(m.Name, i, s, err)
		}
	}
	if m.ForeignKeysOff {
		if err := foreignKeyCheck(ctx, tx, m.Name); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, ledger, args...); err != nil {
		return fmt.Errorf("migration %s: ledger: %w", m.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %s: commit: %w", m.Name, err)
	}
	committed = true
	return nil
}

func statementError(name string, i int, stmt string, err error) error {
	return fmt.Errorf("migration %s: statement %d: %w\n  %s", name, i+1, err, stmt)
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// foreignKeyCheck fails when PRAGMA foreign_key_check reports a row,
// naming the migration and the first offending table (spec §migrations
// rule 13).
func foreignKeyCheck(ctx context.Context, q querier, name string) error {
	rows, err := q.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("migration %s: PRAGMA foreign_key_check: %w", name, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return rows.Err()
	}
	var table string
	var rowid, parent, fkid any
	if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
		return err
	}
	return fmt.Errorf("migration %s: foreign_key_check failed: table %s has a row whose foreign key references nothing", name, table)
}
