package db_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

// TestTxIsAllOrNothing proves a transaction that fails at its last step
// leaves no trace of the steps before it, and one that succeeds yields
// every step's result in order (spec §generated-query-surface): the
// budget's state and its breach are written together or not at all.
func TestTxIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	w := seed(t, d)
	b := w.budgets[0]

	breach := func(runID int) rt.Transaction {
		return rt.Tx(
			db.BudgetUpdate(b.Changeset().StateSet(db.HealthBreached)),
			db.BreachCreate(db.BreachNew().BudgetIDSet(b.ID).RunIDSet(runID).WallMsSet(120)),
		)
	}
	if _, err := run.Run(ctx, d, breach(999)); !errors.As(err, new(*rt.Invalid)) {
		t.Fatalf("a breach of no run = %v, want the foreign key's refusal", err)
	}
	if got := do(t, d, db.BudgetGet(b.ID)); got.State != db.HealthOK {
		t.Errorf("the rolled-back transaction left the budget %q", got.State)
	}
	if n := do(t, d, db.BreachCount()); n != 0 {
		t.Errorf("the rolled-back transaction left %d breaches", n)
	}

	out := do(t, d, breach(w.runs[0].ID))
	if got := out[0].(db.Budget); got.State != db.HealthBreached {
		t.Errorf("step 0 yielded %+v", got)
	}
	if got := out[1].(db.Breach); got.RunID != w.runs[0].ID || got.CreatedAt.IsZero() {
		t.Errorf("step 1 yielded %+v", got)
	}

	nested := rt.Tx(db.TagCreate(db.TagNew().SlugSet("nested").NameSet("Nested")), rt.Tx(db.TagDelete("no such tag")))
	if _, err := run.Run(ctx, d, nested); !errors.Is(err, rt.ErrNotFound) {
		t.Fatalf("nested transaction = %v", err)
	}
	if _, err := run.Run(ctx, d, db.TagGet("nested")); !errors.Is(err, rt.ErrNotFound) {
		t.Error("a transaction inside one joins it, so its failure rolls the outer back")
	}
}

// TestAllKeepsResultOrder proves rt.All yields every step's result at its
// own position, whatever order the steps finish in, and returns the first
// error alone (spec §values-and-effects).
func TestAllKeepsResultOrder(t *testing.T) {
	d := open(t)
	w := seed(t, d)
	for range 20 {
		out := do(t, d, rt.All(
			db.RunGet(w.runs[2].ID),
			db.MachineList(),
			db.SampleLatency(w.runs[0].ID),
			db.MsGeneratePerSize([]int{8}),
			db.DashboardBoards(1),
			db.UserCount(),
		))
		if out[0].(db.Run).ID != w.runs[2].ID || len(out[1].([]db.Machine)) != 2 || out[2].(db.Latency).N != 5 ||
			out[3].([]db.PerSize)[0].Tables != 8 || len(out[4].([]db.DashboardWithWidgets)) != 2 || out[5].(int64) != 3 {
			t.Fatalf("All = %+v", out)
		}
	}
	if _, err := run.Run(context.Background(), d, rt.All(db.UserList(), db.RunGet(12345))); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("All with a missing row = %v", err)
	}
}

// TestStatementCacheAndLogging proves identical statement shapes prepare
// once, a dynamic query of new arguments included, and the logger of
// WithLogger gets one debug line per statement with its text and
// duration (spec §generated-query-surface, the runner).
func TestStatementCacheAndLogging(t *testing.T) {
	d := open(t)
	w := seed(t, d)

	var buf bytes.Buffer
	logged := d.WithLogger(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	before := logged.Statements()
	do(t, logged, db.MsLoadRows(w.runs[0].ID))
	do(t, logged, db.MsLoadRows(w.runs[1].ID))
	do(t, logged, db.SampleQuery(db.SampleRunID.Eq(1), db.SampleLimit(2)))
	do(t, logged, db.SampleQuery(db.SampleRunID.Eq(2), db.SampleLimit(9)))
	if got := logged.Statements() - before; got != 2 {
		t.Errorf("four statements of two shapes prepared %d new statements, want 2", got)
	}
	if d.Statements() != logged.Statements() {
		t.Error("WithLogger shares the cache of the DB it came from")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("logged %d lines, want one per statement:\n%s", len(lines), buf.String())
	}
	for i, l := range lines {
		var rec struct {
			Level    string
			Msg      string
			SQL      string `json:"sql"`
			Duration int64  `json:"duration"`
		}
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Level != "DEBUG" || rec.Msg != "statement" || rec.Duration <= 0 {
			t.Errorf("line %d = %s", i, l)
		}
		if wantSQL := []string{db.MsLoadRows(0).SQL, db.MsLoadRows(0).SQL, db.SampleQuery(db.SampleRunID.Eq(1), db.SampleLimit(2)).SQL}; i < 3 && rec.SQL != wantSQL[i] {
			t.Errorf("line %d logs %q, want %q", i, rec.SQL, wantSQL[i])
		}
	}

	buf.Reset()
	do(t, d, db.UserCount())
	if buf.Len() != 0 {
		t.Error("the DB WithLogger came from kept its own logger")
	}
}
