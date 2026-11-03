package e2e

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/collector"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/internal/perfsweep"
)

// TestCollectStoresWhatTheSweepMeasured runs voltmeter collect in
// process at tiny sizes against the served handler: the stored phase
// rows are the sweep's own numbers, a budget the sweep misses is
// breached in the upload's transaction, a second collection finds its
// machine again by the unique index, the thesis counts agree with an
// independent count of the same trees, and every read route has its
// samples.
func TestCollectStoresWhatTheSweepMeasured(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH:", err)
	}
	e := envStart(t)
	bg := context.Background()
	repo, err := collector.RepoFind(".")
	if err != nil {
		t.Fatal(err)
	}
	var budget db.Budget
	if r := e.rawJSON("POST", app.PathAPIBudgets(), `{"phase": "load", "tables": 2, "max_ms": 0.001}`, &budget); r.StatusCode != http.StatusCreated {
		t.Fatalf("budget: %d %s", r.StatusCode, r.body)
	}
	c := e.gob(1)
	resp, err := c.APIMeDashboardAdd(bg, db.DashboardNew().NameSet("sweep"))
	var board db.Dashboard
	replyRead(t, resp, err, http.StatusCreated, &board)

	cfg := collector.Config{
		Server: e.srv.URL, User: 1, Repo: repo,
		Sizes: []int{2, 4}, Columns: 3, Reps: 1, Profile: 0, Keystroke: false,
		Rev: "HEAD", Commits: 2, Fixtures: []string{"itest/blog", "examples/voltmeter"},
		Samples: 2, Tag: "tiny", Note: "collected in process",
	}
	got, err := collector.Collect(bg, cfg, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	run := got.Stored.Run
	if len(got.Rows) != len(perfsweep.Phases)*len(cfg.Sizes) {
		t.Fatalf("the sweep measured %d rows", len(got.Rows))
	}
	if run.ID == 0 || run.Tag.V != "tiny" || run.Note.V != "collected in process" || string(run.Commit) != got.Upload.Commit {
		t.Errorf("stored run %+v", run)
	}
	headSHA, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(run.Commit) != strings.TrimSpace(string(headSHA)) {
		t.Errorf("the run records commit %q, want the full sha %s the thesis commits are stored under", run.Commit, headSHA)
	}

	load, err := c.APIPhaseMsLoadRows(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	check, err := c.APIPhaseMsCheckRows(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	vet, err := c.APIPhaseMsVetRows(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	generate, err := c.APIPhaseMsGenerateRows(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored := wire.PhaseRowsOf(load, check, vet, generate)
	if len(stored) != len(got.Rows) {
		t.Fatalf("%d rows stored, %d measured", len(stored), len(got.Rows))
	}
	for _, s := range stored {
		r := perfsweep.At(got.Rows, s.Tables, string(s.Phase))
		if r == nil {
			t.Errorf("stored %s at %d was never measured", s.Phase, s.Tables)
			continue
		}
		want := wire.PhaseRow{Phase: s.Phase, Tables: r.Tables, WallMS: units.Millis(r.WallMS), AllocMB: r.AllocMB, HeapMB: r.HeapMB, Allocs: int(r.Allocs), OutputMB: r.OutputMB, SourceKB: r.SourceKB}
		if s != want || r.WallMS <= 0 || r.Allocs == 0 || r.SourceKB <= 0 {
			t.Errorf("stored %+v, the sweep measured %+v", s, *r)
		}
		if (s.Phase == db.PhaseGenerate) != (s.OutputMB > 0) {
			t.Errorf("%s at %d emitted %v MB", s.Phase, s.Tables, s.OutputMB)
		}
	}

	if len(got.Stored.Breaches) != 1 || got.Stored.Breaches[0].BudgetID != budget.ID || float64(got.Stored.Breaches[0].WallMs) != perfsweep.At(got.Rows, 2, "load").WallMS {
		t.Errorf("breaches %+v", got.Stored.Breaches)
	}
	resp, err = c.APIBudget(bg, budget.ID)
	var detail wire.BudgetDetail
	replyRead(t, resp, err, http.StatusOK, &detail)
	if detail.Budget.State != db.HealthBreached || len(detail.Breaches) != 1 {
		t.Errorf("budget after the upload %+v", detail)
	}

	perRoute, err := c.APISamplePerRoute(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	timed := map[string]int{}
	for _, pr := range perRoute {
		timed[pr.Route] = pr.N
	}
	for _, r := range app.Table {
		method := r.Method
		switch method {
		case "":
			method = "GET"
		case "GET", "HEAD", "OPTIONS":
		default:
			continue
		}
		key := method + " " + r.Pattern
		if _, ok := collector.ReadCallsExcluded[key]; ok {
			continue
		}
		if timed[key] != cfg.Samples {
			t.Errorf("%s has %d samples, want %d", key, timed[key], cfg.Samples)
		}
		delete(timed, key)
	}
	if len(timed) > 0 {
		t.Errorf("samples of no read route: %v", timed)
	}
	failures, err := c.APISampleFailures(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("read routes answered %d failures, the first %+v", len(failures), failures[0])
	}
	latency, err := c.APISampleLatency(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latency.N != len(got.Samples) || latency.N != len(got.Calls)*cfg.Samples || !latency.Best.Valid || latency.Best.V <= 0 {
		t.Errorf("latency %+v of %d samples", latency, len(got.Samples))
	}

	if len(got.Thesis.Commits) == 0 {
		t.Fatal("no commit was counted")
	}
	rows := 0
	for _, cm := range got.Thesis.Commits {
		rows++
		for _, fx := range cfg.Fixtures {
			want, present := linesOracle(t, repo, cm.SHA, fx)
			var have *app.ThesisCount
			for i := range cm.Counts {
				if cm.Counts[i].Fixture == fx {
					have = &cm.Counts[i]
				}
			}
			switch {
			case !present && have != nil:
				t.Errorf("%s has no %s, yet counted %+v", cm.SHA, fx, *have)
			case present && have == nil:
				t.Errorf("%s: %s was not counted", cm.SHA, fx)
			case present && *have != want:
				t.Errorf("%s: %s counted %+v, the tree holds %+v", cm.SHA, fx, *have, want)
			}
			if present {
				rows++
			}
		}
	}
	if got.ThesisStored.Written != rows || got.ThesisStored.Skipped != 0 {
		t.Errorf("thesis stored %+v, want %d written", got.ThesisStored, rows)
	}
	head := got.Thesis.Commits[0]
	thesis, err := c.APILineCountThesis(bg, "itest/blog")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, th := range thesis {
		if th.Sha == head.SHA {
			found = true
			if th.CommitSubject != head.Subject || !th.CommitAt.Equal(head.At) || th.VoltLines != head.Counts[0].VoltLines || th.HandLines != head.Counts[0].HandLines || th.GenLines != head.Counts[0].GenLines {
				t.Errorf("thesis row %+v, counted %+v", th, head)
			}
		}
	}
	if !found {
		t.Errorf("the thesis select lacks %s: %+v", head.SHA, thesis)
	}

	cfg.Samples, cfg.Tag, cfg.Note, cfg.Baseline = 0, "", "", run.ID
	again, err := collector.Collect(bg, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Stored.Run.MachineID != run.MachineID || again.Stored.Run.ID <= run.ID || again.Stored.Run.BaselineID.V != run.ID {
		t.Errorf("the second run %+v, the first %+v", again.Stored.Run, run)
	}
	machines, err := c.APIMachines(bg)
	if err != nil {
		t.Fatal(err)
	}
	fleet, err := c.APIMachineFleet(bg)
	if err != nil {
		t.Fatal(err)
	}
	m := got.Machine
	if len(machines) != 1 || machines[0].CPU != m.CPU || machines[0].Cores != m.Cores || machines[0].MemoryGb != m.MemoryGB || machines[0].Os != m.OS || machines[0].Go != m.Go {
		t.Errorf("machines %+v, measured on %+v", machines, m)
	}
	if len(fleet) != 1 || fleet[0].RunCount != 2 {
		t.Errorf("fleet %+v", fleet)
	}
	if again.ThesisStored.Written != 0 || again.ThesisStored.Skipped != rows {
		t.Errorf("the second thesis upload stored %+v, want %d skipped", again.ThesisStored, rows)
	}
	successors, err := c.APIRunSuccessors(bg, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(successors) != 1 || successors[0].FollowersID.V != again.Stored.Run.ID {
		t.Errorf("successors %+v", successors)
	}
}

func linesOracle(t *testing.T, repo, sha, fixture string) (app.ThesisCount, bool) {
	t.Helper()
	ls, err := exec.Command("git", "-C", repo, "ls-tree", "--name-only", sha, "--", fixture).Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.TrimSpace(ls)) == 0 {
		return app.ThesisCount{}, false
	}
	archive, err := exec.Command("git", "-C", repo, "archive", "--format=tar", sha, "--", fixture).Output()
	if err != nil {
		t.Fatal(err)
	}
	c := app.ThesisCount{Fixture: fixture}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		lines := 0
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(nil, 1<<26)
		for sc.Scan() {
			lines++
		}
		switch {
		case strings.HasSuffix(h.Name, ".volt"):
			c.VoltLines += lines
		case strings.HasSuffix(h.Name, ".go"):
			f, err := parser.ParseFile(token.NewFileSet(), h.Name, data, parser.PackageClauseOnly|parser.ParseComments)
			if err != nil {
				t.Fatalf("%s at %s: %v", h.Name, sha, err)
			}
			if ast.IsGenerated(f) {
				c.GenLines += lines
			} else {
				c.HandLines += lines
			}
		}
	}
	return c, true
}

// TestCollectAsksTheServerFirst proves a collection pointed at a server
// that is down, or at a member it does not know, fails before the sweep
// spends its minutes, with nothing measured.
func TestCollectAsksTheServerFirst(t *testing.T) {
	e := envStart(t)
	repo, err := collector.RepoFind(".")
	if err != nil {
		t.Fatal(err)
	}
	cfg := collector.Config{Server: e.srv.URL, User: 99, Repo: repo, Sizes: []int{2}, Columns: 3, Reps: 1}
	got, err := collector.Collect(bg, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "nothing was measured") || len(got.Rows) != 0 {
		t.Errorf("collect as an unknown member = %v with %d rows", err, len(got.Rows))
	}
	cfg.Server, cfg.User = "http://127.0.0.1:1", 1
	if got, err := collector.Collect(bg, cfg, nil); err == nil || len(got.Rows) != 0 {
		t.Errorf("collect against no server = %v with %d rows", err, len(got.Rows))
	}
}
