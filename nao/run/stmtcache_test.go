package run

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
	if err := held.QueryRowContext(ctx).Scan(&n); err == nil || !strings.Contains(err.Error(), "statement is closed") {
		t.Fatalf("the evicted statement once released = %v; want it closed", err)
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
