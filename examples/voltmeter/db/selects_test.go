package db_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

// TestSelectsOverThePhaseGroup proves one select of the phases group
// mints one function per member, each reading its own table (spec
// §groups, §selects-over-groups), and that the set term of front leaves
// ms_generate out.
func TestSelectsOverThePhaseGroup(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	r1, r2 := w.runs[0], w.runs[1]

	load := do(t, d, db.MsLoadRows(r2.ID))
	gen := do(t, d, db.MsGenerateRows(r2.ID))
	equal(t, "MsLoadRows tables", ids(load, func(r db.MsLoad) int { return r.Tables }), sizes)
	if len(gen) != 3 || gen[2].WallMs != wall(3, r2.ID, 8) || gen[2].OutputMb != 2 || load[2].OutputMb != 0 {
		t.Errorf("MsGenerateRows = %+v, MsLoadRows = %+v", gen, load)
	}
	if len(do(t, d, db.MsCheckRows(r1.ID+100))) != 0 {
		t.Error("MsCheckRows of no run returned rows")
	}

	sized := do(t, d, db.MsCheckSized(r1.ID, []int{8, 2}))
	equal(t, "MsCheckSized tables", ids(sized, func(r db.MsCheck) int { return r.Tables }), []int{2, 8})
	if got := do(t, d, db.MsVetSized(r1.ID, nil)); len(got) != 0 {
		t.Errorf("an empty list matches no row, got %+v", got)
	}

	trend := do(t, d, db.MsVetTrend(4))
	equal(t, "MsVetTrend runs", ids(trend, func(r db.MsVet) int { return r.RunID }), ids(w.runs, func(r db.Run) int { return r.ID }))
	if trend[1].WallMs != wall(2, r2.ID, 4) || trend[1].WallMs.String() != "24.2 ms" {
		t.Errorf("trend[1].WallMs = %v", trend[1].WallMs)
	}
}

// TestProjections proves an explicit list mints one shared row type for
// every member and a star with exclusions a derivative per member (spec
// §selects-over-groups), and that the predicate's negation and its float
// parameter bind.
func TestProjections(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	r := w.runs[0]

	var briefs []db.Brief = do(t, d, db.MsLoadBrief(r.ID))
	briefs = append(briefs, do(t, d, db.MsGenerateBrief(r.ID))...)
	if len(briefs) != 6 || briefs[0] != (db.Brief{RunID: r.ID, Tables: 2, WallMs: wall(0, r.ID, 2)}) || briefs[5].WallMs != wall(3, r.ID, 8) {
		t.Errorf("Brief rows = %+v", briefs)
	}

	lean := do(t, d, db.MsCheckLean(r.ID, 5))
	if len(lean) != 1 || lean[0] != (db.MsCheckLeanRow{RunID: r.ID, Tables: 2, WallMs: wall(1, r.ID, 2), AllocMb: 3, HeapMb: 4, Allocs: 200}) {
		t.Errorf("MsCheckLean(not heavy over 5 MB) = %+v, want the 2-table row alone", lean)
	}
	if got := do(t, d, db.MsVetLean(r.ID, 100)); len(got) != 3 {
		t.Errorf("MsVetLean under a high ceiling = %+v, want every size", got)
	}

	dens := do(t, d, db.MsGenerateDensity(r.ID))
	for _, x := range dens {
		if x.AllocsPerTable != 100 || x.AllocPerSource != 3072 {
			t.Errorf("Density = %+v, want 100 allocations per table and 3072 MiB allocated per MiB of source", x)
		}
	}
}

