package rt

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
)

// ErrNotFound is what a Lookup, an Update and a keyed Delete report when
// no row matches (spec §generated-query-surface); the runner maps the
// driver's no-rows error to it.
var ErrNotFound = errors.New("rt: no row")

// ErrNoEffect is what an effect built over nothing yields (spec
// §generated-query-surface): a nil effect where one is performed, a nil
// next or f, and a Fail of a nil error.
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
// Map, an Atomic, a Pure or a Fail. Its methods are unexported, so the
// set is sealed: a type of another package satisfies it only by
// embedding one of these, and then performs as that one (spec
// §generated-query-surface). T is what performing it yields.
type Effect[T any] interface {
	perform(ctx context.Context, x Executor) (T, error)
	performAny(ctx context.Context, x Executor) (any, error)
}

// AnyEffect is an Effect of any result, as Tx and All take them.
type AnyEffect interface {
	performAny(ctx context.Context, x Executor) (any, error)
}

// Perform performs e over x and returns what it yields (spec
// §generated-query-surface): with PerformAny, the only functions that
// hand an effect an executor, which the runner calls. A nil e yields
// ErrNoEffect.
func Perform[T any](ctx context.Context, x Executor, e Effect[T]) (T, error) {
	if e == nil {
		var zero T
		return zero, ErrNoEffect
	}
	return e.perform(ctx, x)
}

// PerformAny is Perform for an effect of any result, as the shim holds
// one.
func PerformAny(ctx context.Context, x Executor, e AnyEffect) (any, error) {
	if e == nil {
		return nil, ErrNoEffect
	}
	return e.performAny(ctx, x)
}

// Query is zero or more rows of T (spec §generated-query-surface): a
// statement, its arguments and the scan of one row, every row scanned
// into a slice that is empty, never nil, unless the query has no
// statement, which yields nil. A query that failed to render carries
// its error in Err.
type Query[T any] struct {
	SQL  string
	Args []any
	Scan func(Row) (T, error)
	Err  error
}

