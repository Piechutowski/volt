package rt

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"reflect"
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
	got, err := Query[any]{SQL: "SELECT 1", Scan: scanAny}.Perform(ctx, x)
	if err != nil || !reflect.DeepEqual(got, []any{1, 2}) {
		t.Errorf("Query = %v, %v", got, err)
	}
	if got, err := (Query[any]{Scan: scanAny}).Perform(ctx, x); err != nil || got != nil {
		t.Errorf("a Query with no statement = %v, %v; want nil", got, err)
	}
	bad := errors.New("render")
	if _, err := (Query[any]{SQL: "x", Err: bad, Scan: scanAny}).Perform(ctx, x); !errors.Is(err, bad) {
		t.Errorf("a Query carrying an error = %v", err)
	}
	if !slices.Equal(x.ran, []string{"SELECT 1"}) {
		t.Errorf("statements run: %v", x.ran)
	}
}

func TestLookupAndWriteEffects(t *testing.T) {
	ctx := context.Background()
	miss := &fakeExecutor{rowErr: ErrNotFound}
	if _, err := (Lookup[any]{SQL: "SELECT", Scan: scanAny}).Perform(ctx, miss); !errors.Is(err, ErrNotFound) {
		t.Errorf("Lookup miss = %v", err)
	}
	inv := &Invalid{Model: "Post"}
	idle := &fakeExecutor{}
	if _, err := (Write[any]{SQL: "INSERT", Err: inv}).Perform(ctx, idle); !errors.Is(err, inv) || len(idle.ran) != 0 {
		t.Errorf("an invalid Write = %v after %v; want its error with nothing run", err, idle.ran)
	}
	if got, err := (Write[any]{Row: "untouched"}).Perform(ctx, miss); err != nil || got != "untouched" {
		t.Errorf("a Write with nothing marked = %v, %v; want the row back", got, err)
	}
}

func TestExecEffect(t *testing.T) {
	ctx := context.Background()
	none := &fakeExecutor{affected: 0}
	if _, err := (Exec{SQL: "DELETE", One: true}).Perform(ctx, none); !errors.Is(err, ErrNotFound) {
		t.Errorf("a keyed delete of nothing = %v", err)
	}
	if n, err := (Exec{SQL: "DELETE"}).Perform(ctx, none); err != nil || n != 0 {
		t.Errorf("a bulk delete of nothing = %d, %v", n, err)
	}
	refused := &fakeExecutor{execErr: errors.New("FOREIGN KEY constraint failed")}
	var inv *Invalid
	if _, err := (Exec{SQL: "DELETE", Model: "Post"}).Perform(ctx, refused); !errors.As(err, &inv) || inv.Model != "Post" {
		t.Errorf("a refused keyed delete = %v; want *Invalid", err)
	}
}

func TestTransactionEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 3}
	out, err := Tx(Exec{SQL: "a"}, Exec{SQL: "b"}).Perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(3), int64(3)}) || x.txs != 1 {
		t.Errorf("Tx = %v, %v in %d transactions", out, err, x.txs)
	}
	boom := errors.New("boom")
	out, err = Tx(Exec{SQL: "a"}, Exec{Err: boom}, Exec{SQL: "c"}).Perform(ctx, x)
	if !errors.Is(err, boom) || out != nil || slices.Contains(x.ran, "c") {
		t.Errorf("Tx with a failing step = %v, %v, ran %v; want the error and no later step", out, err, x.ran)
	}
}

func TestParallelEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 1}
	out, err := All(Exec{SQL: "a"}, Exec{SQL: "b"}, Exec{SQL: "c"}).Perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(1), int64(1), int64(1)}) || len(x.ran) != 3 {
		t.Errorf("All = %v, %v after %v", out, err, x.ran)
	}
	boom := errors.New("boom")
	if _, err := All(Exec{SQL: "a"}, Exec{Err: boom}).Perform(ctx, x); !errors.Is(err, boom) {
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
	got, err := Then(Exec{SQL: "a"}, func(n int64) Effect[int64] { return Exec{SQL: fmt.Sprintf("b %d", n)} }).Perform(ctx, x)
	if err != nil || got != 2 || !slices.Equal(x.ran, []string{"a", "b 2"}) || x.txs != 0 {
		t.Errorf("Then = %d, %v after %v in %d transactions; want the second statement built from the first's result, outside any transaction", got, err, x.ran, x.txs)
	}
	boom := errors.New("boom")
	called := false
	idle := &fakeExecutor{}
	if _, err := Then(Exec{Err: boom}, func(int64) Effect[int64] { called = true; return Exec{SQL: "b"} }).Perform(ctx, idle); !errors.Is(err, boom) || called || len(idle.ran) != 0 {
		t.Errorf("Then over a failing effect = %v, next called %v, ran %v; want the error and nothing else", err, called, idle.ran)
	}
	half := &fakeExecutor{affected: 1}
	if _, err := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{Err: boom} }).Perform(ctx, half); !errors.Is(err, boom) || !slices.Equal(half.ran, []string{"a"}) {
		t.Errorf("Then whose second step fails = %v after %v; want the error with the first step run", err, half.ran)
	}
	if _, err := Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return nil }).Perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Then whose next returns no effect = %v; want ErrNoEffect", err)
	}
	if _, err := Then(nil, func(int64) Effect[int64] { return Exec{SQL: "b"} }).Perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Then over no effect = %v; want ErrNoEffect", err)
	}
}

