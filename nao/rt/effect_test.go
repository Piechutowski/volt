package rt

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

type fakeRows struct {
	rows [][]any
	i    int
}

func (r *fakeRows) Next() bool {
	r.i++
	return r.i <= len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
	for i, d := range dest {
		*(d.(*any)) = r.rows[r.i-1][i]
	}
	return nil
}

func (r *fakeRows) Err() error   { return nil }
func (r *fakeRows) Close() error { return nil }

type fakeRow struct{ err error }

func (r fakeRow) Scan(...any) error { return r.err }

type fakeExecutor struct {
	mu       sync.Mutex
	ran      []string
	rows     [][]any
	rowErr   error
	execErr  error
	affected int64
	txs      int
}

func (x *fakeExecutor) note(sql string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.ran = append(x.ran, sql)
}

func (x *fakeExecutor) Query(ctx context.Context, sql string, args []any) (Rows, error) {
	x.note(sql)
	return &fakeRows{rows: x.rows}, nil
}

func (x *fakeExecutor) QueryRow(ctx context.Context, sql string, args []any) Row {
	x.note(sql)
	return fakeRow{x.rowErr}
}

func (x *fakeExecutor) Exec(ctx context.Context, sql string, args []any) (int64, error) {
	x.note(sql)
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	return x.affected, x.execErr
}

func (x *fakeExecutor) Transaction(ctx context.Context, fn func(Executor) error) error {
	x.mu.Lock()
	x.txs++
	x.mu.Unlock()
	return fn(x)
}

func scanAny(r Row) (any, error) {
	var v any
	err := r.Scan(&v)
	return v, err
}

func TestQueryEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{rows: [][]any{{1}, {2}}}
	got, err := Query[any]{SQL: "SELECT 1", Scan: scanAny}.perform(ctx, x)
	if err != nil || !reflect.DeepEqual(got, []any{1, 2}) {
		t.Errorf("Query = %v, %v", got, err)
	}
	if got, err := (Query[any]{Scan: scanAny}).perform(ctx, x); err != nil || got != nil {
		t.Errorf("a Query with no statement = %v, %v; want nil", got, err)
	}
	bad := errors.New("render")
	if _, err := (Query[any]{SQL: "x", Err: bad, Scan: scanAny}).perform(ctx, x); !errors.Is(err, bad) {
		t.Errorf("a Query carrying an error = %v", err)
	}
	if !slices.Equal(x.ran, []string{"SELECT 1"}) {
		t.Errorf("statements run: %v", x.ran)
	}
}

func TestLookupAndWriteEffects(t *testing.T) {
	ctx := context.Background()
	miss := &fakeExecutor{rowErr: ErrNotFound}
	if _, err := (Lookup[any]{SQL: "SELECT", Scan: scanAny}).perform(ctx, miss); !errors.Is(err, ErrNotFound) {
		t.Errorf("Lookup miss = %v", err)
	}
	inv := &Invalid{Model: "Post"}
	idle := &fakeExecutor{}
	if _, err := (Write[any]{SQL: "INSERT", Err: inv}).perform(ctx, idle); !errors.Is(err, inv) || len(idle.ran) != 0 {
		t.Errorf("an invalid Write = %v after %v; want its error with nothing run", err, idle.ran)
	}
	if got, err := (Write[any]{Row: "untouched"}).perform(ctx, miss); err != nil || got != "untouched" {
		t.Errorf("a Write with nothing marked = %v, %v; want the row back", got, err)
	}
}

