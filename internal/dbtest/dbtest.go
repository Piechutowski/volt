// Package dbtest opens the SQLite database a test runs against: a file
// in the test's temporary directory, opened through run.Open as
// production opens one (spec §generated-query-surface). It registers
// no driver; the test imports github.com/mattn/go-sqlite3 itself.
package dbtest

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/nao/run"
)

// Open is a database in t's temporary directory with ddl applied,
// closed when t ends.
func Open(t testing.TB, ddl string) run.DB {
	t.Helper()
	return OpenDriver(t, "sqlite3", ddl)
}

// OpenDriver is Open through the database/sql driver registered as
// driverName, for a test that wraps mattn's.
func OpenDriver(t testing.TB, driverName, ddl string) run.DB {
	t.Helper()
	db, err := run.Open(driverName, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if ddl != "" {
		if _, err := db.Pool().Exec(ddl); err != nil {
			t.Fatalf("DDL rejected by SQLite: %v", err)
		}
	}
	return db
}

// StatementPrepares reports whether query compiles against db's schema
// and takes exactly n arguments, running nothing: SQLite explains it
// with every argument NULL.
func StatementPrepares(db run.DB, query string, n int) error {
	ctx := context.Background()
	stmt, err := db.Pool().PrepareContext(ctx, "EXPLAIN "+query)
	if err == nil {
		_, err = stmt.ExecContext(ctx, make([]any, n)...)
		stmt.Close()
	}
	if err != nil {
		return fmt.Errorf("%w\n  %s", err, query)
	}
	return nil
}
