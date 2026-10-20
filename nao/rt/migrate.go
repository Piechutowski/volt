// The migration runtime (spec §migration-ledger, §migration-tooling):
// the ledger table, and up, down and status over the statement form of
// a package's migrations. The driver is the caller's, as everywhere in
// rt.
package rt

import (
	"context"
	"database/sql"
	"database/sql/driver"
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
	// by one, with the ledger write after the last, and disables the
	// foreign-key bracket (spec §migrations rule 13).
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
	ledgerExists = "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'volt_migrations'"
	ledgerSelect = "SELECT name, applied_at FROM volt_migrations"
	ledgerHasRow = "SELECT count(*) FROM volt_migrations WHERE name = ?"
	ledgerInsert = "INSERT INTO volt_migrations (name, applied_at) VALUES (?, ?)"
	ledgerDelete = "DELETE FROM volt_migrations WHERE name = ?"
)

// MigrateUp applies every pending migration in name order and returns
// the names applied, in order (spec §migration-ledger rule 2). Each
// migration runs under an immediate transaction that re-reads its
// ledger row first, so that two runners over one database apply it
// once. It stops at the first failure, which names the migration, the
// statement and its index, and returns what was applied before it.
func MigrateUp(ctx context.Context, db *sql.DB, migrations []Migration) (applied []string, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, ledgerDDL); err != nil {
		return nil, fmt.Errorf("rt: creating volt_migrations: %w", err)
	}
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
		skipped, err := migrationRun(ctx, conn, m, m.Up, false, ledgerInsert, m.Name, at)
		if err != nil {
			return applied, err
		}
		if !skipped {
			applied = append(applied, m.Name)
		}
	}
	return applied, nil
}

// MigrateDown reverts the applied migration with the greatest name by
// running its down statements and deleting its row, and returns its
// name (spec §migration-ledger rule 3). An irreversible migration, a
// ledger row no migration has, an empty ledger and a row another
// runner deleted first are errors.
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
	skipped, err := migrationRun(ctx, conn, *m, m.Down, true, ledgerDelete, m.Name)
	if err != nil {
		return "", err
	}
	if skipped {
		return "", fmt.Errorf("rt.MigrateDown: migration %s is no longer applied; another runner reverted it", last)
	}
	return last, nil
}

// MigrateStatus lists every migration of the package and every ledger
// row, in name order, with whether and when each was applied and
// whether a pending one is out of order (spec §migration-tooling rule
// 3). It only reads: a database without the ledger has nothing
// applied, and a read-only connection serves.
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
	var exists int
	if err := conn.QueryRowContext(ctx, ledgerExists).Scan(&exists); err != nil {
		return nil, fmt.Errorf("rt: reading volt_migrations: %w", err)
	}
	done := map[string]string{}
	if exists == 0 {
		return done, nil
	}
	rows, err := conn.QueryContext(ctx, ledgerSelect)
	if err != nil {
		return nil, fmt.Errorf("rt: reading volt_migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, at string
		if err := rows.Scan(&name, &at); err != nil {
			return nil, err
		}
		done[name] = at
	}
	return done, rows.Err()
}

func ledgerHas(ctx context.Context, conn *sql.Conn, name string) (bool, error) {
	var n int
	if err := conn.QueryRowContext(ctx, ledgerHasRow, name).Scan(&n); err != nil {
		return false, fmt.Errorf("migration %s: ledger: %w", name, err)
	}
	return n > 0, nil
}

// migrationRun runs one direction of m on conn and then the ledger
// statement: under one immediate transaction, or statement by statement
// with NoTransaction, inside the foreign-key bracket with ForeignKeysOff
// and no NoTransaction (spec §migrations rule 13). It re-reads the
// ledger row once the transaction holds the write lock, or right before
// the first statement, and skips the migration when the row is not as
// the direction expects: present before a down, absent before an up.
func migrationRun(ctx context.Context, conn *sql.Conn, m Migration, stmts []string, rowBefore bool, ledger string, args ...any) (skipped bool, err error) {
	if m.ForeignKeysOff && !m.NoTransaction {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return false, fmt.Errorf("migration %s: PRAGMA foreign_keys = OFF: %w", m.Name, err)
		}
		defer func() {
			if _, e := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); e != nil && err == nil {
				err = fmt.Errorf("migration %s: PRAGMA foreign_keys = ON: %w", m.Name, e)
			}
		}()
	}
	if m.NoTransaction {
		has, err := ledgerHas(ctx, conn, m.Name)
		if err != nil {
			return false, err
		}
		if has != rowBefore {
			return true, nil
		}
		if err := statementsRun(ctx, conn, m, stmts); err != nil {
			return false, err
		}
		if _, err := conn.ExecContext(ctx, ledger, args...); err != nil {
			return false, fmt.Errorf("migration %s: ledger: %w", m.Name, err)
		}
		return false, nil
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, fmt.Errorf("migration %s: %w", m.Name, err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if _, e := conn.ExecContext(context.Background(), "ROLLBACK"); e != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	has, err := ledgerHas(ctx, conn, m.Name)
	if err != nil {
		return false, err
	}
	if has != rowBefore {
		return true, nil
	}
	if err := statementsRun(ctx, conn, m, stmts); err != nil {
		return false, err
	}
	if m.ForeignKeysOff {
		if err := foreignKeyCheck(ctx, conn, m.Name); err != nil {
			return false, err
		}
	}
	if _, err := conn.ExecContext(ctx, ledger, args...); err != nil {
		return false, fmt.Errorf("migration %s: ledger: %w", m.Name, err)
	}
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("migration %s: commit: %w", m.Name, err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, fmt.Errorf("migration %s: commit: %w", m.Name, err)
	}
	committed = true
	return false, nil
}

func statementsRun(ctx context.Context, conn *sql.Conn, m Migration, stmts []string) error {
	for i, s := range stmts {
		if err := ctx.Err(); err != nil {
			return statementError(m.Name, i, s, err)
		}
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return statementError(m.Name, i, s, err)
		}
	}
	return nil
}

func statementError(name string, i int, stmt string, err error) error {
	return fmt.Errorf("migration %s: statement %d: %w\n  %s", name, i+1, err, stmt)
}

// foreignKeyCheck fails when PRAGMA foreign_key_check reports a row,
// naming the migration and the first offending table (spec §migrations
// rule 13).
func foreignKeyCheck(ctx context.Context, conn *sql.Conn, name string) error {
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
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
