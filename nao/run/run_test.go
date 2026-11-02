package run

import (
	"context"
	"errors"
	"log/slog"
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