func (q Query[T]) perform(ctx context.Context, x Executor) ([]T, error) {
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

func (q Query[T]) performAny(ctx context.Context, x Executor) (any, error) { return q.perform(ctx, x) }

// Lookup is exactly one row of T, or ErrNotFound (spec
// §generated-query-surface). A lookup that failed to render carries its
// error in Err.
type Lookup[T any] struct {
	SQL  string
	Args []any
	Scan func(Row) (T, error)
	Err  error
}

func (l Lookup[T]) perform(ctx context.Context, x Executor) (T, error) {
	if l.Err != nil {
		var zero T
		return zero, l.Err
	}
	return l.Scan(x.QueryRow(ctx, l.SQL, l.Args))
}

func (l Lookup[T]) performAny(ctx context.Context, x Executor) (any, error) { return l.perform(ctx, x) }

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

func (w Write[M]) perform(ctx context.Context, x Executor) (M, error) {
	if w.Err != nil {
		var zero M
		return zero, w.Err
	}
	if w.SQL == "" {
		return w.Row, nil
	}
	return w.Scan(x.QueryRow(ctx, w.SQL, w.Args))
}

func (w Write[M]) performAny(ctx context.Context, x Executor) (any, error) { return w.perform(ctx, x) }

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

func (e Exec) perform(ctx context.Context, x Executor) (int64, error) {
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

func (e Exec) performAny(ctx context.Context, x Executor) (any, error) { return e.perform(ctx, x) }

// Transaction runs its steps in order inside one transaction, all or
// nothing, and yields their results in order (spec
// §generated-query-surface): the first error rolls back and is
// returned, and a nil step yields ErrNoEffect before the transaction
// opens.
type Transaction struct {
	Steps []AnyEffect
}

// Tx is the Transaction of steps.
func Tx(steps ...AnyEffect) Transaction { return Transaction{Steps: steps} }

func (t Transaction) perform(ctx context.Context, x Executor) ([]any, error) {
	if slices.Contains(t.Steps, nil) {
		return nil, ErrNoEffect
	}
	out := make([]any, len(t.Steps))
	err := x.Transaction(ctx, func(tx Executor) error {
		for i, s := range t.Steps {
			v, err := s.performAny(ctx, tx)
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

func (t Transaction) performAny(ctx context.Context, x Executor) (any, error) {
	return t.perform(ctx, x)
}

// Parallel runs its steps at once, each on its own goroutine, joins
// them and yields their results in order (spec
// §generated-query-surface): the first error cancels the rest and is
// returned. A step that panics cancels the rest too, and its panic is
// raised again, with the step's stack, on the goroutine that performs
// the Parallel once every step has joined, so it never ends the
// process; a nil step yields ErrNoEffect before any step runs.
type Parallel struct {
	Steps []AnyEffect
}

// All is the Parallel of steps.
func All(steps ...AnyEffect) Parallel { return Parallel{Steps: steps} }

func (p Parallel) perform(ctx context.Context, x Executor) ([]any, error) {
	if slices.Contains(p.Steps, nil) {
		return nil, ErrNoEffect
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]any, len(p.Steps))
	var mu sync.Mutex
	var first error
	stepsJoin(len(p.Steps), cancel, func(i int) {
		v, err := p.Steps[i].performAny(ctx, x)
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
	if first != nil {
		return nil, first
	}
	return out, nil
}

func (p Parallel) performAny(ctx context.Context, x Executor) (any, error) { return p.perform(ctx, x) }

func stepsJoin(n int, cancel func(), step func(i int)) {
	var mu sync.Mutex
	var raised *stepPanic
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				p, ok := v.(*stepPanic)
				if !ok {
					p = &stepPanic{value: v, stack: debug.Stack()}
				}
				mu.Lock()
				if raised == nil {
					raised = p
				}
				mu.Unlock()
				cancel()
			}()
			step(i)
		})
	}
	wg.Wait()
	if raised != nil {
		panic(raised)
	}
}

type stepPanic struct {
	value any
	stack []byte
}

func (p *stepPanic) Error() string {
	return fmt.Sprintf("%v\n\nraised again from the goroutine of a parallel step:\n%s", p.value, p.stack)
}

func (p *stepPanic) Unwrap() error {
	err, _ := p.value.(error)
	return err
}

type thenEffect[T, U any] struct {
	first Effect[T]
	next  func(T) Effect[U]
}

// Then is e and then the effect next makes of its result, performed on
// the same executor (spec §generated-query-surface): one step after the
// other, the first error stopping it, atomic only inside an Atomic or a
// Transaction. next is pure: it turns a value into a description and
// performs nothing.
func Then[T, U any](e Effect[T], next func(T) Effect[U]) Effect[U] {
	return thenEffect[T, U]{first: e, next: next}
}

func (t thenEffect[T, U]) perform(ctx context.Context, x Executor) (U, error) {
	var zero U
	if t.first == nil || t.next == nil {
		return zero, ErrNoEffect
	}
	v, err := t.first.perform(ctx, x)
	if err != nil {
		return zero, err
	}
	e := t.next(v)
	if e == nil {
		return zero, ErrNoEffect
	}
	return e.perform(ctx, x)
}

func (t thenEffect[T, U]) performAny(ctx context.Context, x Executor) (any, error) {
	return t.perform(ctx, x)
}

type mapEffect[T, U any] struct {
	inner Effect[T]
	f     func(T) U
}

// Map is e with its result through f (spec §generated-query-surface); f
// is pure, from a value to a value.
func Map[T, U any](e Effect[T], f func(T) U) Effect[U] {
	return mapEffect[T, U]{inner: e, f: f}
}

func (m mapEffect[T, U]) perform(ctx context.Context, x Executor) (U, error) {
	var zero U
	if m.inner == nil || m.f == nil {
		return zero, ErrNoEffect
	}
	v, err := m.inner.perform(ctx, x)
	if err != nil {
		return zero, err
	}
	return m.f(v), nil
}

func (m mapEffect[T, U]) performAny(ctx context.Context, x Executor) (any, error) {
	return m.perform(ctx, x)
}

type atomicEffect[T any] struct {
	inner Effect[T]
}

// Atomic is e and everything it performs in one transaction, committed
// when all of it succeeds and rolled back on the first error (spec
// §generated-query-surface); inside a Transaction or another Atomic it
// joins the transaction already open.
func Atomic[T any](e Effect[T]) Effect[T] { return atomicEffect[T]{inner: e} }

func (a atomicEffect[T]) perform(ctx context.Context, x Executor) (T, error) {
	var out T
	if a.inner == nil {
		return out, ErrNoEffect
	}
	err := x.Transaction(ctx, func(tx Executor) error {
		v, err := a.inner.perform(ctx, tx)
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

func (a atomicEffect[T]) performAny(ctx context.Context, x Executor) (any, error) {
	return a.perform(ctx, x)
}

type pureEffect[T any] struct {
	v T
}

// Pure is the effect that performs nothing and yields v (spec
// §generated-query-surface), what a next returns when nothing is left
// to perform.
func Pure[T any](v T) Effect[T] { return pureEffect[T]{v: v} }

func (p pureEffect[T]) perform(context.Context, Executor) (T, error) { return p.v, nil }

func (p pureEffect[T]) performAny(ctx context.Context, x Executor) (any, error) {
	return p.perform(ctx, x)
}

type failEffect[T any] struct {
	err error
}

// Fail is the effect that performs nothing and fails with err (spec
// §generated-query-surface), what a next returns when it refuses what
// it read; inside an Atomic it rolls back what the Atomic performed. A
// Fail of a nil err yields ErrNoEffect.
func Fail[T any](err error) Effect[T] { return failEffect[T]{err: err} }

func (f failEffect[T]) perform(context.Context, Executor) (T, error) {
	var zero T
	if f.err == nil {
		return zero, ErrNoEffect
	}
	return zero, f.err
}

func (f failEffect[T]) performAny(ctx context.Context, x Executor) (any, error) {
	return f.perform(ctx, x)
}

// ParallelMap applies f to every element of xs at once, one goroutine
// per element, and joins them before returning the results in order
// (spec §values-and-effects): CPU parallelism inside pure code. A panic
// of f is raised again, with its goroutine's stack, on the caller's
// once every element has joined, as a Parallel's step's is.
func ParallelMap[A, B any](xs []A, f func(A) B) []B {
	out := make([]B, len(xs))
	stepsJoin(len(xs), func() {}, func(i int) { out[i] = f(xs[i]) })
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
// Then by its kind and its first effect and a Map or an Atomic by its
// kind and its inner one, since a function cannot be spelled, a Pure
// by its value and a Fail by its error.
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
func (p pureEffect[T]) describe() string    { return fmt.Sprintf("pure %v", p.v) }
func (f failEffect[T]) describe() string    { return fmt.Sprintf("fail %v", f.err) }
