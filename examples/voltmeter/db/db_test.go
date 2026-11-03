package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
	_ "github.com/mattn/go-sqlite3"
)

var sizes = []int{2, 4, 8}

func open(t *testing.T) run.DB {
	t.Helper()
	d, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "voltmeter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := run.MigrateUp(context.Background(), d, db.Migrations); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return d
}

func do[T any](t *testing.T, d run.DB, e rt.Effect[T]) T {
	t.Helper()
	v, err := run.Run(context.Background(), d, e)
	if err != nil {
		t.Fatalf("%s: %v", rt.Describe(e), err)
	}
	return v
}

func refused(t *testing.T, err error) *rt.Invalid {
	t.Helper()
	var inv *rt.Invalid
	if !errors.As(err, &inv) {
		t.Fatalf("want *rt.Invalid, got %v", err)
	}
	return inv
}

func checks(inv *rt.Invalid) string {
	var out []string
	for _, f := range inv.Fields {
		out = append(out, f.Check+"("+strings.Join(f.Columns, ",")+")")
	}
	return strings.Join(out, " ")
}

type world struct {
	machine, idle db.Machine
	runs          []db.Run
	commits       []db.Commit
	budgets       []db.Budget
	dashboards    []db.Dashboard
}

func wall(phase, run, tables int) units.Millis {
	return units.Millis(float64(phase*10+tables) + float64(run)/10)
}

