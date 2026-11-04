package rt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrNotFound is what a Lookup, an Update and a keyed Delete report when
// no row matches (spec §generated-query-surface); the runner maps the
// driver's no-rows error to it.
var ErrNotFound = errors.New("rt: no row")

// ErrNoEffect is what a Then, a Map or an Atomic over no effect yields,
// and a Then whose next returns none (spec §generated-query-surface).
var ErrNoEffect = errors.New("rt: no effect to perform")

// Row is what a generated scan function reads: one row's columns.
type Row interface {
	Scan(dest ...any) error
}

// Rows is what a Query reads: a cursor over the rows a statement
// returned, closed when read.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// Executor performs statements for the effects (spec
// §generated-query-surface): the runner's handle over one pool or one
// transaction. nao/run implements it and nothing else does; an Exec
// reports the rows affected, and a Transaction runs fn over an
// executor that commits when fn returns nil and rolls back otherwise.
type Executor interface {
	Query(ctx context.Context, sql string, args []any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args []any) Row
	Exec(ctx context.Context, sql string, args []any) (int64, error)
	Transaction(ctx context.Context, fn func(Executor) error) error
}

// Effect is a computation over the database described as a value and
// performed by the runner (spec §values-and-effects): a Query, a
// Lookup, a Write, an Exec, a With, a Transaction, an All, a Then, a
// Map or an Atomic, nothing else, since effect is unexported. T is what
// performing it yields.
type Effect[T any] interface {
	Perform(ctx context.Context, x Executor) (T, error)
	PerformAny(ctx context.Context, x Executor) (any, error)
	effect()
}

// AnyEffect is an Effect of any result, as Tx and All take them.
type AnyEffect interface {
	PerformAny(ctx context.Context, x Executor) (any, error)
	effect()
}

// Query is zero or more rows of T (spec §generated-query-surface): a
// statement, its arguments and the scan of one row. A query that failed
// to render carries its error in Err; one with no statement yields nil.
type Query[T any] struct {
	SQL  string
	Args []any
	Scan func(Row) (T, error)
	Err  error
}

func (Query[T]) effect() {}

