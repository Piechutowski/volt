package itest

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func pragmaRead(t *testing.T, db run.DB, name string) int {
	t.Helper()
	var v int
	if err := db.Pool().QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}

func orphanRefused(t *testing.T, db run.DB) {
	t.Helper()
	_, err := run.Run(context.Background(), db, OrderCreate(OrderNew().UserIDSet(999).TotalSet(1).FeeSet(0)))
	if err == nil {
		t.Fatal("an order of no user was stored; want foreign_keys = ON to refuse it")
	}
}

func slowly[T any](d time.Duration, e rt.Effect[T]) func(int64) rt.Effect[T] {
	return func(int64) rt.Effect[T] {
		time.Sleep(d)
		return e
	}
}

const slowWriteSQL = "UPDATE users SET name = name WHERE (WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 100000000) SELECT count(*) FROM c) > 0"

// TestTransactionTheContextEndsKeepsItsConnection proves the runner's
// transactions on the one connection run.Open pins (spec
// §generated-query-surface, the runner): a transaction its context
// ends between two statements, inside a write SQLite rolls back when
// interrupted, or beside a failing sibling of a top-level Parallel
// fails with the context's error and rolls back, and the connection
// stays, with an in-memory database's rows and foreign_keys = ON.
func TestTransactionTheContextEndsKeepsItsConnection(t *testing.T) {
	db := newDB(t)
	u := seedUser(t, db, "ann@example.com")
	ended := map[string]func(ctx context.Context) error{
		"between two statements": func(ctx context.Context) error {
			_, err := run.Run(ctx, db, rt.Atomic(rt.Then(UserCount(), slowly(50*time.Millisecond, UserCreate(UserNew().EmailSet("late@example.com").NameSet("Late"))))))
			return err
		},
		"inside an interrupted write": func(ctx context.Context) error {
			_, err := run.Run(ctx, db, rt.Atomic(rt.Then(UserCreate(UserNew().EmailSet("early@example.com").NameSet("Early")), func(User) rt.Effect[int64] {
				return rt.Exec{SQL: slowWriteSQL}
			})))
			return err
		},
		"in a plain Tx": func(ctx context.Context) error {
			_, err := run.Run(ctx, db, rt.Tx(UserCreate(UserNew().EmailSet("tx@example.com").NameSet("Tx")), rt.Exec{SQL: slowWriteSQL}))
			return err
		},
	}
	for name, perform := range ended {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		err := perform(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("a transaction its deadline ended %s = %v; want context.DeadlineExceeded", name, err)
		}
		if n, err := run.Run(context.Background(), db, UserCount()); err != nil || n != 1 {
			t.Fatalf("after a transaction its deadline ended %s, UserCount = %d, %v; want the 1 user committed before it", name, n, err)
		}
		if fk := pragmaRead(t, db, "foreign_keys"); fk != 1 {
			t.Fatalf("after a transaction its deadline ended %s, foreign_keys = %d; want 1", name, fk)
		}
	}
	sibling := errors.New("sibling")
	for range 20 {
		_, err := run.Run(context.Background(), db, rt.All(
			rt.Atomic(rt.Then(UserCount(), slowly(10*time.Millisecond, UserCreate(UserNew().EmailSet("beside@example.com").NameSet("Beside"))))),
			rt.Fail[int](sibling),
		))
		if !errors.Is(err, sibling) {
			t.Fatalf("All(Atomic, failing sibling) = %v; want the sibling's error", err)
		}
	}
	if n, err := run.Run(context.Background(), db, UserCount()); err != nil || n != 1 {
		t.Fatalf("after Atomics a sibling cancelled, UserCount = %d, %v; want 1", n, err)
	}
	if fk := pragmaRead(t, db, "foreign_keys"); fk != 1 {
		t.Fatalf("after Atomics a sibling cancelled, foreign_keys = %d; want 1", fk)
	}
	orphanRefused(t, db)
	if _, err := run.Run(context.Background(), db, OrderCreate(OrderNew().UserIDSet(u.ID).TotalSet(1).FeeSet(0))); err != nil {
		t.Fatalf("an order of the user after it all: %v", err)
	}
}

// TestOpenSetsThePragmasOnEveryConnection proves run.Open's pragmas
// (spec §generated-query-surface, the runner): a connection the pool
// opens after another closed carries them too, since each is set per
// connection.
func TestOpenSetsThePragmasOnEveryConnection(t *testing.T) {
	db, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Pool().SetMaxIdleConns(0)
	for i := range 3 {
		if fk, busy := pragmaRead(t, db, "foreign_keys"), pragmaRead(t, db, "busy_timeout"); fk != 1 || busy != 5000 {
			t.Fatalf("connection %d: foreign_keys = %d, busy_timeout = %d; want 1 and 5000", i, fk, busy)
		}
	}
	if n := db.Pool().Stats().OpenConnections; n != 0 {
		t.Fatalf("%d connections stayed open; want each closed, so that the test reads fresh ones", n)
	}
}

// TestAtomicCheckUnderAPool proves the check inside a transaction that
// spec §generated-query-surface spells, an Atomic over a Then, over a
// pool of several connections run.Wrap takes: twenty Atomics at once, each reading an
// order and shipping it only while pending, ship it once and refuse the
// rest with their Fail, none failing with database is locked, since a
// transaction begins immediately and waits for the write lock under the
// pool's busy_timeout.
func TestAtomicCheckUnderAPool(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pool.db")
	opened, err := run.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	pool, err := sql.Open("sqlite3", "file:"+path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(8)
	db := run.Wrap(pool)
	defer db.Close()
	ddlApply(t, db)
	u := seedUser(t, db, "ann@example.com")
	shipped := errors.New("already shipped")
	ship := func(id int) rt.Effect[Order] {
		return rt.Atomic(rt.Then(OrderGet(id), func(o Order) rt.Effect[Order] {
			if o.Status != OrderStatePending {
				return rt.Fail[Order](shipped)
			}
			return OrderUpdate(o.Changeset().StatusSet(OrderStateShipped))
		}))
	}
	for round := range 5 {
		o, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(u.ID).TotalSet(1).FeeSet(0)))
		if err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		outcomes := map[string]int{}
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				_, err := run.Run(ctx, db, ship(o.ID))
				key := "shipped"
				switch {
				case errors.Is(err, shipped):
					key = "refused"
				case err != nil:
					key = err.Error()
				}
				mu.Lock()
				outcomes[key]++
				mu.Unlock()
			})
		}
		wg.Wait()
		if outcomes["shipped"] != 1 || outcomes["refused"] != 19 {
			t.Fatalf("round %d: %v; want one shipped and 19 refused", round, outcomes)
		}
	}
}
