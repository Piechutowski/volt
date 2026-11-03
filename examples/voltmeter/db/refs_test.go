package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

// TestRefActions proves the referential actions as SQLite performs them
// (spec §relationships-ref): a tag's rename follows into its runs and its
// deletion clears them, a deleted baseline clears its followers' column,
// and a deleted machine takes its runs and, through them, every row that
// references a run.
func TestRefActions(t *testing.T) {
	ctx := context.Background()
	d := dbOpen(t)
	w := worldSeed(t, d)
	r1, r2, r3 := w.runs[0], w.runs[1], w.runs[2]

	tag := do(t, d, db.TagGet("release"))
	do(t, d, db.TagUpdate(tag.Changeset().SlugSet("v1")))
	if got := do(t, d, db.RunGet(r2.ID)); got.Tag.V != "v1" {
		t.Errorf("update: cascade carried the new slug as %+v", got.Tag)
	}
	do(t, d, db.TagDelete("v1"))
	if got := do(t, d, db.RunGet(r2.ID)); got.Tag.Valid {
		t.Errorf("delete: set_null left %+v", got.Tag)
	}

	do(t, d, db.BreachCreate(db.BreachNew().BudgetIDSet(w.budgets[1].ID).RunIDSet(r1.ID).WallMsSet(9)))
	do(t, d, db.RunDelete(r1.ID))
	for _, r := range []db.Run{r2, r3} {
		if got := do(t, d, db.RunGet(r.ID)); got.BaselineID.Valid {
			t.Errorf("run %d still names the deleted baseline: %+v", r.ID, got.BaselineID)
		}
	}
	if n := do(t, d, db.SampleCount(db.SampleRunID.Eq(r1.ID))); n != 0 {
		t.Errorf("%d samples outlived their run", n)
	}
	if n := do(t, d, db.BreachCount()); n != 0 {
		t.Errorf("%d breaches outlived their run", n)
	}

	do(t, d, db.MachineDelete(w.machine.ID))
	counts := do(t, d, rt.All(db.RunCount(), db.MsLoadCount(), db.MsCheckCount(), db.MsVetCount(), db.MsGenerateCount(), db.KeystrokeCount(), db.MachineCount()))
	for i, want := range []int64{0, 0, 0, 0, 0, 0, 1} {
		if counts[i].(int64) != want {
			t.Errorf("after the machine's delete, count %d = %v, want %d", i, counts[i], want)
		}
	}

	if _, err := run.Run(ctx, d, db.WidgetCreate(db.WidgetNew().DashboardIDSet(999).PositionSet(1).PhaseSet(db.PhaseVet).TablesSet(2))); err == nil {
		t.Error("a widget on no dashboard was stored")
	}
	do(t, d, db.DashboardDelete(w.dashboards[0].ID))
	if n := do(t, d, db.WidgetCount(db.WidgetDashboardID.Eq(w.dashboards[0].ID))); n != 0 {
		t.Errorf("%d widgets outlived their dashboard", n)
	}
}

// TestAutoincrementNeverReusesARunID proves runs, the one table with
// autoincrement, never assigns a deleted id again, while samples, whose
// plain database-assigned key is SQLite's rowid, takes the largest id
// again once its row is gone (spec §columns, §mapping-to-sqlite-ddl).
func TestAutoincrementNeverReusesARunID(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	newRun := func() db.Run {
		t.Helper()
		return do(t, d, db.RunCreate(db.RunNew().CommitSet("eeee5555").MachineIDSet(w.idle.ID)))
	}
	last := newRun()
	do(t, d, db.RunDelete(last.ID))
	next := newRun()
	if next.ID != last.ID+1 {
		t.Errorf("after deleting run %d the next run is %d, want %d: AUTOINCREMENT never reuses an id", last.ID, next.ID, last.ID+1)
	}

	newSample := func() db.Sample {
		t.Helper()
		return do(t, d, db.SampleCreate(db.SampleNew().RunIDSet(next.ID).RouteSet("/").StatusSet(200).MsSet(1).AtSet(0)))
	}
	s := newSample()
	do(t, d, db.SampleDelete(s.ID))
	if again := newSample(); again.ID != s.ID {
		t.Errorf("a plain rowid key takes the largest id plus one, so %d comes back; got %d", s.ID, again.ID)
	}

	explicit := do(t, d, db.RunCreate(db.RunNew().IDSet(1000).CommitSet("ffff").MachineIDSet(w.idle.ID)))
	if explicit.ID != 1000 || newRun().ID != 1001 {
		t.Errorf("an explicit id is stored and raises the high-water mark: %d", explicit.ID)
	}
	if _, err := run.Run(context.Background(), d, db.RunCreate(db.RunNew().IDSet(1000).CommitSet("ffff").MachineIDSet(w.idle.ID))); !errors.As(err, new(*rt.Invalid)) {
		t.Errorf("a second run 1000 = %v, want a refused write", err)
	}
}
