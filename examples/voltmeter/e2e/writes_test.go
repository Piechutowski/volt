package e2e

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
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

// TestRefusedUploadLeavesNoRun proves an upload refused after its run
// was stored deletes the run again, whether the transaction of
// measurements failed (a duplicated row) or the run itself was refused
// (a baseline no run has), while the machine and tag it found or added
// stay: a concurrent upload may already have stored its run on them
// (TestRefusedFirstUploadsKeepTheirNeighbours).
func TestRefusedUploadLeavesNoRun(t *testing.T) {
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

	ms, err := c.APIMachines(bg)
	var cpus []string
	for _, m := range ms {
		cpus = append(cpus, m.CPU)
	}
	if err != nil || !slices.Equal(cpus, []string{machine.CPU, "Orphan CPU", "Unborn CPU"}) {
		t.Errorf("machines after the refused uploads = %v, %v", cpus, err)
	}
	tags, err := c.APITags(bg)
	var slugs []string
	for _, tg := range tags {
		slugs = append(slugs, tg.Slug)
	}
	if err != nil || !slices.Equal(slugs, []string{"baseline", "kept", "orphan", "unborn"}) {
		t.Errorf("tags after the refused uploads = %v, %v", slugs, err)
	}
	if n := e.count(t, db.RunCount()); n != 1 {
		t.Errorf("%d runs after the refused uploads, want 1", n)
	}
	if n := e.count(t, db.MsLoadCount(), db.MsCheckCount(), db.MsVetCount(), db.MsGenerateCount(), db.KeystrokeCount()); n != 5 {
		t.Errorf("%d measurement rows after the refused uploads, want the kept run's 5", n)
	}
}

// TestRefusedFirstUploadsKeepTheirNeighbours proves a refused first
// upload of a new machine and tag never takes a concurrent valid
// upload's run with it: each pair races an upload refused for a
// duplicated row against a valid one on the same new machine and tag,
// and every valid upload answers 201 and its run stays with its tag.
func TestRefusedFirstUploadsKeepTheirNeighbours(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	const pairs = 60
	stored := make([]int, pairs)
	var wg sync.WaitGroup
	for i := range pairs {
		wg.Go(func() {
			good := sweep(fmt.Sprintf("beef%04d", i), []int{2}, 1)
			good.Machine.CPU = fmt.Sprintf("Pair CPU %d", i)
			good.Tag = fmt.Sprintf("pair-%d", i)
			bad := good
			bad.Commit = fmt.Sprintf("bad0%04d", i)
			bad.Phases = append(slices.Clip(good.Phases), good.Phases[0])
			var pair sync.WaitGroup
			pair.Go(func() {
				resp, err := c.APIUpload(bg, bad)
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusConflict {
					t.Errorf("pair %d: the duplicated upload = %d, want 409", i, resp.StatusCode)
				}
			})
			pair.Go(func() {
				resp, err := c.APIUpload(bg, good)
				if err != nil {
					t.Error(err)
					return
				}
				var out wire.UploadResult
				if resp.StatusCode != http.StatusCreated {
					b, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					t.Errorf("pair %d: the valid upload = %d %s, want 201", i, resp.StatusCode, b)
					return
				}
				err = volt.DecodeBody(volt.FormatGOB, resp.Body, &out)
				resp.Body.Close()
				if err != nil {
					t.Error(err)
					return
				}
				stored[i] = out.Run.ID
			})
			pair.Wait()
		})
	}
	wg.Wait()
	runs, err := c.APIRunListed(bg)
	if err != nil {
		t.Fatal(err)
	}
	tagged := map[int]string{}
	for _, r := range runs {
		tagged[r.ID] = r.TagName.V
	}
	for i, id := range stored {
		if id == 0 {
			continue
		}
		if want := fmt.Sprintf("pair-%d", i); tagged[id] != want {
			t.Errorf("pair %d: run %d answered 201 and now reads tag %q, want %q (gone: %t)", i, id, tagged[id], want, tagged[id] == "")
		}
	}
	if len(runs) != pairs {
		t.Errorf("%d runs stored, want %d", len(runs), pairs)
	}
}

