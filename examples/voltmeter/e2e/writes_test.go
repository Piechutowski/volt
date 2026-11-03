package e2e

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

// TestWritesNegotiateFirst proves a writing controller answers a 406
// before its first effect (spec §formats, rules 1 and 4): an upload, a
// budget, an annotation, a sample batch and a dashboard asked for in a
// format the server does not speak store nothing.
func TestWritesNegotiateFirst(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	run := runUpload(t, c, sweep("feed0001", []int{2}, 1)).Run
	csv := []string{"Accept", "text/csv"}
	writes := []struct{ method, path, body string }{
		{"POST", app.PathAPIUpload(), `{"commit": "abcd1234", "machine": {"cpu": "another"}}`},
		{"POST", app.PathAPIBudgets(), `{"phase": "load", "tables": 2, "max_ms": 5}`},
		{"PATCH", app.PathAPIRunAnnotate(run.ID), `{"note": "never stored"}`},
		{"POST", app.PathAPISamplesRecord(run.ID), `{"samples": [{"route": "GET /ping", "status": 200, "ms": 1, "at": 1}]}`},
		{"POST", app.PathAPIThesisRecord(), `{"commits": [{"sha": "abcd1234", "counts": [{"fixture": "itest/blog"}]}]}`},
		{"POST", app.PathAPIBudgetsReset(), ``},
		{"POST", app.PathAPIMeDashboardAdd(), `{"name": "never"}`},
	}
	for _, w := range writes {
		r := e.raw(w.method, w.path, w.body, append(csv, "X-User", "1")...)
		if r.StatusCode != http.StatusNotAcceptable {
			t.Errorf("%s %s with Accept text/csv = %d %s", w.method, w.path, r.StatusCode, r.body)
		}
	}
	runs, err := c.APIRunListed(bg)
	if err != nil || len(runs) != 1 || runs[0].Note.Valid {
		t.Errorf("runs after the refused writes = %+v, %v", runs, err)
	}
	if ms, err := c.APIMachines(bg); err != nil || len(ms) != 1 {
		t.Errorf("machines after the refused upload = %+v, %v", ms, err)
	}
	if lat, err := c.APISampleLatency(bg, run.ID); err != nil || lat.N != 0 {
		t.Errorf("samples after the refused batch = %+v, %v", lat, err)
	}
	if n := e.count(t, db.BudgetCount(), db.DashboardCount(), db.CommitCount()); n != 0 {
		t.Errorf("%d budget, dashboard and commit rows after the refused writes", n)
	}
}

func (e *env) count(t *testing.T, counts ...rt.Lookup[int64]) int64 {
	t.Helper()
	var sum int64
	for _, c := range counts {
		n, err := run.Perform(bg, e.db, c)
		if err != nil {
			t.Fatal(err)
		}
		sum += n.(int64)
	}
	return sum
}

// TestFirstUploadsRace proves concurrent first uploads of one new
// machine and one new tag all store their run: the upload the unique
// index refuses reads the row the other added, so no sweep is lost to a
// 409 and each machine and tag is stored once.
func TestFirstUploadsRace(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	const rounds, together = 6, 8
	for round := range rounds {
		var wg sync.WaitGroup
		statuses := make([]int, together)
		for i := range together {
			wg.Go(func() {
				u := sweep(fmt.Sprintf("abc%05d", round*together+i), []int{2}, 1)
				u.Machine.CPU = fmt.Sprintf("Race CPU %d", round)
				u.Tag = fmt.Sprintf("race-%d", round)
				resp, err := c.APIUpload(bg, u)
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
				statuses[i] = resp.StatusCode
			})
		}
		wg.Wait()
		for i, s := range statuses {
			if s != http.StatusCreated {
				t.Errorf("round %d upload %d = %d, want 201", round, i, s)
			}
		}
	}
	fleet, err := c.APIMachineFleet(bg)
	if err != nil || len(fleet) != rounds {
		t.Fatalf("fleet = %+v, %v; want %d machines", fleet, err, rounds)
	}
	for _, m := range fleet {
		if m.RunCount != together {
			t.Errorf("machine %s holds %d runs, want %d", m.CPU, m.RunCount, together)
		}
	}
	if tags, err := c.APITags(bg); err != nil || len(tags) != rounds+1 {
		t.Errorf("tags = %+v, %v; want the baseline and %d", tags, err, rounds)
	}
}