func TestExecEffect(t *testing.T) {
	ctx := context.Background()
	none := &fakeExecutor{affected: 0}
	if _, err := (Exec{SQL: "DELETE", One: true}).perform(ctx, none); !errors.Is(err, ErrNotFound) {
		t.Errorf("a keyed delete of nothing = %v", err)
	}
	if n, err := (Exec{SQL: "DELETE"}).perform(ctx, none); err != nil || n != 0 {
		t.Errorf("a bulk delete of nothing = %d, %v", n, err)
	}
	refused := &fakeExecutor{execErr: errors.New("FOREIGN KEY constraint failed")}
	var inv *Invalid
	if _, err := (Exec{SQL: "DELETE", Model: "Post"}).perform(ctx, refused); !errors.As(err, &inv) || inv.Model != "Post" {
		t.Errorf("a refused keyed delete = %v; want *Invalid", err)
	}
}

func TestTransactionEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 3}
	out, err := Tx(Exec{SQL: "a"}, Exec{SQL: "b"}).perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(3), int64(3)}) || x.txs != 1 {
		t.Errorf("Tx = %v, %v in %d transactions", out, err, x.txs)
	}
	boom := errors.New("boom")
	out, err = Tx(Exec{SQL: "a"}, Exec{Err: boom}, Exec{SQL: "c"}).perform(ctx, x)
	if !errors.Is(err, boom) || out != nil || slices.Contains(x.ran, "c") {
		t.Errorf("Tx with a failing step = %v, %v, ran %v; want the error and no later step", out, err, x.ran)
	}
}

func TestParallelEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 1}
	out, err := All(Exec{SQL: "a"}, Exec{SQL: "b"}, Exec{SQL: "c"}).perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(1), int64(1), int64(1)}) || len(x.ran) != 3 {
		t.Errorf("All = %v, %v after %v", out, err, x.ran)
	}
	boom := errors.New("boom")
	if _, err := All(Exec{SQL: "a"}, Exec{Err: boom}).perform(ctx, x); !errors.Is(err, boom) {
		t.Errorf("All with a failing step = %v", err)
	}
	doubled := ParallelMap([]int{1, 2, 3}, func(n int) int { return n * 2 })
	if !reflect.DeepEqual(doubled, []int{2, 4, 6}) {
		t.Errorf("ParallelMap = %v", doubled)
	}
}

type joinExecutor struct{ *fakeExecutor }

func (x joinExecutor) Transaction(ctx context.Context, fn func(Executor) error) error {
	x.mu.Lock()
	x.txs++
	x.mu.Unlock()
	return fn(inTx(x))
}

type inTx joinExecutor

func (x inTx) Query(ctx context.Context, sql string, args []any) (Rows, error) {
	return x.fakeExecutor.Query(ctx, "tx "+sql, args)
}

func (x inTx) QueryRow(ctx context.Context, sql string, args []any) Row {
	return x.fakeExecutor.QueryRow(ctx, "tx "+sql, args)
}

func (x inTx) Exec(ctx context.Context, sql string, args []any) (int64, error) {
	return x.fakeExecutor.Exec(ctx, "tx "+sql, args)
}

func (x inTx) Transaction(ctx context.Context, fn func(Executor) error) error { return fn(x) }

func TestThenEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 2}
	got, err := Then(Exec{SQL: "a"}, func(n int64) Effect[int64] { return Exec{SQL: fmt.Sprintf("b %d", n)} }).perform(ctx, x)
	if err != nil || got != 2 || !slices.Equal(x.ran, []string{"a", "b 2"}) || x.txs != 0 {
		t.Errorf("Then = %d, %v after %v in %d transactions; want the second statement built from the first's result, outside any transaction", got, err, x.ran, x.txs)
	}
	boom := errors.New("boom")
	called := false
	idle := &fakeExecutor{}
	if _, err := Then(Exec{Err: boom}, func(int64) Effect[int64] { called = true; return Exec{SQL: "b"} }).perform(ctx, idle); !errors.Is(err, boom) || called || len(idle.ran) != 0 {
		t.Errorf("Then over a failing effect = %v, next called %v, ran %v; want the error and nothing else", err, called, idle.ran)
	}
	half := &fakeExecutor{affected: 1}
	if _, err := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{Err: boom} }).perform(ctx, half); !errors.Is(err, boom) || !slices.Equal(half.ran, []string{"a"}) {
		t.Errorf("Then whose second step fails = %v after %v; want the error with the first step run", err, half.ran)
	}
	if _, err := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return nil }).perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Then whose next returns no effect = %v; want ErrNoEffect", err)
	}
	if _, err := Then(nil, func(int64) Effect[int64] { return Exec{SQL: "b"} }).perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Then over no effect = %v; want ErrNoEffect", err)
	}
}

func TestMapEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 2}
	got, err := Map(Exec{SQL: "a"}, func(n int64) string { return fmt.Sprint(n * 10) }).perform(ctx, x)
	if err != nil || got != "20" {
		t.Errorf("Map = %q, %v; want f of the result", got, err)
	}
	inv := &Invalid{Model: "Post"}
	called := false
	if _, err := Map(Write[int]{Err: inv}, func(int) int { called = true; return 1 }).perform(ctx, x); !errors.Is(err, inv) || called {
		t.Errorf("Map over a failing effect = %v, f called %v; want the error unchanged and f never called", err, called)
	}
	if _, err := Map(nil, func(int) int { return 1 }).perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Map over no effect = %v; want ErrNoEffect", err)
	}
}

func TestAtomicEffect(t *testing.T) {
	ctx := context.Background()
	x := joinExecutor{&fakeExecutor{affected: 1}}
	nested := Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[[]any] { return Tx(Exec{SQL: "b"}, Atomic(Exec{SQL: "c"})) }))
	out, err := nested.perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(1), int64(1)}) || x.txs != 1 || !slices.Equal(x.ran, []string{"tx a", "tx b", "tx c"}) {
		t.Errorf("Atomic = %v, %v after %v in %d transactions; want every step on the one transaction", out, err, x.ran, x.txs)
	}
	inv := &Invalid{Model: "Post"}
	failing := joinExecutor{&fakeExecutor{affected: 1}}
	got, err := Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{Err: inv} })).perform(ctx, failing)
	var as *Invalid
	if !errors.As(err, &as) || as != inv || got != 0 {
		t.Errorf("Atomic with a failing step = %d, %v; want the step's *Invalid unwrapped", got, err)
	}
	parallel := joinExecutor{&fakeExecutor{affected: 1}}
	if _, err := Atomic(All(Exec{SQL: "a"}, Exec{SQL: "b"})).perform(ctx, parallel); err != nil || parallel.txs != 1 || !slices.Equal(parallel.ran, []string{"tx a", "tx b"}) {
		t.Errorf("a Parallel inside an Atomic = %v after %v in %d transactions; want both steps in order on the one transaction", err, parallel.ran, parallel.txs)
	}
	if _, err := Atomic[int](nil).perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Atomic over no effect = %v; want ErrNoEffect", err)
	}
}