// TestAggregates proves group: yields one row per group with every
// aggregate (spec §selects-over-groups), and that an aggregate without
// group: is a one-row Lookup whose nullable aggregates read NULL over no
// rows.
func TestAggregates(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)

	per := do(t, d, db.MsLoadPerSize([]int{2, 4}))
	want := []db.PerSize{
		{Tables: 2, N: 3, MeanAlloc: 3, LeastAlloc: 3, PeakHeap: 4, TotalAllocs: 600},
		{Tables: 4, N: 3, MeanAlloc: 6, LeastAlloc: 6, PeakHeap: 8, TotalAllocs: 1200},
	}
	if len(per) != 2 || per[0] != want[0] || per[1] != want[1] {
		t.Errorf("MsLoadPerSize = %+v, want %+v", per, want)
	}

	lat := do(t, d, db.SampleLatency(w.runs[0].ID))
	if lat.N != 5 || lat.Mean.V != 2.1 || lat.Best.V != 0.5 || lat.Worst.V != 4 || !lat.Mean.Valid {
		t.Errorf("SampleLatency = %+v", lat)
	}
	none := do(t, d, db.SampleLatency(w.runs[2].ID))
	if none.N != 0 || none.Mean.Valid || none.Worst.Valid {
		t.Errorf("SampleLatency of a run without samples = %+v, want n 0 and NULL aggregates", none)
	}

	routes := do(t, d, db.SamplePerRoute(w.runs[0].ID))
	if len(routes) != 3 || routes[0] != (db.PerRoute{Route: "/missing", N: 1, Mean: 0.5, Worst: 0.5}) || routes[1] != (db.PerRoute{Route: "/runs", N: 3, Mean: 2, Worst: 3}) {
		t.Errorf("SamplePerRoute = %+v", routes)
	}
	fail := do(t, d, db.SampleFailures(w.runs[0].ID))
	if len(fail) != 1 || fail[0].Status != 404 {
		t.Errorf("SampleFailures = %+v", fail)
	}
}

// TestJoins proves a required relation joins inner and an optional one
// left, so a run without a tag keeps its row with a NULL name, that a
// reverse relation joins left and counts zero for a machine without runs,
// and that a renamed reverse relation joins a table to itself (spec
// §selects-over-groups, §relationships-ref).
func TestJoins(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	r1, r2, r3 := w.runs[0], w.runs[1], w.runs[2]

	listed := do(t, d, db.RunListed())
	equal(t, "RunListed ids", ids(listed, func(l db.Listed) int { return l.ID }), []int{r3.ID, r2.ID, r1.ID})
	if listed[2].TagName.V != "Baseline" || listed[1].TagName.V != "Release" || listed[0].TagName.Valid {
		t.Errorf("tag names = %+v", listed)
	}
	if listed[2].MachineCPU != "Test CPU 3000" || listed[2].MachineOs != "linux" || listed[2].Commit != "aaaa1111" || listed[1].Note.V != "after the fold" || !listed[1].Dirty {
		t.Errorf("RunListed[2] = %+v", listed[2])
	}

	fleet := do(t, d, db.MachineFleet())
	if len(fleet) != 2 || fleet[0] != (db.Fleet{ID: w.machine.ID, CPU: "Test CPU 3000", RunCount: 3}) || fleet[1] != (db.Fleet{ID: w.idle.ID, CPU: "Idle CPU", RunCount: 0}) {
		t.Errorf("MachineFleet = %+v", fleet)
	}

	succ := do(t, d, db.RunSuccessors(r1.ID))
	if len(succ) != 2 || succ[0].FollowersID.V != r2.ID || succ[1].FollowersCommit.V != "cccc3333" {
		t.Errorf("RunSuccessors(r1) = %+v", succ)
	}
	lone := do(t, d, db.RunSuccessors(r3.ID))
	if len(lone) != 1 || lone[0].ID != r3.ID || lone[0].FollowersID.Valid {
		t.Errorf("RunSuccessors(r3) = %+v, want r3 once with no follower", lone)
	}

	thesis := do(t, d, db.LineCountThesis("itest/blog"))
	if len(thesis) != 3 || thesis[0].CommitSubject != "Add the fold" || thesis[2].Sha != "cccc3333" || thesis[2].GenLines != 3002 || !thesis[0].CommitAt.Before(thesis[1].CommitAt) {
		t.Errorf("LineCountThesis = %+v", thesis)
	}
}

