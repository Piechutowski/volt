package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

// TestStmtCacheEvictsAStatementInUse proves that an eviction closes a
// statement only once the caller holding it released it: the statement
// a second Prepare evicted still runs, and its release closes it.
func TestStmtCacheEvictsAStatementInUse(t *testing.T) {
	ctx := context.Background()
	pool, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cache := NewStmtCache(1)
	defer cache.Close()
	held, release, err := cache.Prepare(ctx, pool, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := cache.Prepare(ctx, pool, "SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	other()
	if cache.Len() != 1 {
		t.Fatalf("the cache holds %d statements; want 1, the first evicted", cache.Len())
	}
	var n int
	if err := held.QueryRowContext(ctx).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the evicted statement still held = %d, %v; want it to run", n, err)
	}
	release()
	release()
	stmtClosedWait(t, held, "the evicted statement once released")
}

func stmtClosed(stmt *sql.Stmt) error {
	var n int
	err := stmt.QueryRowContext(context.Background()).Scan(&n)
	if err != nil && strings.Contains(err.Error(), "statement is closed") {
		return nil
	}
	return fmt.Errorf("read %d, %v; want it closed", n, err)
}

func stmtClosedWait(t *testing.T, stmt *sql.Stmt, name string) {
	t.Helper()
	err := stmtClosed(stmt)
	for start := time.Now(); err != nil && time.Since(start) < 5*time.Second; err = stmtClosed(stmt) {
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		t.Fatalf("%s after 5s: %v", name, err)
	}
}

func stepHold(t *testing.T, pool *sql.DB) (stop func()) {
	t.Helper()
	stepping := make(chan struct{})
	var once sync.Once
	cancelHook.Lock()
	cancelHook.fn = func() { once.Do(func() { close(stepping) }) }
	cancelHook.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	var err error
	stepped := make(chan struct{})
	go func() {
		defer close(stepped)
		_, err = pool.ExecContext(ctx, "SELECT count(*) FROM (WITH RECURSIVE c(x) AS (SELECT cancel_ctx() UNION ALL SELECT x + 1 FROM c WHERE x < 4000000000) SELECT x FROM c)")
	}()
	interrupt := func() {
		cancel()
		<-stepped
		cancelHook.Lock()
		cancelHook.fn = nil
		cancelHook.Unlock()
	}
	t.Cleanup(interrupt)
	select {
	case <-stepping:
	case <-time.After(5 * time.Second):
		t.Fatal("the long statement did not start in 5s")
	}
	return func() {
		t.Helper()
		interrupt()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the long statement = %v; want it still stepping until interrupted", err)
		}
	}
}

func within(t *testing.T, name string, f func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- f() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s waited 2s for a step on another connection", name)
	}
}