func TestDescribeComposedEffects(t *testing.T) {
	get := Lookup[int]{SQL: "SELECT ?", Args: []any{1}}
	one := Then(get, func(int) Effect[int64] { return Exec{SQL: "a"} })
	other := Then(get, func(int) Effect[int64] { return Exec{SQL: "b"} })
	if Describe(one) != Describe(other) || Describe(one) != "then(lookup SELECT ? [1])" {
		t.Errorf("Then describes as %q and %q; want its first effect alone", Describe(one), Describe(other))
	}
	if Describe(Then(Lookup[int]{SQL: "SELECT ?", Args: []any{2}}, func(int) Effect[int64] { return Exec{SQL: "a"} })) == Describe(one) {
		t.Error("Thens over two first effects describe the same")
	}
	if got := Describe(Atomic(Map(get, func(n int) int { return n }))); got != "atomic(map(lookup SELECT ? [1]))" {
		t.Errorf("Atomic over a Map describes as %q", got)
	}
	if Describe(Atomic(one)) == Describe(one) || Describe(Map(get, func(n int) int { return n })) == Describe(get) {
		t.Error("a composed effect describes as the effect it wraps")
	}
	if got := Describe(Pure(7)); got != "pure 7" || Describe(Pure(8)) == got {
		t.Errorf("Pure describes as %q; want its value", got)
	}
	n, m := 1, 1
	if !Same(Pure(&n), Pure(&m)) || Same(Pure(&n), Pure(new(int))) || !Same(Tx(Pure([]*int{&n})), Tx(Pure([]*int{&m}))) {
		t.Error("Same tells Pures apart by where their values point; want reflect.DeepEqual over the values")
	}
	if !Same(one, other) || Same(one, Atomic(one)) || !Same(nil, nil) || Same(nil, one) {
		t.Error("Same disagrees with the descriptions it compares")
	}
	if got := Describe(Fail[int](errors.New("refused"))); got != "fail refused" || Describe(Fail[int](errors.New("other"))) == got {
		t.Errorf("Fail describes as %q; want its error", got)
	}
}

type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return e.msg }

// TestSameComparesValues proves rt.Same over what each kind carries
// (spec §requests-and-responses): arguments by value and type, never
// as %v prints them, an Exec's One and Model, a Write's Row when it has
// no statement, a Fail's error whole, and a NaN as the same as itself.
func TestSameComparesValues(t *testing.T) {
	nan := math.NaN()
	same := map[string][2]AnyEffect{
		"equal pointers":   {Pure(&statusError{1, "a"}), Pure(&statusError{1, "a"})},
		"a NaN":            {Pure(nan), Pure(nan)},
		"a NaN argument":   {Query[int]{SQL: "q", Args: []any{nan}}, Query[int]{SQL: "q", Args: []any{nan}}},
		"equal rows":       {Write[[]int]{Row: []int{1}}, Write[[]int]{Row: []int{1}}},
		"equal errors":     {Fail[int](&statusError{409, "taken"}), Fail[int](&statusError{409, "taken"})},
		"a nested NaN":     {Tx(Pure(map[string]float64{"x": nan})), Tx(Pure(map[string]float64{"x": nan}))},
		"a statement once": {Exec{SQL: "d", Args: []any{1}, One: true, Model: "User"}, Exec{SQL: "d", Args: []any{1}, One: true, Model: "User"}},
	}
	for name, pair := range same {
		if !Same(pair[0], pair[1]) {
			t.Errorf("%s: %q and %q are not Same; want them Same", name, Describe(pair[0]), Describe(pair[1]))
		}
	}
	different := map[string][2]AnyEffect{
		"arguments split by a space": {Query[int]{SQL: "q", Args: []any{"Ann Lee", "x"}}, Query[int]{SQL: "q", Args: []any{"Ann", "Lee x"}}},
		"an int and a text argument": {Query[int]{SQL: "q", Args: []any{1}}, Query[int]{SQL: "q", Args: []any{"1"}}},
		"One and bulk":               {Exec{SQL: "d", Args: []any{1}, One: true}, Exec{SQL: "d", Args: []any{1}}},
		"two Models":                 {Exec{SQL: "d", Model: "User"}, Exec{SQL: "d", Model: "Post"}},
		"two rows, nothing marked":   {Write[int]{Row: 1}, Write[int]{Row: 2}},
		"two statuses, one message":  {Fail[int](&statusError{409, "taken"}), Fail[int](&statusError{422, "taken"})},
		"two Pure types":             {Pure(1), Pure(int64(1))},
	}
	for name, pair := range different {
		if Same(pair[0], pair[1]) {
			t.Errorf("%s: %q and %q are Same; want them told apart", name, Describe(pair[0]), Describe(pair[1]))
		}
	}
	if got := Describe(Exec{SQL: "d", Args: []any{1}, One: true, Model: "User"}); got != "exec one of User d [1]" {
		t.Errorf("an Exec of one row describes as %q", got)
	}
	if got := Describe(Write[int]{Row: 3}); got != "write  [] row 3" {
		t.Errorf("a Write with no statement describes as %q; want the row it yields", got)
	}
	type node struct{ next *node }
	a, b := &node{}, &node{}
	a.next, b.next = a, b
	if !Equal(a, b) || Equal(func() {}, func() {}) || !Equal(nil, nil) || Equal(nil, 0) {
		t.Error("Equal disagrees with reflect.DeepEqual away from NaN")
	}
}

