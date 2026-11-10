package run

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
)

func TestRunLogsEachStatementAndMapsAMiss(t *testing.T) {
	ctx := context.Background()
	var log strings.Builder
	db := migrateDB(t).WithLogger(slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if _, err := db.Pool().Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	scan := func(r rt.Row) (string, error) {
		var s string
		err := r.Scan(&s)
		return s, err
	}
	if _, err := Run(ctx, db, rt.Exec{SQL: "INSERT INTO posts (title) VALUES (?)", Args: []any{"one"}}); err != nil {
		t.Fatal(err)
	}
	got, err := Run(ctx, db, rt.Query[string]{SQL: "SELECT title FROM posts", Scan: scan})
	if err != nil || len(got) != 1 || got[0] != "one" {
		t.Fatalf("Query = %v, %v", got, err)
	}
	if _, err := Run(ctx, db, rt.Lookup[string]{SQL: "SELECT title FROM posts WHERE id = ?", Args: []any{9}, Scan: scan}); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("a miss = %v; want rt.ErrNotFound", err)
	}
	if n := strings.Count(log.String(), "msg=statement"); n != 3 {
		t.Errorf("%d statement lines logged, want 3:\n%s", n, log.String())
	}
	if !strings.Contains(log.String(), "duration=") || !strings.Contains(log.String(), `sql="SELECT title FROM posts"`) {
		t.Errorf("a statement line lacks its text or duration:\n%s", log.String())
	}
	if db.Statements() != 3 {
		t.Errorf("%d statements cached, want 3", db.Statements())
	}
}

func TestRunTransactionRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	if _, err := db.Pool().Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL UNIQUE)"); err != nil {
		t.Fatal(err)
	}
	insert := func(title string) rt.Exec {
		return rt.Exec{SQL: "INSERT INTO posts (title) VALUES (?)", Args: []any{title}, Model: "Post"}
	}
	var inv *rt.Invalid
	if _, err := Run(ctx, db, rt.Tx(insert("a"), insert("a"))); !errors.As(err, &inv) || inv.Fields[0].Check != "unique" {
		t.Fatalf("Tx with a duplicate = %v; want the unique refusal", err)
	}
	count := rt.Lookup[int64]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int64]}
	if n, err := Run(ctx, db, count); err != nil || n != 0 {
		t.Errorf("after the rollback count = %d, %v; want 0", n, err)
	}
	out, err := Run(ctx, db, rt.Tx(insert("a"), rt.All(insert("b"), insert("c")), count))
	if err != nil || len(out) != 3 || out[2] != int64(3) {
		t.Fatalf("Tx with a parallel step = %v, %v; want three rows counted inside the transaction", out, err)
	}
}

func TestOpenRefusesAnInMemoryDatabase(t *testing.T) {
	for _, dsn := range []string{":memory:", "file::memory:?cache=shared", "file:blog.db?mode=memory"} {
		if _, err := Open("sqlite3", dsn); err == nil || !strings.Contains(err.Error(), "holds nothing") {
			t.Errorf("Open(%q) = %v; want the in-memory refusal", dsn, err)
		}
		if _, err := OpenReadOnly("sqlite3", dsn); err == nil || !strings.Contains(err.Error(), "holds nothing") {
			t.Errorf("OpenReadOnly(%q) = %v; want the in-memory refusal", dsn, err)
		}
	}
	if err := MemoryRefused(filepath.Join(t.TempDir(), "blog.db")); err != nil {
		t.Errorf("MemoryRefused of a file = %v", err)
	}
}

func TestOpenReadOnlyRefusesWritesAndKeepsTheJournalMode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plain.db")
	plain, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	plain.Close()
	db, err := OpenReadOnly("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Run(ctx, db, rt.Exec{SQL: "INSERT INTO posts (title) VALUES ('one')"}); err == nil {
		t.Error("a write through OpenReadOnly succeeded")
	}
	n, err := Run(ctx, db, rt.Lookup[int]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int]})
	if err != nil || n != 0 {
		t.Errorf("a read through OpenReadOnly = %d, %v", n, err)
	}
	var mode string
	if err := db.Pool().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("journal_mode %q %v; want the file's delete kept", mode, err)
	}
}