// TestPredicateForms proves like, is null, in over a list of a value set,
// a declared type that binds through its Valuer, and a multi-column
// equality (spec §predicates, §types).
func TestPredicateForms(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)

	adds := do(t, d, db.CommitMatching("Add%"))
	if len(adds) != 2 || adds[0].Sha != "cccc3333" || adds[1].Sha != "aaaa1111" {
		t.Errorf("CommitMatching(Add%%) = %+v", adds)
	}
	untagged := do(t, d, db.RunUntagged())
	equal(t, "RunUntagged", ids(untagged, func(r db.Run) int { return r.ID }), []int{w.runs[2].ID})

	mixed := do(t, d, db.RunOfCommit(units.Commit("AAAA1111")))
	if len(mixed) != 1 || mixed[0].ID != w.runs[0].ID || mixed[0].Commit != "aaaa1111" {
		t.Errorf("RunOfCommit binds through units.Commit's Valuer, lower case: %+v", mixed)
	}

	breached := do(t, d, db.BudgetByState([]db.Health{db.HealthBreached}))
	if len(breached) != 1 || breached[0].Phase != db.PhaseGenerate {
		t.Errorf("BudgetByState(breached) = %+v", breached)
	}
	if all := do(t, d, db.BudgetByState([]db.Health{db.HealthOK, db.HealthBreached})); len(all) != 2 || all[0].State != db.HealthOK {
		t.Errorf("BudgetByState(both) = %+v", all)
	}
	if over := do(t, d, db.BudgetOver(db.PhaseLoad, 4, 150)); len(over) != 1 || over[0].ID != w.budgets[0].ID {
		t.Errorf("BudgetOver(load, 4, 150) = %+v", over)
	}
	if over := do(t, d, db.BudgetOver(db.PhaseLoad, 4, 50)); len(over) != 0 {
		t.Errorf("BudgetOver under the ceiling = %+v", over)
	}

	m := w.machine
	found := do(t, d, db.MachineIdentified(m.CPU, m.Cores, m.MemoryGb, m.Os, m.Arch, m.Go))
	if len(found) != 1 || found[0].ID != m.ID || found[0].CreatedAt.IsZero() {
		t.Errorf("MachineIdentified = %+v", found)
	}
}

// TestWithNestsChildren proves with yields each parent with its children
// in key order, an empty slice for a parent without any, and one parent
// for a key lookup (spec §selects-over-groups).
func TestWithNestsChildren(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)

	boards := do(t, d, db.DashboardBoards(1))
	if len(boards) != 2 || boards[0].Name != "Empty" || boards[1].Name != "Overview" {
		t.Fatalf("DashboardBoards(1) = %+v", boards)
	}
	if boards[0].Widgets == nil || len(boards[0].Widgets) != 0 {
		t.Errorf("a parent without children carries an empty slice, got %#v", boards[0].Widgets)
	}
	equal(t, "Overview positions", ids(boards[1].Widgets, func(x db.Widget) int { return x.Position }), []int{1, 2})
	one := do(t, d, db.DashboardBoard(w.dashboards[2].ID))
	if len(one) != 1 || len(one[0].Widgets) != 1 || one[0].Widgets[0].Phase != db.PhaseGenerate {
		t.Errorf("DashboardBoard = %+v", one)
	}
	if got := do(t, d, db.DashboardBoards(3)); len(got) != 0 {
		t.Errorf("DashboardBoards of an owner without boards = %+v", got)
	}
}

// TestPredicateAlgebra proves the predicate forms the other selects do
// not use (spec §predicates, §groups): or under parentheses, !=, <=, an
// in over a literal list, is not null, a pred built from preds, and a
// select over a group written as a union.
func TestPredicateAlgebra(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	r1, r2, r3 := w.runs[0].ID, w.runs[1].ID, w.runs[2].ID

	vet := do(t, d, db.MsVetSmallest(r1, 4))
	gen := do(t, d, db.MsGenerateSmallest(r1, 4))
	equal(t, "MsVetSmallest(r1, 4) tables", ids(vet, func(m db.MsVet) int { return m.Tables }), []int{2, 4})
	equal(t, "MsGenerateSmallest(r1, 4) tables", ids(gen, func(m db.MsGenerate) int { return m.Tables }), []int{2, 4})
	if gen[1].WallMs != wall(3, r1, 4) || vet[0].RunID != r1 {
		t.Errorf("smallest rows = %+v, %+v", vet, gen)
	}

	slow := do(t, d, db.SampleSlow(r1, 2.5))
	if got := ids(slow, func(s db.Sample) int { return int(s.Ms * 10) }); !slices.Equal(got, []int{30, 40, 5}) {
		t.Errorf("SampleSlow(r1, 2.5) took %v tenths, want 3 ms, 4 ms and the 404", got)
	}
	if rows := do(t, d, db.SampleServerErrors(r1)); len(rows) != 0 {
		t.Errorf("server errors before any = %+v", rows)
	}
	do(t, d, db.SampleCreate(db.SampleNew().RunIDSet(r1).RouteSet("/runs").StatusSet(503).MsSet(9).AtSet(1)))
	do(t, d, db.SampleCreate(db.SampleNew().RunIDSet(r1).RouteSet("/runs").StatusSet(501).MsSet(9).AtSet(2)))
	if rows := do(t, d, db.SampleServerErrors(r1)); len(rows) != 1 || rows[0].Status != 503 {
		t.Errorf("SampleServerErrors keeps 500, 502, 503 and 504 only: %+v", rows)
	}

	others := do(t, d, db.RunOthers(w.machine.ID, r1))
	equal(t, "RunOthers(machine, r1)", ids(others, func(r db.Run) int { return r.ID }), []int{r3, r2})
	annotated := do(t, d, db.RunAnnotated())
	equal(t, "RunAnnotated", ids(annotated, func(r db.Run) int { return r.ID }), []int{r2})
}