func TestPureAndFailEffects(t *testing.T) {
	ctx := context.Background()
	idle := &fakeExecutor{}
	if got, err := Pure("draft").perform(ctx, idle); err != nil || got != "draft" || len(idle.ran) != 0 || idle.txs != 0 {
		t.Errorf("Pure = %q, %v after %v in %d transactions; want its value with nothing performed", got, err, idle.ran, idle.txs)
	}
	refused := errors.New("only a draft is published")
	if _, err := Fail[string](refused).perform(ctx, idle); err != refused || len(idle.ran) != 0 {
		t.Errorf("Fail = %v after %v; want its error unchanged with nothing performed", err, idle.ran)
	}
	if _, err := Fail[string](nil).perform(ctx, idle); !errors.Is(err, ErrNoEffect) {
		t.Errorf("a Fail of no error = %v; want ErrNoEffect", err)
	}
	inv := &Invalid{Model: "Post"}
	x := joinExecutor{&fakeExecutor{affected: 1}}
	_, err := Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Fail[int64](inv) })).perform(ctx, x)
	var as *Invalid
	if !errors.As(err, &as) || as != inv || !slices.Equal(x.ran, []string{"tx a"}) || x.txs != 1 {
		t.Errorf("an Atomic whose next fails = %v after %v; want the next's error unwrapped from the one transaction", err, x.ran)
	}
	got, err := Atomic(Then(Exec{SQL: "a"}, func(n int64) Effect[int64] { return Pure(n + 1) })).perform(ctx, x)
	if err != nil || got != 2 {
		t.Errorf("an Atomic whose next yields a value = %d, %v; want the value", got, err)
	}
}

// TestNothingYieldsErrNoEffect proves spec §generated-query-surface
// on nil: a nil effect, next or f yields ErrNoEffect wherever it sits,
// and the effect that holds it performs none of its steps.
func TestNothingYieldsErrNoEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 1}
	cases := map[string]AnyEffect{
		"a Transaction with a nil step": Tx(Exec{SQL: "a"}, nil),
		"a Parallel with a nil step":    All(Exec{SQL: "a"}, nil),
		"a Then with no next":           Then[int64, int64](Exec{SQL: "a"}, nil),
		"a Map with no f":               Map[int64, int64](Exec{SQL: "a"}, nil),
	}
	for name, e := range cases {
		if _, err := PerformAny(ctx, x, e); !errors.Is(err, ErrNoEffect) {
			t.Errorf("%s = %v; want ErrNoEffect", name, err)
		}
	}
	if len(x.ran) != 0 || x.txs != 0 {
		t.Errorf("an effect holding nothing ran %v in %d transactions; want nothing", x.ran, x.txs)
	}
	if _, err := Perform[int](ctx, x, nil); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Perform of no effect = %v; want ErrNoEffect", err)
	}
	if _, err := PerformAny(ctx, x, nil); !errors.Is(err, ErrNoEffect) {
		t.Errorf("PerformAny of no effect = %v; want ErrNoEffect", err)
	}
}

