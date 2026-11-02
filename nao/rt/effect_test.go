package rt

import (
	"context"
	"errors"
	"reflect"
	"slices"
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
