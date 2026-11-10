package itest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Piechutowski/volt/internal/dbtest"
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
// stays, with its rows and foreign_keys = ON.
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
// spec §generated-query-surface spells, an Atomic over a Then, over
// several DBs on one file, as several processes hold it: twenty Atomics
// at once, each reading an order and shipping it only while pending,
// ship it once and refuse the rest with their Fail, none failing with
// database is locked, since a transaction begins immediately and waits
// for the write lock under the busy timeout.
func TestAtomicCheckUnderAPool(t *testing.T) {
	ctx := context.Background()
	dbs := dbsOpen(t, 4)
	db := dbs[0]
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
		for i := range 20 {
			wg.Go(func() {
				_, err := run.Run(ctx, dbs[i%len(dbs)], ship(o.ID))
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

func dbsOpen(t *testing.T, n int) []run.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shared.db")
	dbs := make([]run.DB, n)
	for i := range dbs {
		db, err := run.Open("sqlite3", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		dbs[i] = db
	}
	ddlApply(t, dbs[0])
	return dbs
}

func writeLockHold(t *testing.T, db run.DB) {
	t.Helper()
	conn, err := db.Pool().Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	})
}

// TestReadTransactionBesideAWriter proves that a transaction of reads
// alone begins deferred (spec §generated-query-surface, the runner): a
// With, and an Atomic and a Transaction of reads, read their snapshot
// at once while another DB on the file holds the write lock, where an
// immediate begin would wait for it and fail with database is locked
// after the busy timeout.
func TestReadTransactionBesideAWriter(t *testing.T) {
	ctx := context.Background()
	dbs := dbsOpen(t, 2)
	db := dbs[0]
	seedUser(t, db, "ann@example.com")
	writeLockHold(t, dbs[1])
	start := time.Now()
	if got, err := run.Run(ctx, db, UserEveryone()); err != nil || len(got) != 1 {
		t.Fatalf("a with select beside a writer = %v, %v", got, err)
	}
	if n, err := run.Run(ctx, db, rt.Atomic(UserCount())); err != nil || n != 1 {
		t.Fatalf("an Atomic of a read beside a writer = %d, %v", n, err)
	}
	if out, err := run.Run(ctx, db, rt.Tx(UserCount(), UserList())); err != nil || len(out) != 2 {
		t.Fatalf("a Tx of reads beside a writer = %v, %v", out, err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("the reads beside a writer took %v; want them not to wait for its lock", d)
	}
}

// TestWriteBeginEndsWithItsContext proves the wait of an immediate
// begin (spec §generated-query-surface, the runner): while another
// connection holds the write lock, a transaction that writes waits for
// it until its context ends, then fails with the context's error at
// the deadline or the cancellation, not database is locked after the
// busy timeout, and once the lock is free it takes it.
func TestWriteBeginEndsWithItsContext(t *testing.T) {
	dbs := dbsOpen(t, 2)
	db := dbs[0]
	conn, err := dbs[1].Pool().Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	create := func(email string) rt.Effect[User] {
		return rt.Atomic(UserCreate(UserNew().EmailSet(email).NameSet("Ann")))
	}
	deadline, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := run.Run(deadline, db, create("a@example.com")); !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("a write under a 100ms deadline beside a writer = %v after %v; want the deadline at once", err, time.Since(start))
	}
	cancelled, stop := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, stop)
	start = time.Now()
	if _, err := run.Run(cancelled, db, create("b@example.com")); !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
		t.Fatalf("a write cancelled beside a writer = %v after %v; want the cancellation at once", err, time.Since(start))
	}
	time.AfterFunc(100*time.Millisecond, func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") })
	if _, err := run.Run(context.Background(), db, create("c@example.com")); err != nil {
		t.Fatalf("a write once the lock is free = %v", err)
	}
	if got := pragmaRead(t, db, "busy_timeout"); got != 5000 {
		t.Fatalf("busy_timeout after the waits = %d; want the pool's 5000 kept", got)
	}
}

// TestUnreadRowKeepsNoConnection proves that a Lookup releases its
// statement whatever its Scan does (spec §generated-query-surface, the
// runner): on the one connection run.Open pins, a Scan that refuses
// before reading the row and one that is nil and panics, inside a
// transaction and outside one, leave the connection to the next
// statement.
func TestUnreadRowKeepsNoConnection(t *testing.T) {
	db := newDB(t)
	refuse := errors.New("refused before reading")
	refusing := rt.Lookup[int]{SQL: "SELECT 1", Scan: func(rt.Row) (int, error) { return 0, refuse }}
	scanless := rt.Lookup[int]{SQL: "SELECT 1"}
	for name, e := range map[string]rt.Effect[int]{
		"a refusing Scan outside a transaction": refusing,
		"a refusing Scan inside an Atomic":      rt.Atomic(refusing),
		"no Scan outside a transaction":         scanless,
		"no Scan inside an Atomic":              rt.Atomic(scanless),
	} {
		done := make(chan any, 1)
		go func() {
			defer func() {
				if v := recover(); v != nil {
					done <- v
				}
			}()
			_, err := run.Run(context.Background(), db, e)
			done <- err
		}()
		select {
		case got := <-done:
			if err, _ := got.(error); !errors.Is(err, refuse) && strings.Contains(name, "refusing") {
				t.Errorf("%s = %v; want its refusal", name, got)
			}
			if _, ok := got.(runtime.Error); !ok && strings.Contains(name, "no Scan") {
				t.Errorf("%s = %v; want the nil function's panic", name, got)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s did not return in 3s", name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		n, err := run.Run(ctx, db, UserCount())
		cancel()
		if err != nil || n != 0 {
			t.Fatalf("after %s the next statement = %d, %v; want the connection free", name, n, err)
		}
		if s := db.Pool().Stats(); s.InUse != 0 {
			t.Fatalf("after %s %d connections stay in use", name, s.InUse)
		}
	}
}

type pragmaCutDriver struct {
	inner driver.Driver
	cut   func()
	sent  func(query string)
}

func (d pragmaCutDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return pragmaCutConn{Conn: conn, cut: d.cut, sent: d.sent}, nil
}

var pragmaCutDrivers atomic.Int64

func pragmaCutOpen(t *testing.T, cut func(), sent func(query string)) run.DB {
	t.Helper()
	probe, err := sql.Open("sqlite3", "")
	if err != nil {
		t.Fatal(err)
	}
	inner := probe.Driver()
	_ = probe.Close()
	name := fmt.Sprintf("sqlite3_pragma_cut_%d", pragmaCutDrivers.Add(1))
	sql.Register(name, pragmaCutDriver{inner: inner, cut: cut, sent: sent})
	db := dbtest.OpenDriver(t, name, "")
	ddlApply(t, db)
	return db
}

type pragmaCutConn struct {
	driver.Conn
	cut  func()
	sent func(query string)
}

func (c pragmaCutConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.sent != nil {
		c.sent(query)
	}
	res, err := c.Conn.(driver.ExecerContext).ExecContext(context.Background(), query, args)
	if query == "PRAGMA busy_timeout = 0" && err == nil {
		c.cut()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return res, err
}

func (c pragmaCutConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

// TestWriteBeginRestoresTheBusyTimeout proves that the wait of an
// immediate begin leaves the connection's busy timeout as it found it
// (spec §generated-query-surface, the runner): a context that ends
// while the runner sets the timeout to zero, after SQLite applied it,
// as go-sqlite3 reports an interrupted pragma, fails the transaction
// with the context's error, and the connection's timeout stays 5000.
func TestWriteBeginRestoresTheBusyTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := pragmaCutOpen(t, cancel, nil)
	_, err := run.Run(ctx, db, rt.Atomic(UserCreate(UserNew().EmailSet("a@example.com").NameSet("Ann"))))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a write whose context ended at the pragma = %v; want context.Canceled", err)
	}
	if got := pragmaRead(t, db, "busy_timeout"); got != 5000 {
		t.Fatalf("busy_timeout after a context ended at the pragma = %d; want the pool's 5000", got)
	}
	if _, err := run.Run(context.Background(), db, rt.Atomic(UserCreate(UserNew().EmailSet("b@example.com").NameSet("Bob")))); err != nil {
		t.Fatalf("the next write = %v", err)
	}
}

// TestWriteBeginOfAnEndedContextTakesNoLock proves that the wait of an
// immediate begin makes no attempt once its context ended (spec
// §generated-query-surface, the runner): a context that ends while the
// runner sets the busy timeout to zero fails the transaction with the
// context's error without sending BEGIN IMMEDIATE, so a request already
// gone takes no write lock from the writers still waiting for it, and
// the timeout it set is restored.
func TestWriteBeginOfAnEndedContextTakesNoLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var sent []string
	record := func(query string) {
		mu.Lock()
		sent = append(sent, query)
		mu.Unlock()
	}
	db := pragmaCutOpen(t, cancel, record)
	mu.Lock()
	sent = nil
	mu.Unlock()
	_, err := run.Run(ctx, db, rt.Atomic(UserCreate(UserNew().EmailSet("a@example.com").NameSet("Ann"))))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a write whose context ended at the pragma = %v; want context.Canceled", err)
	}
	mu.Lock()
	got := strings.Join(sent, "; ")
	mu.Unlock()
	if strings.Contains(got, "BEGIN IMMEDIATE") {
		t.Fatalf("the runner sent %q; want no BEGIN IMMEDIATE once the context ended", got)
	}
	if !strings.Contains(got, "PRAGMA busy_timeout = 0; PRAGMA busy_timeout = 5000") {
		t.Fatalf("the runner sent %q; want the timeout set to zero and restored", got)
	}
	if n, err := run.Run(context.Background(), db, UserCount()); err != nil || n != 0 {
		t.Fatalf("after a write whose context ended, UserCount = %d, %v; want 0", n, err)
	}
}