// TestRefusedUploadLeavesNothing proves an upload refused after it added
// its machine and tag removes them again with its run, whether the
// transaction of measurements failed (a duplicated row) or the run
// itself was refused (a baseline no run has), while a machine and a tag
// that runs already use stay.
func TestRefusedUploadLeavesNothing(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	kept := sweep("cafe0001", []int{2}, 1)
	kept.Tag = "kept"
	runUpload(t, c, kept)

	broken := sweep("cafe0002", []int{2}, 1)
	broken.Machine.CPU = "Orphan CPU"
	broken.Tag = "orphan"
	broken.Phases = append(broken.Phases, broken.Phases[0])
	resp, err := c.APIUpload(bg, broken)
	replyRead(t, resp, err, http.StatusConflict, nil)

	unborn := sweep("cafe0003", []int{2}, 1)
	unborn.Machine.CPU = "Unborn CPU"
	unborn.Tag = "unborn"
	unborn.Baseline = 99
	resp, err = c.APIUpload(bg, unborn)
	if r := replyRead(t, resp, err, http.StatusUnprocessableEntity, nil); r.problem(t).Details[0].Check != "foreign_key" {
		t.Errorf("an upload naming a missing baseline = %s", r.body)
	}

	again := sweep("cafe0004", []int{2}, 1)
	again.Tag = "kept"
	again.Phases = append(again.Phases, again.Phases[0])
	resp, err = c.APIUpload(bg, again)
	replyRead(t, resp, err, http.StatusConflict, nil)

	ms, err := c.APIMachines(bg)
	if err != nil || len(ms) != 1 || ms[0].CPU != machine.CPU {
		t.Errorf("machines after the refused uploads = %+v, %v", ms, err)
	}
	tags, err := c.APITags(bg)
	var slugs []string
	for _, tg := range tags {
		slugs = append(slugs, tg.Slug)
	}
	if err != nil || !slices.Equal(slugs, []string{"baseline", "kept"}) {
		t.Errorf("tags after the refused uploads = %v, %v", slugs, err)
	}
	if n := e.count(t, db.RunCount()); n != 1 {
		t.Errorf("%d runs after the refused uploads, want 1", n)
	}
}

// TestMeasurementsValidated proves an upload and a sample batch holding
// a value no measurement produces are a 422 itemized per field, with
// nothing stored (spec §validators): sizes below one table, negative
// counts and costs, an empty route, a status outside HTTP's and a
// negative duration.
func TestMeasurementsValidated(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	run := runUpload(t, c, sweep("dead0001", []int{2}, 1)).Run

	bad := sweep("dead0002", []int{2}, 1)
	bad.Phases[0].Tables = -4
	bad.Phases[1].Allocs = -7
	bad.Keystrokes[0].MSPerOp = -1
	bad.Keystrokes[0].Tables = 0
	resp, err := c.APIUpload(bg, bad)
	r := replyRead(t, resp, err, http.StatusUnprocessableEntity, nil)
	var checks []string
	for _, d := range r.problem(t).Details {
		checks = append(checks, d.Check+"("+strings.Join(d.Columns, ",")+")")
	}
	want := []string{"TablesCounted(tables)", "AllocsCounted(allocs)", "TablesCounted(tables)", "KeystrokeSane(ms_per_op,mb_per_op,allocs_per_op)"}
	if !slices.Equal(checks, want) {
		t.Errorf("an upload of impossible numbers = %v, want %v", checks, want)
	}

	samples := app.SampleBatch{Samples: []app.SampleTimed{
		{Route: "GET /ping", Status: 200, MS: 0.2, At: 1},
		{Route: "", Status: -5, MS: -3, At: 2},
	}}
	resp, err = c.APISamplesRecord(bg, run.ID, samples)
	r = replyRead(t, resp, err, http.StatusUnprocessableEntity, nil)
	checks = checks[:0]
	for _, d := range r.problem(t).Details {
		checks = append(checks, d.Check+"("+strings.Join(d.Columns, ",")+")")
	}
	if !slices.Equal(checks, []string{"required(route)", "SampleSane(status,ms)"}) {
		t.Errorf("a batch with an impossible sample = %v", checks)
	}
	if lat, err := c.APISampleLatency(bg, run.ID); err != nil || lat.N != 0 {
		t.Errorf("latency after the refused batch = %+v, %v; the valid sample is not stored either", lat, err)
	}
	if n := e.count(t, db.RunCount()); n != 1 {
		t.Errorf("%d runs after the refused upload", n)
	}
}