// TestMeasurementsValidated proves an upload and a sample batch holding
// a value no measurement produces are a 422 itemized per field, with
// nothing stored (spec §validators): sizes below one table, negative
// counts, costs and sizes, a machine without an identity or a
// processor, an upload measuring no phase, an empty route, a status
// outside HTTP's and a negative duration.
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

	empty := app.Upload{Commit: "dead0003", Machine: app.UploadMachine{Cores: -4, MemoryGB: -1}}
	resp, err = c.APIUpload(bg, empty)
	r = replyRead(t, resp, err, http.StatusUnprocessableEntity, nil)
	checks = checks[:0]
	for _, d := range r.problem(t).Details {
		checks = append(checks, d.Check+"("+strings.Join(d.Columns, ",")+")")
	}
	if want := []string{"MachineNamed(cpu,os,arch,go)", "MachineSized(cores,memory_gb)", "required(phases)"}; !slices.Equal(checks, want) {
		t.Errorf("an upload of no machine and no phase = %v, want %v", checks, want)
	}
	negative := sweep("dead0004", []int{2}, 1)
	negative.Phases[3].OutputMB = -7
	resp, err = c.APIUpload(bg, negative)
	if r := replyRead(t, resp, err, http.StatusUnprocessableEntity, nil); r.problem(t).Details[0].Check != "MegabytesMeasured" {
		t.Errorf("an upload of a negative output size = %s", r.body)
	}
	if ms, err := c.APIMachines(bg); err != nil || len(ms) != 1 {
		t.Errorf("machines after the refused uploads = %+v, %v", ms, err)
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

// TestThesisJudgedAndRaced proves a thesis upload stores no row its
// validators refuse (spec §validators), and that collectors uploading
// the same history at once all succeed: the one the unique key refuses
// reads again and finds the rows stored, so each row is written once
// and no upload answers 409.
func TestThesisJudgedAndRaced(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	malformed := app.ThesisUpload{Commits: []app.ThesisCommit{{SHA: "not a sha / ../", Counts: []app.ThesisCount{{Fixture: "../../etc", VoltLines: -5, HandLines: -1, GenLines: -9}}}}}
	resp, err := c.APIThesisRecord(bg, malformed)
	r := replyRead(t, resp, err, http.StatusUnprocessableEntity, nil)
	var checks []string
	for _, d := range r.problem(t).Details {
		checks = append(checks, d.Check)
	}
	if want := []string{"required", "ShaShape", "CommitDated", "ShaShape", "FixtureShape", "LinesCounted"}; !slices.Equal(checks, want) {
		t.Errorf("a malformed thesis upload = %v, want %v", checks, want)
	}
	if n := e.count(t, db.CommitCount(), db.LineCountCount()); n != 0 {
		t.Errorf("%d thesis rows after the refused upload", n)
	}

	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var body app.ThesisUpload
	for i := range 300 {
		body.Commits = append(body.Commits, app.ThesisCommit{
			SHA: fmt.Sprintf("%040x", i+1), Subject: "Commit", Author: "Ada", At: at.Add(time.Duration(i) * time.Minute),
			Counts: []app.ThesisCount{{Fixture: "itest/blog", VoltLines: i, HandLines: 2 * i, GenLines: 3 * i}},
		})
	}
	const together = 4
	var wg sync.WaitGroup
	written := make([]int, together)
	for i := range together {
		wg.Go(func() {
			resp, err := c.APIThesisRecord(bg, body)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("upload %d = %d %s, want 201 or 200", i, resp.StatusCode, b)
				return
			}
			var out wire.Stored
			if err := volt.DecodeBody(volt.FormatGOB, resp.Body, &out); err != nil {
				t.Error(err)
				return
			}
			if out.Written+out.Skipped != 600 {
				t.Errorf("upload %d = %+v, want 600 rows written or skipped", i, out)
			}
			written[i] = out.Written
		})
	}
	wg.Wait()
	sum := 0
	for _, w := range written {
		sum += w
	}
	if sum != 600 {
		t.Errorf("the uploads wrote %v, want 600 rows in all", written)
	}
	if n := e.count(t, db.CommitCount(), db.LineCountCount()); n != 600 {
		t.Errorf("%d thesis rows stored, want 600", n)
	}
}