// TestSelectFormsTheOthersLeaveOut proves the forms of a predicate and a
// projection the other selects do not use, value by value (spec
// §predicates, §selects-over-groups): a qualified column of the joined
// machine, two columns compared, a negated membership of string
// literals, true, a float and an int literal, a string literal pattern
// and inequality, a date/time bound, arithmetic with a parameter and
// both additive operators, min and max of text and time, and a with
// select with a projection.
func TestSelectFormsTheOthersLeaveOut(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)
	r1, r2 := w.runs[0], w.runs[1]

	on := do(t, d, db.RunOnSystem("linux"))
	if !slices.Equal(ids(on, func(r db.OnSystem) int { return r.ID }), []int{1, 2, 3}) || on[1].MachineOs != "linux" || on[1].Commit != "bbbb2222" {
		t.Errorf("RunOnSystem(linux) = %+v", on)
	}
	if got := do(t, d, db.RunOnSystem("darwin")); len(got) != 0 {
		t.Errorf("RunOnSystem(darwin) = %+v", got)
	}

	do(t, d, db.LineCountCreate(db.LineCountNew().ShaSet("aaaa1111").FixtureSet("examples/tiny").VoltLinesSet(1).HandLinesSet(50).GenLinesSet(10)))
	outgrown := do(t, d, db.LineCountOutgrown())
	var got []string
	for _, lc := range outgrown {
		got = append(got, lc.Sha+" "+lc.Fixture)
	}
	if want := []string{"aaaa1111 itest/blog", "bbbb2222 itest/blog", "cccc3333 itest/blog"}; !slices.Equal(got, want) {
		t.Errorf("LineCountOutgrown = %v, want %v: the tiny fixture has more hand lines, the voltmeter fixture is left out", got, want)
	}

	equal(t, "RunUncommitted", ids(do(t, d, db.RunUncommitted()), func(r db.Run) int { return r.ID }), []int{r2.ID})

	sha := func(cs []db.Commit) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Sha)
		}
		return out
	}
	if got := sha(do(t, d, db.CommitAdded(w.commits[0].At))); !slices.Equal(got, []string{"aaaa1111", "cccc3333"}) {
		t.Errorf("CommitAdded since the first commit = %v", got)
	}
	if got := sha(do(t, d, db.CommitAdded(w.commits[0].At.Add(time.Second)))); !slices.Equal(got, []string{"cccc3333"}) {
		t.Errorf("CommitAdded a second after the first commit = %v", got)
	}

	equal(t, "SampleQuick", ids(do(t, d, db.SampleQuick()), func(s db.Sample) int { return s.ID }), []int{1, 2})

	churn := do(t, d, db.MsGenerateChurn(1024, r1.ID))
	want := []db.Churn{{RunID: r1.ID, Tables: 2, Garbage: -1024, Moved: 3.5}, {RunID: r1.ID, Tables: 4, Garbage: -2048, Moved: 7}, {RunID: r1.ID, Tables: 8, Garbage: -4096, Moved: 14}}
	if !slices.Equal(churn, want) {
		t.Errorf("MsGenerateChurn = %+v, want %+v", churn, want)
	}

	span := do(t, d, db.CommitSpan())
	if !span.First.V.Equal(w.commits[0].At) || !span.Last.V.Equal(w.commits[2].At) || span.FirstAuthor.V != "ADA" || span.LastSubject.V != "Remove the project element" {
		t.Errorf("CommitSpan = %+v; min and max of text compare bytes, so ADA sorts before Ada", span)
	}

	names := do(t, d, db.DashboardNames(1))
	if len(names) != 2 || names[0].Name != "Empty" || len(names[0].Widgets) != 0 || names[1].Name != "Overview" || len(names[1].Widgets) != 2 || names[1].Widgets[0].Position != 1 {
		t.Errorf("DashboardNames(1) = %+v", names)
	}
}