// TestParallelInsideTransactionRunsInOrder proves what spec
// §generated-query-surface says of the runner: inside a Transaction or
// an Atomic a Parallel runs its steps in order on the one transaction,
// the first error stopping it before the next step starts, nested
// Parallels too, and a step's panic is raised as it is, on the
// goroutine performing it.
func TestParallelInsideTransactionRunsInOrder(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	steps := func() []AnyEffect {
		return []AnyEffect{Exec{SQL: "a"}, All(Exec{SQL: "b"}, Exec{SQL: "c"}), Exec{SQL: "d"}}
	}
	for name, e := range map[string]AnyEffect{
		"a Transaction": Tx(All(steps()...)),
		"an Atomic":     Atomic(All(steps()...)),
		"a Then":        Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[[]any] { return All(steps()[1:]...) })),
	} {
		x := joinExecutor{&fakeExecutor{affected: 1}}
		if _, err := PerformAny(ctx, x, e); err != nil || !slices.Equal(x.ran, []string{"tx a", "tx b", "tx c", "tx d"}) {
			t.Errorf("a Parallel inside %s = %v after %v; want every step in order", name, err, x.ran)
		}
	}
	x := joinExecutor{&fakeExecutor{affected: 1}}
	failing := []AnyEffect{Exec{SQL: "a"}, All(Exec{SQL: "b"}, Exec{Err: boom}, Exec{SQL: "c"}), Exec{SQL: "d"}}
	if _, err := Atomic(All(failing...)).perform(ctx, x); !errors.Is(err, boom) || !slices.Equal(x.ran, []string{"tx a", "tx b"}) {
		t.Errorf("a failing Parallel inside an Atomic = %v after %v; want the error with no step after it started", err, x.ran)
	}
	x = joinExecutor{&fakeExecutor{affected: 1}}
	broken := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{SQL: fmt.Sprint(firstOf(nil))} })
	raised := panicRecovered(func() { _, _ = PerformAny(ctx, x, Atomic(All(broken, Exec{SQL: "c"}))) })
	if _, ok := raised.(runtime.Error); !ok || !slices.Equal(x.ran, []string{"tx a"}) {
		t.Errorf("a panicking step of a Parallel inside an Atomic raised %T after %v; want the runtime error itself, no later step run", raised, x.ran)
	}
	pool := &fakeExecutor{affected: 1}
	if _, err := All(Exec{SQL: "a"}, Exec{SQL: "b"}).perform(ctx, pool); err != nil || pool.txs != 0 || len(pool.ran) != 2 {
		t.Errorf("a Parallel outside a transaction = %v after %v in %d transactions", err, pool.ran, pool.txs)
	}
}

type waitExecutor struct{ *fakeExecutor }

func (x waitExecutor) Exec(ctx context.Context, sql string, args []any) (int64, error) {
	if sql != "wait" {
		return x.fakeExecutor.Exec(ctx, sql, args)
	}
	<-ctx.Done()
	x.note("cancelled")
	return 0, ctx.Err()
}

func firstOf(ids []int64) int64 { return ids[0] }

func panicRecovered(f func()) (v any) {
	defer func() { v = recover() }()
	f()
	return nil
}

// TestParallelPanicRaisedOnTheCaller proves what spec
// §generated-query-surface says of a Parallel's panic: a step's panic cancels the other
// steps, waits for them to join, and is raised again on the goroutine
// that performs the Parallel, carrying the step's stack, once however
// deep the Parallels nest; ParallelMap does the same.
func TestParallelPanicRaisedOnTheCaller(t *testing.T) {
	ctx := context.Background()
	x := waitExecutor{&fakeExecutor{affected: 1}}
	broken := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{SQL: fmt.Sprint(firstOf(nil))} })
	for name, e := range map[string]AnyEffect{
		"a Parallel":        All(Exec{SQL: "wait"}, broken),
		"a nested Parallel": All(Exec{SQL: "wait"}, All(broken)),
	} {
		x.ran = nil
		err, ok := panicRecovered(func() { _, _ = PerformAny(ctx, x, e) }).(error)
		var re runtime.Error
		if !ok || !errors.As(err, &re) || !strings.Contains(err.Error(), "rt.firstOf") {
			t.Fatalf("%s whose step panics raised %v; want the runtime error with the step's stack", name, err)
		}
		if n := strings.Count(err.Error(), "raised again"); n != 1 {
			t.Errorf("%s raised a panic wrapped %d times; want once", name, n)
		}
		if !slices.Contains(x.ran, "cancelled") {
			t.Errorf("%s raised its panic after %v; want the waiting step cancelled and joined first", name, x.ran)
		}
	}
	err, ok := panicRecovered(func() {
		ParallelMap([]int64{1, 2}, func(n int64) int64 { return firstOf(make([]int64, n-1)) })
	}).(error)
	var re runtime.Error
	if !ok || !errors.As(err, &re) || !strings.Contains(err.Error(), "rt.firstOf") {
		t.Errorf("ParallelMap whose f panics raised %v; want the runtime error with its stack", err)
	}
}