func TestMapEffect(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecutor{affected: 2}
	got, err := Map(Exec{SQL: "a"}, func(n int64) string { return fmt.Sprint(n * 10) }).Perform(ctx, x)
	if err != nil || got != "20" {
		t.Errorf("Map = %q, %v; want f of the result", got, err)
	}
	inv := &Invalid{Model: "Post"}
	called := false
	if _, err := Map(Write[int]{Err: inv}, func(int) int { called = true; return 1 }).Perform(ctx, x); !errors.Is(err, inv) || called {
		t.Errorf("Map over a failing effect = %v, f called %v; want the error unchanged and f never called", err, called)
	}
	if _, err := Map(nil, func(int) int { return 1 }).Perform(ctx, x); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Map over no effect = %v; want ErrNoEffect", err)
	}
}

func TestAtomicEffect(t *testing.T) {
	ctx := context.Background()
	x := joinExecutor{&fakeExecutor{affected: 1}}
	nested := Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[[]any] { return Tx(Exec{SQL: "b"}, Atomic(Exec{SQL: "c"})) }))
	out, err := nested.Perform(ctx, x)
	if err != nil || !reflect.DeepEqual(out, []any{int64(1), int64(1)}) || x.txs != 1 || !slices.Equal(x.ran, []string{"tx a", "tx b", "tx c"}) {
		t.Errorf("Atomic = %v, %v after %v in %d transactions; want every step on the one transaction", out, err, x.ran, x.txs)
	}
	inv := &Invalid{Model: "Post"}
	failing := joinExecutor{&fakeExecutor{affected: 1}}
	got, err := Atomic(Then(Exec{SQL: "a"}, func(int64) Effect[int64] { return Exec{Err: inv} })).Perform(ctx, failing)
	var as *Invalid
	if !errors.As(err, &as) || as != inv || got != 0 {
		t.Errorf("Atomic with a failing step = %d, %v; want the step's *Invalid unwrapped", got, err)
	}
	parallel := joinExecutor{&fakeExecutor{affected: 1}}
	if _, err := Atomic(All(Exec{SQL: "a"}, Exec{SQL: "b"})).Perform(ctx, parallel); err != nil || parallel.txs != 1 || !slices.Equal(slices.Sorted(slices.Values(parallel.ran)), []string{"tx a", "tx b"}) {
		t.Errorf("a Parallel inside an Atomic = %v after %v in %d transactions; want both steps on the one transaction", err, parallel.ran, parallel.txs)
	}
	if _, err := Atomic[int](nil).Perform(ctx, x); !errors.Is(err, ErrNoEffect) {
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
}

// TestEffectKindsDescribed holds the sealed set of effects to Describe,
// the one switch that walks them (spec §generated-query-surface): every
// type of the package that declares effect() has a sample here, and
// each sample describes by its kind, never by its Go type's name.
func TestEffectKindsDescribed(t *testing.T) {
	samples := map[string]AnyEffect{
		"Query":        Query[int]{SQL: "q"},
		"Lookup":       Lookup[int]{SQL: "l"},
		"Write":        Write[int]{SQL: "w"},
		"Exec":         Exec{SQL: "e"},
		"With":         With[int, int, int, int]{Parent: Query[int]{SQL: "p"}, Child: "c"},
		"Transaction":  Tx(Exec{SQL: "e"}),
		"Parallel":     All(Exec{SQL: "e"}),
		"thenEffect":   Then(Exec{SQL: "e"}, func(int64) Effect[int64] { return Exec{SQL: "f"} }),
		"mapEffect":    Map(Exec{SQL: "e"}, func(n int64) int64 { return n }),
		"atomicEffect": Atomic(Exec{SQL: "e"}),
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
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
			if !ok || fd.Recv == nil || fd.Name.Name != "effect" {
				continue
			}
			recv := fd.Recv.List[0].Type
			switch r := recv.(type) {
			case *ast.IndexExpr:
				recv = r.X
			case *ast.IndexListExpr:
				recv = r.X
			}
			declared = append(declared, recv.(*ast.Ident).Name)
		}
	}
	slices.Sort(declared)
	if want := slices.Sorted(maps.Keys(samples)); !slices.Equal(declared, want) {
		t.Fatalf("the package declares the effects %v; the samples cover %v", declared, want)
	}
	for kind, e := range samples {
		if got := Describe(e); got == fmt.Sprintf("%T", e) || strings.Contains(got, "%!") {
			t.Errorf("%s describes as %q, its Go type", kind, got)
		}
	}
}