func seed(t *testing.T, d run.DB) world {
	t.Helper()
	var w world
	w.machine = do(t, d, db.MachineCreate(db.MachineNew().CPUSet("Test CPU 3000").CoresSet(8).MemoryGbSet(16).OsSet("linux").ArchSet("amd64").GoSet("go1.27.0")))
	w.idle = do(t, d, db.MachineCreate(db.MachineNew().CPUSet("Idle CPU").CoresSet(2).MemoryGbSet(4).OsSet("linux").ArchSet("arm64").GoSet("go1.27.0")))
	do(t, d, db.TagCreate(db.TagNew().SlugSet("release").NameSet("Release")))

	r1 := do(t, d, db.RunCreate(db.RunNew().CommitSet("AAAA1111").MachineIDSet(w.machine.ID).TagSet(rt.Some("baseline"))))
	r2 := do(t, d, db.RunCreate(db.RunNew().CommitSet("bbbb2222").DirtySet(true).MachineIDSet(w.machine.ID).TagSet(rt.Some("release")).BaselineIDSet(rt.Some(r1.ID)).NoteSet(rt.Some("after the fold"))))
	r3 := do(t, d, db.RunCreate(db.RunNew().CommitSet("cccc3333").MachineIDSet(w.machine.ID).BaselineIDSet(rt.Some(r1.ID))))
	w.runs = []db.Run{r1, r2, r3}

	var steps []rt.AnyEffect
	for _, r := range w.runs {
		for _, n := range sizes {
			steps = append(steps,
				db.MsLoadCreate(db.MsLoadNew().RunIDSet(r.ID).TablesSet(n).WallMsSet(wall(0, r.ID, n)).AllocMbSet(float64(n)*1.5).HeapMbSet(float64(n)*2).AllocsSet(n*100).SourceKbSet(float64(n)/2)),
				db.MsCheckCreate(db.MsCheckNew().RunIDSet(r.ID).TablesSet(n).WallMsSet(wall(1, r.ID, n)).AllocMbSet(float64(n)*1.5).HeapMbSet(float64(n)*2).AllocsSet(n*100).SourceKbSet(float64(n)/2)),
				db.MsVetCreate(db.MsVetNew().RunIDSet(r.ID).TablesSet(n).WallMsSet(wall(2, r.ID, n)).AllocMbSet(float64(n)*1.5).HeapMbSet(float64(n)*2).AllocsSet(n*100).SourceKbSet(float64(n)/2)),
				db.MsGenerateCreate(db.MsGenerateNew().RunIDSet(r.ID).TablesSet(n).WallMsSet(wall(3, r.ID, n)).AllocMbSet(float64(n)*1.5).HeapMbSet(float64(n)*2).AllocsSet(n*100).OutputMbSet(float64(n)/4).SourceKbSet(float64(n)/2)),
				db.KeystrokeCreate(db.KeystrokeNew().RunIDSet(r.ID).TablesSet(n).MsPerOpSet(float64(n)/10).MbPerOpSet(0.5).AllocsPerOpSet(n*7)),
			)
		}
	}
	for i, s := range []struct {
		route  string
		status int
		ms     float64
	}{{"/runs", 200, 1}, {"/runs", 200, 2}, {"/runs", 200, 3}, {"/runs/:id", 200, 4}, {"/missing", 404, 0.5}} {
		steps = append(steps, db.SampleCreate(db.SampleNew().RunIDSet(r1.ID).RouteSet(s.route).StatusSet(s.status).MsSet(s.ms).AtSet(1_700_000_000_000+i)))
	}
	do(t, d, rt.Tx(steps...))

	day := func(m time.Month) time.Time { return time.Date(2026, m, 1, 12, 0, 0, 0, time.UTC) }
	w.commits = []db.Commit{
		do(t, d, db.CommitCreate(db.CommitNew().ShaSet("aaaa1111").SubjectSet("Add the fold").AuthorSet("Ada").AtSet(day(time.January)))),
		do(t, d, db.CommitCreate(db.CommitNew().ShaSet("bbbb2222").SubjectSet("Remove the project element").AuthorSet("Grace").AtSet(day(time.February)))),
		do(t, d, db.CommitCreate(db.CommitNew().ShaSet("cccc3333").SubjectSet("Add autoincrement").AuthorSet("ADA").AtSet(day(time.March)))),
	}
	for i, c := range w.commits {
		do(t, d, db.LineCountCreate(db.LineCountNew().ShaSet(c.Sha).FixtureSet("itest/blog").VoltLinesSet(100+i).HandLinesSet(200+i).GenLinesSet(3000+i)))
	}
	do(t, d, db.LineCountCreate(db.LineCountNew().ShaSet("cccc3333").FixtureSet("examples/voltmeter").VoltLinesSet(400).HandLinesSet(500).GenLinesSet(5000)))

	w.budgets = []db.Budget{
		do(t, d, db.BudgetCreate(db.BudgetNew().PhaseSet(db.PhaseLoad).TablesSet(4).MaxMsSet(100))),
		do(t, d, db.BudgetCreate(db.BudgetNew().PhaseSet(db.PhaseGenerate).TablesSet(8).MaxMsSet(5).StateSet(db.HealthBreached))),
	}

	overview := do(t, d, db.DashboardCreate(db.DashboardNew().OwnerIDSet(1).NameSet("Overview")))
	empty := do(t, d, db.DashboardCreate(db.DashboardNew().OwnerIDSet(1).NameSet("Empty")))
	graces := do(t, d, db.DashboardCreate(db.DashboardNew().OwnerIDSet(2).NameSet("Generate only")))
	w.dashboards = []db.Dashboard{overview, empty, graces}
	do(t, d, rt.Tx(
		db.WidgetCreate(db.WidgetNew().DashboardIDSet(overview.ID).PositionSet(1).PhaseSet(db.PhaseLoad).TablesSet(4)),
		db.WidgetCreate(db.WidgetNew().DashboardIDSet(overview.ID).PositionSet(2).PhaseSet(db.PhaseCheck).TablesSet(8)),
		db.WidgetCreate(db.WidgetNew().DashboardIDSet(graces.ID).PositionSet(1).PhaseSet(db.PhaseGenerate).TablesSet(2)),
	))
	return w
}

func ids[T any](rows []T, id func(T) int) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = id(r)
	}
	return out
}

func equal(t *testing.T, what string, got, want []int) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}