// TestEffectKindsDescribed holds the sealed set of effects to its
// samples (spec §generated-query-surface): every type of the package
// that declares a method of the sealed set, perform, performAny or
// describe, on a value or a pointer receiver, has a sample here, so a
// kind that is only an AnyEffect, or one that embeds another and
// describes itself, is caught, and each sample describes by its kind.
func TestEffectKindsDescribed(t *testing.T) {
	samples := map[string]struct {
		e    AnyEffect
		want string
	}{
		"Query":        {Query[int]{SQL: "q"}, "query q []"},
		"Lookup":       {Lookup[int]{SQL: "l"}, "lookup l []"},
		"Write":        {Write[int]{SQL: "w"}, "write w []"},
		"Exec":         {Exec{SQL: "e"}, "exec e []"},
		"With":         {With[int, int, int, int]{Parent: Query[int]{SQL: "p"}, Child: "c"}, "with p []; children c []"},
		"Transaction":  {Tx(Exec{SQL: "e"}), "tx(exec e [])"},
		"Parallel":     {All(Exec{SQL: "e"}), "all(exec e [])"},
		"thenEffect":   {Then(Exec{SQL: "e"}, func(int64) Effect[int64] { return Exec{SQL: "f"} }), "then(exec e [])"},
		"mapEffect":    {Map(Exec{SQL: "e"}, func(n int64) int64 { return n }), "map(exec e [])"},
		"atomicEffect": {Atomic(Exec{SQL: "e"}), "atomic(exec e [])"},
		"pureEffect":   {Pure(7), "pure 7"},
		"failEffect":   {Fail[int](errors.New("refused")), "fail refused"},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	sealed := map[string]bool{"perform": true, "performAny": true, "describe": true}
	fset := token.NewFileSet()
	var declared []string
	for _, en := range entries {
		if !strings.HasSuffix(en.Name(), ".go") || strings.HasSuffix(en.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, en.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || !sealed[fd.Name.Name] {
				continue
			}
			recv := fd.Recv.List[0].Type
			if r, ok := recv.(*ast.StarExpr); ok {
				recv = r.X
			}
			switch r := recv.(type) {
			case *ast.IndexExpr:
				recv = r.X
			case *ast.IndexListExpr:
				recv = r.X
			}
			id, ok := recv.(*ast.Ident)
			if !ok {
				t.Fatalf("%s: the receiver of %s is %T, which the test cannot name", fset.Position(fd.Pos()), fd.Name.Name, recv)
			}
			declared = append(declared, id.Name)
		}
	}
	slices.Sort(declared)
	declared = slices.Compact(declared)
	if want := slices.Sorted(maps.Keys(samples)); !slices.Equal(declared, want) {
		t.Fatalf("the package declares the effects %v; the samples cover %v", declared, want)
	}
	for kind, sample := range samples {
		if got := Describe(sample.e); got != sample.want {
			t.Errorf("%s describes as %q; want %q", kind, got, sample.want)
		}
	}
}
