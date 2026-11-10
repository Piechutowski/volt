package run

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

// DB is the one stateful object of the data layer (spec
// §values-and-effects): the writer's pool of one connection, the
// readers' pool, a statement cache for each, through which every
// statement outside a transaction is prepared, and the logger that
// writes one line per statement. It is made in main, handed to the
// generated router and held by the generated shims; a DB is a value
// whose fields are shared.
type DB struct {
	pool    *sql.DB
	cache   *StmtCache
	readers *readers
	log     *slog.Logger
}

type readers struct {
	pool  *sql.DB
	cache *StmtCache
	mu    sync.Mutex
	limit atomic.Int64
}

func (r *readers) fit() {
	n := int64(runtime.GOMAXPROCS(0))
	if r.limit.Load() == n {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limit.Load() == n {
		return
	}
	r.pool.SetMaxOpenConns(int(n))
	r.pool.SetMaxIdleConns(int(n))
	r.limit.Store(n)
}

// Open opens the SQLite database file at dsn via the named database/sql
// driver (spec §generated-query-surface, the runner): a writer of one
// connection with WAL, a busy timeout of five seconds and foreign keys
// on, and readers as many as GOMAXPROCS, following it, with the same
// timeout and foreign keys and query_only. It refuses an in-memory
// database.
func Open(driverName, dsn string) (DB, error) {
	return open("run.Open", driverName, dsn, false)
}

// OpenReadOnly opens the SQLite database file at dsn for reading alone
// (spec §generated-query-surface): readers only, each with a busy
// timeout of five seconds, foreign keys on and query_only, so a write
// fails and the file is left as it was, its journal mode included.
func OpenReadOnly(driverName, dsn string) (DB, error) {
	return open("run.OpenReadOnly", driverName, dsn, true)
}

func open(name, driverName, dsn string, readOnly bool) (DB, error) {
	if err := MemoryRefused(dsn); err != nil {
		return DB{}, fmt.Errorf("%s: %w", name, err)
	}
	probe, err := sql.Open(driverName, dsn)
	if err != nil {
		return DB{}, err
	}
	d := probe.Driver()
	_ = probe.Close()
	var c driver.Connector = dsnConnector{driver: d, dsn: dsn}
	if dc, ok := d.(driver.DriverContext); ok {
		if c, err = dc.OpenConnector(dsn); err != nil {
			return DB{}, err
		}
	}
	var writer *sql.DB
	if !readOnly {
		writer = sql.OpenDB(pragmaConnector{inner: c, pragmas: writerPragmas})
		writer.SetMaxOpenConns(1)
		if err := writer.Ping(); err != nil {
			_ = writer.Close()
			return DB{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	r := &readers{pool: sql.OpenDB(pragmaConnector{inner: c, pragmas: readerPragmas}), cache: NewStmtCache(256)}
	r.fit()
	if err := r.pool.Ping(); err != nil {
		_ = r.pool.Close()
		if writer != nil {
			_ = writer.Close()
		}
		return DB{}, fmt.Errorf("%s: %w", name, err)
	}
	db := DB{pool: writer, cache: NewStmtCache(256), readers: r, log: slog.Default()}
	if readOnly {
		db.pool, db.cache = r.pool, r.cache
	}
	return db, nil
}

// MemoryRefused is the error Open and OpenReadOnly return for a dsn that
// names an in-memory database, and nil for a file (spec
// §generated-query-surface): an in-memory database lives on one
// connection and is gone when it closes.
func MemoryRefused(dsn string) error {
	if dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:") || strings.Contains(dsn, "mode=memory") {
		return fmt.Errorf("%s is an in-memory database, which holds nothing beyond its one connection; name a file", dsn)
	}
	return nil
}

var writerPragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA busy_timeout = 5000",
	"PRAGMA foreign_keys = ON",
}

var readerPragmas = []string{
	"PRAGMA busy_timeout = 5000",
	"PRAGMA foreign_keys = ON",
	"PRAGMA query_only = ON",
}

type dsnConnector struct {
	driver driver.Driver
	dsn    string
}

func (c dsnConnector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open(c.dsn) }
func (c dsnConnector) Driver() driver.Driver                        { return c.driver }

type pragmaConnector struct {
	inner   driver.Connector
	pragmas []string
}

func (c pragmaConnector) Driver() driver.Driver { return c.inner.Driver() }

func (c pragmaConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	for _, pragma := range c.pragmas {
		if err := driverExec(ctx, conn, pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return conn, nil
}

func driverExec(ctx context.Context, conn driver.Conn, query string) error {
	if e, ok := conn.(driver.ExecerContext); ok {
		_, err := e.ExecContext(ctx, query, nil)
		if !errors.Is(err, driver.ErrSkip) {
			return err
		}
	}
	stmt, err := conn.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if s, ok := stmt.(driver.StmtExecContext); ok {
		_, err = s.ExecContext(ctx, nil)
	} else {
		_, err = stmt.Exec(nil)
	}
	return err
}

// Pool is the writer's pool, the readers' for a DB OpenReadOnly made,
// for what touches the database outside the effects: the tool's fold,
// migrations, tests' DDL and a program's own shutdown.
func (db DB) Pool() *sql.DB { return db.pool }

// WithLogger is db logging each statement through l, at debug level.
func (db DB) WithLogger(l *slog.Logger) DB {
	db.log = l
	return db
}

// Close closes the cached statements and the pools.
func (db DB) Close() error {
	if db.pool == db.readers.pool {
		return errors.Join(db.cache.Close(), db.pool.Close())
	}
	return errors.Join(db.cache.Close(), db.pool.Close(), db.readers.cache.Close(), db.readers.pool.Close())
}

func (db DB) readersIdleClose() {
	r := db.readers
	if r.pool == db.pool {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pool.SetMaxIdleConns(0)
	r.limit.Store(0)
}

var errReaderWrite = errors.New("run: a transaction that can write reached a reader")

// Run performs e over db and returns what it yields (spec
// §values-and-effects): the one place a described effect meets the
// database.
func Run[T any](ctx context.Context, db DB, e rt.Effect[T]) (T, error) {
	return rt.Perform(ctx, db.writer(), e)
}

type poolExecutor struct {
	db    DB
	pool  *sql.DB
	cache *StmtCache
	reads bool
}

func (db DB) writer() poolExecutor {
	return poolExecutor{db: db, pool: db.pool, cache: db.cache, reads: db.pool == db.readers.pool}
}

func (db DB) reader() poolExecutor {
	db.readers.fit()
	return poolExecutor{db: db, pool: db.readers.pool, cache: db.readers.cache, reads: true}
}

func (x poolExecutor) Reader() rt.Executor {
	if x.reads {
		return x
	}
	return x.db.reader()
}

func (x poolExecutor) Query(ctx context.Context, query string, args []any) (rt.Rows, error) {
	stmt, release, err := x.cache.Prepare(ctx, x.pool, query)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	defer func() {
		if rows == nil {
			release()
		}
	}()
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	rows, err = stmt.QueryContext(ctx, bound...)
	x.db.logged(query, start, err)
	if err != nil {
		return nil, err
	}
	return heldRows{Rows: rows, release: release}, nil
}

type heldRows struct {
	*sql.Rows
	release func()
}

func (r heldRows) Close() error {
	defer r.release()
	return r.Rows.Close()
}

func (x poolExecutor) Exec(ctx context.Context, query string, args []any) (int64, error) {
	stmt, release, err := x.cache.Prepare(ctx, x.pool, query)
	if err != nil {
		return 0, err
	}
	defer release()
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	res, err := stmt.ExecContext(ctx, bound...)
	x.db.logged(query, start, err)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (x poolExecutor) Transaction(ctx context.Context, writes bool, fn func(rt.Executor) error) error {
	if writes && x.reads {
		return errReaderWrite
	}
	conn, err := x.pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if writes {
		err = immediateBegin(ctx, conn)
	} else {
		_, err = conn.ExecContext(ctx, "BEGIN")
	}
	if err != nil {
		return err
	}
	open := true
	defer func() {
		if open {
			rollback(conn)
		}
	}()
	stmts := &txStmts{conn: conn, byText: map[string]*sql.Stmt{}}
	defer stmts.close()
	if err := fn(txExecutor{db: x.db, conn: conn, stmts: stmts}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stmts.close()
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "COMMIT"); err != nil {
		return err
	}
	open = false
	return nil
}

var busyDelays = [...]time.Duration{1, 2, 5, 10, 15, 20, 25, 25, 25, 50, 50, 100}

func immediateBegin(ctx context.Context, conn *sql.Conn) error {
	settled := context.WithoutCancel(ctx)
	var timeout int64
	if err := conn.QueryRowContext(settled, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
		return err
	}
	_, err := conn.ExecContext(settled, "PRAGMA busy_timeout = 0")
	if err == nil {
		err = immediateRetry(ctx, conn, time.Now().Add(time.Duration(timeout)*time.Millisecond))
	}
	if _, e := conn.ExecContext(settled, fmt.Sprintf("PRAGMA busy_timeout = %d", timeout)); e != nil {
		if err == nil {
			rollback(conn)
		}
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return errors.Join(err, e)
	}
	return err
}

func immediateRetry(ctx context.Context, conn *sql.Conn, deadline time.Time) error {
	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err == nil || !strings.Contains(err.Error(), "database is locked") {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		wait := min(busyDelays[min(i, len(busyDelays)-1)]*time.Millisecond, time.Until(deadline))
		if wait <= 0 {
			return err
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func rollback(conn *sql.Conn) {
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err == nil {
		return
	}
	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
}

type txExecutor struct {
	db    DB
	conn  *sql.Conn
	stmts *txStmts
}

const txStmtsMax = 64

type txStmts struct {
	conn   *sql.Conn
	byText map[string]*sql.Stmt
}

func (s *txStmts) prepared(ctx context.Context, query string) (*sql.Stmt, error) {
	if stmt, ok := s.byText[query]; ok {
		return stmt, nil
	}
	if len(s.byText) >= txStmtsMax {
		return nil, nil
	}
	stmt, err := s.conn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	s.byText[query] = stmt
	return stmt, nil
}

func (s *txStmts) close() {
	for query, stmt := range s.byText {
		_ = stmt.Close()
		delete(s.byText, query)
	}
}

func (x txExecutor) Query(ctx context.Context, query string, args []any) (rt.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	stmt, err := x.stmts.prepared(ctx, query)
	if err != nil {
		x.db.logged(query, start, err)
		return nil, err
	}
	var rows *sql.Rows
	if stmt != nil {
		rows, err = stmt.QueryContext(ctx, bound...)
	} else {
		rows, err = x.conn.QueryContext(ctx, query, bound...)
	}
	x.db.logged(query, start, err)
	return rows, err
}

func (x txExecutor) Exec(ctx context.Context, query string, args []any) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	stmt, err := x.stmts.prepared(ctx, query)
	if err != nil {
		x.db.logged(query, start, err)
		return 0, err
	}
	var res sql.Result
	if stmt != nil {
		res, err = stmt.ExecContext(ctx, bound...)
	} else {
		res, err = x.conn.ExecContext(ctx, query, bound...)
	}
	x.db.logged(query, start, err)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (x txExecutor) Transaction(ctx context.Context, _ bool, fn func(rt.Executor) error) error {
	return fn(x)
}

func (x txExecutor) Reader() rt.Executor { return x }

func (db DB) logged(query string, start time.Time, err error) {
	db.log.Debug("statement", "sql", query, "duration", time.Since(start), "err", err)
}

// Statements is how many statements the caches hold, the writer's and
// the readers', for tests and observability.
func (db DB) Statements() int {
	if db.cache == db.readers.cache {
		return db.cache.Len()
	}
	return db.cache.Len() + db.readers.cache.Len()
}

// Perform is Run for an effect of any result, as the shim holds one.
func Perform(ctx context.Context, db DB, e rt.AnyEffect) (any, error) {
	return rt.PerformAny(ctx, db.writer(), e)
}

// Performer is the function the generated router hands the shim: Perform
// over db (spec §requests-and-responses).
func Performer(db DB) func(context.Context, rt.AnyEffect) (any, error) {
	return func(ctx context.Context, e rt.AnyEffect) (any, error) { return Perform(ctx, db, e) }
}