func poolCancelOpen(t *testing.T, conns int) *sql.DB {
	t.Helper()
	pool, err := sql.Open("sqlite3_cancel", "file:"+filepath.Join(t.TempDir(), "c.db")+"?_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(conns)
	t.Cleanup(func() { pool.Close() })
	return pool
}

// TestStmtCacheClosesApartFromItsCallers proves that closing a statement
// holds up none of the cache's callers: while one connection of the pool
// steps a long statement, the Prepare that evicts a statement prepared
// there, the release that closes one, and a cache hit run on the other
// connection return at once, where a close waits for that step, and
// under the cache's mutex made every Prepare wait with it; the cache's
// Close waits for the two statements to close, which they do once the
// step ends.
func TestStmtCacheClosesApartFromItsCallers(t *testing.T) {
	ctx := context.Background()
	pool := poolCancelOpen(t, 2)
	cache := NewStmtCache(2)
	t.Cleanup(func() { cache.Close() })
	idle, release, err := cache.Prepare(ctx, pool, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	release()
	held, releaseHeld, err := cache.Prepare(ctx, pool, "SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	stop := stepHold(t, pool)
	prepared := func(query string, want int) error {
		stmt, release, err := cache.Prepare(ctx, pool, query)
		if err != nil {
			return err
		}
		defer release()
		var n int
		if err := stmt.QueryRowContext(ctx).Scan(&n); err != nil || n != want {
			return fmt.Errorf("%s read %d, %v", query, n, err)
		}
		return nil
	}
	within(t, "the Prepare that evicts an idle statement", func() error { return prepared("SELECT 3", 3) })
	within(t, "the Prepare that evicts a held statement", func() error { return prepared("SELECT 4", 4) })
	within(t, "the release that closes it", func() error {
		releaseHeld()
		return nil
	})
	within(t, "a cache hit", func() error { return prepared("SELECT 4", 4) })
	if cache.Len() != 2 {
		t.Fatalf("the cache holds %d statements; want 2", cache.Len())
	}
	closing := make(chan error, 1)
	go func() { closing <- cache.Close() }()
	select {
	case <-closing:
		t.Fatal("the cache's Close returned while the statements it evicted waited for the step to close")
	case <-time.After(100 * time.Millisecond):
	}
	stop()
	select {
	case err := <-closing:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cache's Close did not return in 5s after the step ended")
	}
	if err := stmtClosed(idle); err != nil {
		t.Fatalf("the idle statement evicted during the step, after Close: %v", err)
	}
	if err := stmtClosed(held); err != nil {
		t.Fatalf("the held statement released during the step, after Close: %v", err)
	}
}

// TestQueryOfAnEvictedStatementClosesApart proves the runner's Query
// against the same wait (spec §generated-query-surface, the runner): a
// statement evicted while its rows are read, and prepared also on a
// connection stepping a long statement, stays open until the request
// closed its rows and closes after, so the request returns at once with
// what it read, where the close the rows ran waited for the step.
func TestQueryOfAnEvictedStatementClosesApart(t *testing.T) {
	ctx := context.Background()
	pool := poolCancelOpen(t, 3)
	db := DB{pool: pool, cache: NewStmtCache(1), log: slog.Default()}
	t.Cleanup(func() { db.cache.Close() })
	read := func(evict bool) rt.Query[int64] {
		return rt.Query[int64]{SQL: "SELECT 1 UNION ALL SELECT 2", Scan: func(r rt.Row) (int64, error) {
			if evict {
				evict = false
				if _, err := Run(ctx, db, rt.Exec{SQL: "SELECT 3"}); err != nil {
					return 0, err
				}
				time.Sleep(50 * time.Millisecond)
			}
			var n int64
			err := r.Scan(&n)
			return n, err
		}}
	}
	if got, err := Run(ctx, db, read(false)); err != nil || len(got) != 2 {
		t.Fatalf("the read = %v, %v", got, err)
	}
	stmt, release, err := db.cache.Prepare(ctx, pool, "SELECT 1 UNION ALL SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	release()
	stop := stepHold(t, pool)
	within(t, "a Query whose statement is evicted while its rows are read", func() error {
		got, err := Run(ctx, db, read(true))
		if err == nil && (len(got) != 2 || got[0] != 1 || got[1] != 2) {
			err = fmt.Errorf("read %v", got)
		}
		return err
	})
	if db.cache.Len() != 1 {
		t.Fatalf("the cache holds %d statements; want 1", db.cache.Len())
	}
	stop()
	stmtClosedWait(t, stmt, "the statement evicted while its rows were read")
}

// TestStmtCacheReleaseOfAFailedPrepare proves that every release Prepare
// returns can be called: after a refused prepare and on a closed cache
// it does nothing, so a caller that defers it before the error check
// does not panic.
func TestStmtCacheReleaseOfAFailedPrepare(t *testing.T) {
	ctx := context.Background()
	pool, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cache := NewStmtCache(1)
	_, release, err := cache.Prepare(ctx, pool, "SELEC 1")
	if err == nil {
		t.Fatal("a prepare of no statement succeeded")
	}
	release()
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	_, release, err = cache.Prepare(ctx, pool, "SELECT 1")
	if err == nil {
		t.Fatal("a Prepare on a closed cache succeeded")
	}
	release()
	if cache.Len() != 0 {
		t.Fatalf("the closed cache holds %d statements", cache.Len())
	}
}

// TestStmtCacheEvictionUnderLoad proves the runner's statements outside
// a transaction against an eviction on another goroutine (spec
// §generated-query-surface, the runner): many goroutines running more
// statement texts than the cache holds all succeed.
func TestStmtCacheEvictionUnderLoad(t *testing.T) {
	ctx := context.Background()
	opened := migrateDB(t)
	opened.Pool().SetMaxOpenConns(8)
	db := DB{pool: opened.pool, cache: NewStmtCache(4), log: opened.log}
	defer db.cache.Close()
	var mu sync.Mutex
	failures := map[string]int{}
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for i := range 300 {
				shape := (g*31 + i) % 40
				got, err := Run(ctx, db, rt.Lookup[int64]{SQL: fmt.Sprintf("SELECT %d", shape), Scan: rt.ScanOne[int64]})
				if err == nil && got != int64(shape) {
					err = fmt.Errorf("SELECT %d read %d", shape, got)
				}
				if err != nil {
					mu.Lock()
					failures[err.Error()]++
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	if len(failures) != 0 {
		t.Fatalf("lookups over an evicting cache failed: %v", failures)
	}
}
