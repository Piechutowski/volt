package run

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/nao/rt"
)

type stmtCounts struct {
	mu       sync.Mutex
	prepared map[string]int
	closed   map[string]int
}

func (c *stmtCounts) count(m map[string]int, query string) {
	c.mu.Lock()
	m[query]++
	c.mu.Unlock()
}

func (c *stmtCounts) of(query string) (prepared, closed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prepared[query], c.closed[query]
}

type countingDriver struct{ counts *stmtCounts }

func (d countingDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite3.SQLiteDriver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return countingConn{conn: conn, counts: d.counts}, nil
}

type countingConn struct {
	conn   driver.Conn
	counts *stmtCounts
}

func (c countingConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	stmt, err := c.conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	c.counts.count(c.counts.prepared, query)
	return countingStmt{Stmt: stmt, query: query, counts: c.counts}, nil
}

func (c countingConn) Close() error              { return c.conn.Close() }
func (c countingConn) Begin() (driver.Tx, error) { return nil, errors.New("Begin is not used") }

type countingStmt struct {
	driver.Stmt
	query  string
	counts *stmtCounts
}

func (s countingStmt) Close() error {
	s.counts.count(s.counts.closed, s.query)
	return s.Stmt.Close()
}

func (s countingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
}

func (s countingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
}

var countingDrivers atomic.Int64

func countingOpen(t *testing.T) (DB, *stmtCounts) {
	t.Helper()
	counts := &stmtCounts{prepared: map[string]int{}, closed: map[string]int{}}
	name := fmt.Sprintf("sqlite3_counting_%d", countingDrivers.Add(1))
	sql.Register(name, countingDriver{counts: counts})
	db := migrateDBAt(t, name, filepath.Join(t.TempDir(), "count.db"))
	if _, err := db.Pool().Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL UNIQUE)"); err != nil {
		t.Fatal(err)
	}
	return db, counts
}

const postInsert = "INSERT INTO posts (title) VALUES (?)"

func postInserts(n, from int) []rt.AnyEffect {
	steps := make([]rt.AnyEffect, n)
	for i := range steps {
		steps[i] = rt.Exec{SQL: postInsert, Args: []any{fmt.Sprintf("post %d", from+i)}}
	}
	return steps
}

// TestTransactionPreparesEachStatementOnce proves the statements of a
// transaction (spec §generated-query-surface, the runner): a
// transaction of a thousand identical inserts prepares the insert once
// on its connection and closes it before it ends, the next transaction
// prepares it once again, and every row commits.
func TestTransactionPreparesEachStatementOnce(t *testing.T) {
	ctx := context.Background()
	db, counts := countingOpen(t)
	for round := range 2 {
		if _, err := Run(ctx, db, rt.Tx(postInserts(1000, round*1000)...)); err != nil {
			t.Fatal(err)
		}
		if prepared, closed := counts.of(postInsert); prepared != round+1 || closed != round+1 {
			t.Fatalf("after transaction %d the insert was prepared %d and closed %d times; want %d each", round+1, prepared, closed, round+1)
		}
	}
	n, err := Run(ctx, db, rt.Lookup[int]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int]})
	if err != nil || n != 2000 {
		t.Fatalf("count = %d, %v; want 2000", n, err)
	}
}

// TestTransactionClosesItsStatementsWhateverItsEnd proves that a
// transaction's statements close however it ends (spec
// §generated-query-surface, the runner): one that a step fails, one that
// a constraint refuses, and one whose step panics roll back, close the
// insert they prepared, and leave nothing behind.
func TestTransactionClosesItsStatementsWhateverItsEnd(t *testing.T) {
	ctx := context.Background()
	db, counts := countingOpen(t)
	failed := errors.New("failed")
	ends := map[string]rt.AnyEffect{
		"a failing step": rt.Tx(append(postInserts(3, 0), rt.Fail[int](failed))...),
		"a refused row":  rt.Tx(append(postInserts(3, 0), postInserts(1, 0)...)...),
		"a panic": rt.Tx(append(postInserts(3, 0), rt.Then(rt.Pure(1), func(int) rt.Effect[int] {
			panic("step panicked")
		}))...),
	}
	want := 0
	for name, e := range ends {
		func() {
			defer func() { _ = recover() }()
			if _, err := Perform(ctx, db, e); err == nil {
				t.Errorf("%s: the transaction succeeded", name)
			}
		}()
		want++
		if prepared, closed := counts.of(postInsert); prepared != want || closed != want {
			t.Errorf("after %s the insert was prepared %d and closed %d times; want %d each", name, prepared, closed, want)
		}
	}
	n, err := Run(ctx, db, rt.Lookup[int]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int]})
	if err != nil || n != 0 {
		t.Fatalf("count = %d, %v; want every transaction rolled back", n, err)
	}
}

// TestTransactionBeyondItsStatementCap proves the cap on a transaction's
// statements (spec §generated-query-surface, the runner): past 64
// distinct texts, a statement runs prepared for itself alone and the
// transaction still commits every row.
func TestTransactionBeyondItsStatementCap(t *testing.T) {
	ctx := context.Background()
	db, counts := countingOpen(t)
	steps := make([]rt.AnyEffect, 100)
	texts := make([]string, len(steps))
	for i := range steps {
		texts[i] = postInsert + strings.Repeat(" ", i)
		steps[i] = rt.Exec{SQL: texts[i], Args: []any{fmt.Sprintf("post %d", i)}}
	}
	if _, err := Run(ctx, db, rt.Tx(steps...)); err != nil {
		t.Fatal(err)
	}
	for i, text := range texts {
		if prepared, closed := counts.of(text); prepared != 1 || closed != 1 {
			t.Errorf("statement %d prepared %d and closed %d times; want once each", i, prepared, closed)
		}
	}
	n, err := Run(ctx, db, rt.Lookup[int]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int]})
	if err != nil || n != 100 {
		t.Fatalf("count = %d, %v; want 100", n, err)
	}
}
