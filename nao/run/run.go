package run

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

// DB is the one stateful object of the data layer (spec
// §values-and-effects): the connection pool, the statement cache every
// statement is prepared through, and the logger that writes one line
// per statement. It is made in main, handed to the generated router and
// held by the generated shims; a DB is a value whose fields are shared.
type DB struct {
	pool  *sql.DB
	cache *StmtCache
	log   *slog.Logger
}

// Open opens a SQLite database via the named database/sql driver and
// applies the pragmas the generated code is designed for (spec
// §generated-query-surface) to every connection the pool opens: WAL, a
// busy timeout of five seconds and foreign keys on. It pins the pool to
// a single connection.
func Open(driverName, dsn string) (DB, error) {
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
	pool := sql.OpenDB(pragmaConnector{c})
	pool.SetMaxOpenConns(1)
	if err := pool.Ping(); err != nil {
		_ = pool.Close()
		return DB{}, fmt.Errorf("run.Open: %w", err)
	}
	return Wrap(pool), nil
}

var pragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA busy_timeout = 5000",
	"PRAGMA foreign_keys = ON",
}

type dsnConnector struct {
	driver driver.Driver
	dsn    string
}

func (c dsnConnector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open(c.dsn) }
func (c dsnConnector) Driver() driver.Driver                        { return c.driver }

type pragmaConnector struct{ inner driver.Connector }

func (c pragmaConnector) Driver() driver.Driver { return c.inner.Driver() }

func (c pragmaConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	for _, pragma := range pragmas {
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

// Wrap is a DB over a pool the caller opened, with its own pragmas, a
// cache of 256 statements and the default logger.
func Wrap(pool *sql.DB) DB {
	return DB{pool: pool, cache: NewStmtCache(256), log: slog.Default()}
}

// Pool is the pool, for what reads the database outside the effects:
// the tool's fold and a program's own shutdown.
func (db DB) Pool() *sql.DB { return db.pool }

// WithLogger is db logging each statement through l, at debug level.
func (db DB) WithLogger(l *slog.Logger) DB {
	db.log = l
	return db
}

// Close closes the cached statements and the pool.
func (db DB) Close() error {
	return errors.Join(db.cache.Close(), db.pool.Close())
}

// Run performs e over db and returns what it yields (spec
// §values-and-effects): the one place a described effect meets the
// database.
func Run[T any](ctx context.Context, db DB, e rt.Effect[T]) (T, error) {
	return rt.Perform(ctx, poolExecutor{db}, e)
}

type poolExecutor struct{ db DB }

func (x poolExecutor) stmt(ctx context.Context, query string) (*sql.Stmt, error) {
	return x.db.cache.Prepare(ctx, x.db.pool, query)
}

func (x poolExecutor) Query(ctx context.Context, query string, args []any) (rt.Rows, error) {
	stmt, err := x.stmt(ctx, query)
	if err != nil {
		return nil, err
	}
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	rows, err := stmt.QueryContext(ctx, bound...)
	x.db.logged(query, start, err)
	return rows, err
}

func (x poolExecutor) QueryRow(ctx context.Context, query string, args []any) rt.Row {
	stmt, err := x.stmt(ctx, query)
	if err != nil {
		return errRow{err}
	}
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return errRow{err}
	}
	start := time.Now()
	r := stmt.QueryRowContext(ctx, bound...)
	x.db.logged(query, start, r.Err())
	return row{r}
}

func (x poolExecutor) Exec(ctx context.Context, query string, args []any) (int64, error) {
	stmt, err := x.stmt(ctx, query)
	if err != nil {
		return 0, err
	}
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

func (x poolExecutor) Transaction(ctx context.Context, fn func(rt.Executor) error) error {
	conn, err := x.db.pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	open := true
	defer func() {
		if open {
			rollback(conn)
		}
	}()
	if err := fn(txExecutor{db: x.db, conn: conn}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "COMMIT"); err != nil {
		return err
	}
	open = false
	return nil
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
	db   DB
	conn *sql.Conn
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
	rows, err := x.conn.QueryContext(ctx, query, bound...)
	x.db.logged(query, start, err)
	return rows, err
}

func (x txExecutor) QueryRow(ctx context.Context, query string, args []any) rt.Row {
	if err := ctx.Err(); err != nil {
		return errRow{err}
	}
	bound, err := rt.ArgsBound(args)
	if err != nil {
		return errRow{err}
	}
	start := time.Now()
	r := x.conn.QueryRowContext(ctx, query, bound...)
	x.db.logged(query, start, r.Err())
	return row{r}
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
	res, err := x.conn.ExecContext(ctx, query, bound...)
	x.db.logged(query, start, err)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (x txExecutor) Transaction(ctx context.Context, fn func(rt.Executor) error) error {
	return fn(x)
}

func (db DB) logged(query string, start time.Time, err error) {
	db.log.Debug("statement", "sql", query, "duration", time.Since(start), "err", err)
}

type row struct{ r *sql.Row }

func (r row) Scan(dest ...any) error {
	err := r.r.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return rt.ErrNotFound
	}
	return err
}

type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

// Statements is how many statements the cache holds, for tests and
// observability.
func (db DB) Statements() int { return db.cache.Len() }

// Perform is Run for an effect of any result, as the shim holds one.
func Perform(ctx context.Context, db DB, e rt.AnyEffect) (any, error) {
	return rt.PerformAny(ctx, poolExecutor{db}, e)
}

// Performer is the function the generated router hands the shim: Perform
// over db (spec §requests-and-responses).
func Performer(db DB) func(context.Context, rt.AnyEffect) (any, error) {
	return func(ctx context.Context, e rt.AnyEffect) (any, error) { return Perform(ctx, db, e) }
}