// Perform runs the statement and scans every row; the result is an
// empty slice, never nil, unless the query has no statement.
func (q Query[T]) Perform(ctx context.Context, x Executor) ([]T, error) {
	if q.Err != nil {
		return nil, q.Err
	}
	if q.SQL == "" {
		return nil, nil
	}
	rows, err := x.Query(ctx, q.SQL, q.Args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := q.Scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// PerformAny is Perform with its result as any.
func (q Query[T]) PerformAny(ctx context.Context, x Executor) (any, error) { return q.Perform(ctx, x) }

// Lookup is exactly one row of T, or ErrNotFound (spec
// §generated-query-surface). A lookup that failed to render carries its
// error in Err.
type Lookup[T any] struct {
	SQL  string
	Args []any
	Scan func(Row) (T, error)
	Err  error
}

func (Lookup[T]) effect() {}

// Perform runs the statement and scans its one row.
func (l Lookup[T]) Perform(ctx context.Context, x Executor) (T, error) {
	if l.Err != nil {
		var zero T
		return zero, l.Err
	}
	return l.Scan(x.QueryRow(ctx, l.SQL, l.Args))
}

// PerformAny is Perform with its result as any.
func (l Lookup[T]) PerformAny(ctx context.Context, x Executor) (any, error) { return l.Perform(ctx, x) }

// Write is an insert or an update of a changeset's marks that returns
// the stored row (spec §changesets): a Write built from an invalid
// changeset carries its *Invalid in Err and the runner returns it
// without running a statement; an update with nothing marked has no
// statement and yields Row, the changeset's row untouched.
type Write[M any] struct {
	SQL  string
	Args []any
	Scan func(Row) (M, error)
	Row  M
	Err  error
}

func (Write[M]) effect() {}

// Perform runs the statement, when there is one, and scans the stored
// row.
func (w Write[M]) Perform(ctx context.Context, x Executor) (M, error) {
	if w.Err != nil {
		var zero M
		return zero, w.Err
	}
	if w.SQL == "" {
		return w.Row, nil
	}
	return w.Scan(x.QueryRow(ctx, w.SQL, w.Args))
}

// PerformAny is Perform with its result as any.
func (w Write[M]) PerformAny(ctx context.Context, x Executor) (any, error) { return w.Perform(ctx, x) }

// Insert is the Write that inserts the marked columns of c, DEFAULT
// VALUES when none, and returns the stored row (spec §changesets): an
// unmarked rowid alias that is not the database-assigned key is a
// not_null field error, since SQLite would store a rowid the row never
// gave.
func Insert[M any](c Changeset[M]) Write[M] {
	c = c.keyUnmarkedRefuse()
	if err := c.Err(); err != nil {
		return Write[M]{Err: err}
	}
	sql, args := c.InsertRender()
	return Write[M]{SQL: sql, Args: args, Scan: c.d().Scan}
}

// Update is the Write that writes the marked columns of c to the row it
// was made from and returns the stored row (spec §changesets): ErrNoRow
// for a changeset not made from a row, before validity; nothing marked
// yields the row untouched.
func Update[M any](c Changeset[M]) Write[M] {
	if !c.row {
		return Write[M]{Err: ErrNoRow}
	}
	if err := c.Err(); err != nil {
		return Write[M]{Err: err}
	}
	sql, args := c.UpdateRender()
	return Write[M]{SQL: sql, Args: args, Scan: c.d().Scan, Row: c.Row()}
}

// Exec is a statement that returns no rows and yields how many it
// affected (spec §generated-query-surface): a keyed delete, which is
// ErrNotFound when none was, or a bulk delete or update. A database
// refusal is mapped onto Model's fields when Model is set.
type Exec struct {
	SQL   string
	Args  []any
	Model string
	One   bool
	Err   error
}

func (Exec) effect() {}

// Perform runs the statement and reports the rows affected.
func (e Exec) Perform(ctx context.Context, x Executor) (int64, error) {
	if e.Err != nil {
		return 0, e.Err
	}
	n, err := x.Exec(ctx, e.SQL, e.Args)
	if err != nil {
		if e.Model != "" {
			return 0, Refused(e.Model, err)
		}
		return 0, err
	}
	if e.One && n == 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

// PerformAny is Perform with its result as any.
func (e Exec) PerformAny(ctx context.Context, x Executor) (any, error) { return e.Perform(ctx, x) }

// Transaction runs its steps in order inside one transaction, all or
// nothing, and yields their results in order (spec
// §generated-query-surface).
type Transaction struct {
	Steps []AnyEffect
}

// Tx is the Transaction of steps.
func Tx(steps ...AnyEffect) Transaction { return Transaction{Steps: steps} }

func (Transaction) effect() {}

// Perform runs every step inside one transaction; the first error rolls
// back and is returned.
func (t Transaction) Perform(ctx context.Context, x Executor) ([]any, error) {
	out := make([]any, len(t.Steps))
	err := x.Transaction(ctx, func(tx Executor) error {
		for i, s := range t.Steps {
			v, err := s.PerformAny(ctx, tx)
			if err != nil {
				return err
			}
			out[i] = v
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PerformAny is Perform with its result as any.
func (t Transaction) PerformAny(ctx context.Context, x Executor) (any, error) {
	return t.Perform(ctx, x)
}

// Parallel runs its steps at once, each on its own goroutine, and
// yields their results in order (spec §values-and-effects); the first
// error cancels the rest and is returned.
type Parallel struct {
	Steps []AnyEffect
}

// All is the Parallel of steps.
func All(steps ...AnyEffect) Parallel { return Parallel{Steps: steps} }

func (Parallel) effect() {}

// Perform runs every step at once and joins them before returning; the
// error returned is the one that cancelled the rest.
func (p Parallel) Perform(ctx context.Context, x Executor) ([]any, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]any, len(p.Steps))
	var mu sync.Mutex
	var first error
	var wg sync.WaitGroup
	for i, s := range p.Steps {
		wg.Go(func() {
			v, err := s.PerformAny(ctx, x)
			if err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
				cancel()
				return
			}
			out[i] = v
		})
	}
	wg.Wait()
	if first != nil {
		return nil, first
	}
	return out, nil
}

// PerformAny is Perform with its result as any.
func (p Parallel) PerformAny(ctx context.Context, x Executor) (any, error) { return p.Perform(ctx, x) }

type thenEffect[T, U any] struct {
	first Effect[T]
	next  func(T) Effect[U]
}

// Then is e and then the effect next makes of its result, performed on
// the same executor (spec §generated-query-surface): one step after the
// other, atomic only inside an Atomic or a Transaction. next is pure: it
// turns a value into a description and performs nothing.
func Then[T, U any](e Effect[T], next func(T) Effect[U]) Effect[U] {
	return thenEffect[T, U]{first: e, next: next}
}

func (thenEffect[T, U]) effect() {}

// Perform performs the first effect, calls next with its result and
// performs what next returns; the first error stops it and is returned.
func (t thenEffect[T, U]) Perform(ctx context.Context, x Executor) (U, error) {
	var zero U
	if t.first == nil {
		return zero, ErrNoEffect
	}
	v, err := t.first.Perform(ctx, x)
	if err != nil {
		return zero, err
	}
	e := t.next(v)
	if e == nil {
		return zero, ErrNoEffect
	}
	return e.Perform(ctx, x)
}

// PerformAny is Perform with its result as any.
func (t thenEffect[T, U]) PerformAny(ctx context.Context, x Executor) (any, error) {
	return t.Perform(ctx, x)
}

type mapEffect[T, U any] struct {
	inner Effect[T]
	f     func(T) U
}

// Map is e with its result through f (spec §generated-query-surface); f
// is pure.
func Map[T, U any](e Effect[T], f func(T) U) Effect[U] {
	return mapEffect[T, U]{inner: e, f: f}
}

func (mapEffect[T, U]) effect() {}

// Perform performs the inner effect and yields f of its result.
func (m mapEffect[T, U]) Perform(ctx context.Context, x Executor) (U, error) {
	var zero U
	if m.inner == nil {
		return zero, ErrNoEffect
	}
	v, err := m.inner.Perform(ctx, x)
	if err != nil {
		return zero, err
	}
	return m.f(v), nil
}

// PerformAny is Perform with its result as any.
func (m mapEffect[T, U]) PerformAny(ctx context.Context, x Executor) (any, error) {
	return m.Perform(ctx, x)
}

type atomicEffect[T any] struct {
	inner Effect[T]
}

// Atomic is e and everything it performs in one transaction, committed
// when all of it succeeds and rolled back on the first error (spec
// §generated-query-surface); inside a Transaction or another Atomic it
// joins the transaction already open.
func Atomic[T any](e Effect[T]) Effect[T] { return atomicEffect[T]{inner: e} }

func (atomicEffect[T]) effect() {}

// Perform performs the inner effect over the executor's transaction.
func (a atomicEffect[T]) Perform(ctx context.Context, x Executor) (T, error) {
	var out T
	if a.inner == nil {
		return out, ErrNoEffect
	}
	err := x.Transaction(ctx, func(tx Executor) error {
		v, err := a.inner.Perform(ctx, tx)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// PerformAny is Perform with its result as any.
func (a atomicEffect[T]) PerformAny(ctx context.Context, x Executor) (any, error) {
	return a.Perform(ctx, x)
}

// ParallelMap applies f to every element of xs at once, one goroutine
// per element, and joins them before returning the results in order
// (spec §values-and-effects): CPU parallelism inside pure code.
func ParallelMap[A, B any](xs []A, f func(A) B) []B {
	out := make([]B, len(xs))
	var wg sync.WaitGroup
	for i, x := range xs {
		wg.Go(func() { out[i] = f(x) })
	}
	wg.Wait()
	return out
}

// ScanOne reads a one-column row into a T, the scan of a Count or an
// Exists (spec §generated-query-surface).
func ScanOne[T any](r Row) (T, error) {
	var v T
	err := r.Scan(&v)
	return v, err
}

// Describe spells an effect for comparison and logs (spec
// §requests-and-responses): its kind, statement and arguments, a
// transaction's or a parallel's steps in order, an error it carries, a
// Then by its first effect and a Map or an Atomic by its inner one,
// since a function cannot be spelled.
func Describe(e AnyEffect) string {
	switch e := e.(type) {
	case describer:
		return e.describe()
	case Transaction:
		return "tx(" + describeAll(e.Steps) + ")"
	case Parallel:
		return "all(" + describeAll(e.Steps) + ")"
	}
	return fmt.Sprintf("%T", e)
}

type describer interface{ describe() string }

func describeAll(steps []AnyEffect) string {
	parts := make([]string, len(steps))
	for i, s := range steps {
		parts[i] = Describe(s)
	}
	return strings.Join(parts, "; ")
}

func describeStatement(kind, sql string, args []any, err error) string {
	if err != nil {
		return kind + " error: " + err.Error()
	}
	return fmt.Sprintf("%s %s %v", kind, sql, args)
}

func (q Query[T]) describe() string  { return describeStatement("query", q.SQL, q.Args, q.Err) }
func (l Lookup[T]) describe() string { return describeStatement("lookup", l.SQL, l.Args, l.Err) }
func (w Write[M]) describe() string  { return describeStatement("write", w.SQL, w.Args, w.Err) }
func (e Exec) describe() string      { return describeStatement("exec", e.SQL, e.Args, e.Err) }

func (t thenEffect[T, U]) describe() string { return "then(" + Describe(t.first) + ")" }
func (m mapEffect[T, U]) describe() string  { return "map(" + Describe(m.inner) + ")" }
func (a atomicEffect[T]) describe() string  { return "atomic(" + Describe(a.inner) + ")" }
