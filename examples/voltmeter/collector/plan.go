// Package collector is voltmeter collect: the sweep, the keystroke and
// the thesis line counts stored through the generated client, then the
// latency of the read routes. Collect and RepoFind are the edge that
// runs the sweep, git, the file system and HTTP; every other function is
// pure (spec §values-and-effects).
package collector

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/internal/perfsweep"
)

// UploadOf is the upload body of one sweep: the commit and the machine
// perfsweep.Info reported, every row of the sweep and every keystroke.
// A commit Info could not read is an error, since a run names its
// commit.
func UploadOf(m perfsweep.Machine, rows []perfsweep.Row, ks []perfsweep.Keystroke, tag, note string, baseline int) (app.Upload, error) {
	sha, dirty := strings.CutSuffix(m.Commit, "+dirty")
	commit, err := units.CommitParse(sha)
	if err != nil {
		return app.Upload{}, fmt.Errorf("the repository's commit %q: %w", m.Commit, err)
	}
	u := app.Upload{
		Commit:   string(commit),
		Dirty:    dirty,
		Tag:      tag,
		Note:     note,
		Baseline: baseline,
		Machine:  app.UploadMachine{CPU: m.CPU, Cores: m.Cores, MemoryGB: m.MemoryGB, OS: m.OS, Arch: m.Arch, Go: m.Go},
	}
	for _, r := range rows {
		u.Phases = append(u.Phases, app.UploadPhase{
			Phase:    db.Phase(r.Phase),
			Tables:   r.Tables,
			WallMS:   r.WallMS,
			AllocMB:  r.AllocMB,
			HeapMB:   r.HeapMB,
			Allocs:   int(r.Allocs),
			OutputMB: r.OutputMB,
			SourceKB: r.SourceKB,
		})
	}
	for _, k := range ks {
		u.Keystrokes = append(u.Keystrokes, app.UploadKeystroke{Tables: k.Tables, MSPerOp: k.MSPerOp, MBPerOp: k.MBPerOp, AllocsPerOp: k.AllocsPerOp})
	}
	return u, nil
}

// LogFormat is the git log format CommitsParse reads: hash, author,
// author date and subject, split by the unit separator, each commit
// ended by the record separator.
const LogFormat = "%H%x1f%an%x1f%aI%x1f%s%x1e"

// CommitsParse reads the output of git log --format=LogFormat into the
// commits of a thesis upload, without counts.
func CommitsParse(out []byte) ([]app.ThesisCommit, error) {
	var cs []app.ThesisCommit
	for _, rec := range bytes.Split(out, []byte{0x1e}) {
		rec = bytes.TrimSpace(rec)
		if len(rec) == 0 {
			continue
		}
		f := strings.Split(string(rec), "\x1f")
		if len(f) != 4 {
			return nil, fmt.Errorf("git log record %q has %d fields, want 4", rec, len(f))
		}
		at, err := time.Parse(time.RFC3339, f[2])
		if err != nil {
			return nil, fmt.Errorf("git log date of %s: %w", f[0], err)
		}
		cs = append(cs, app.ThesisCommit{SHA: f[0], Author: f[1], At: at.UTC(), Subject: f[3]})
	}
	return cs, nil
}

// Blob is one file of a commit's tree: its object id and its path from
// the repository root.
type Blob struct {
	Object string
	Path   string
}

// TreeParse reads the output of git ls-tree -r -z into the blobs it
// lists; submodules and other non-blob entries are left out.
func TreeParse(out []byte) ([]Blob, error) {
	var bs []Blob
	for _, ent := range bytes.Split(out, []byte{0}) {
		if len(ent) == 0 {
			continue
		}
		meta, path, ok := bytes.Cut(ent, []byte{'\t'})
		f := strings.Fields(string(meta))
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("git ls-tree entry %q", ent)
		}
		if f[1] == "blob" {
			bs = append(bs, Blob{Object: f[2], Path: string(path)})
		}
	}
	return bs, nil
}

// BatchParse reads the output of git cat-file --batch into the contents
// of the objects in the order they were asked for.
func BatchParse(out []byte) ([][]byte, error) {
	var objs [][]byte
	for len(out) > 0 {
		head, rest, ok := bytes.Cut(out, []byte{'\n'})
		f := strings.Fields(string(head))
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("git cat-file header %q", head)
		}
		n, err := strconv.Atoi(f[2])
		if err != nil || n > len(rest) || (n < len(rest) && rest[n] != '\n') {
			return nil, fmt.Errorf("git cat-file object %s of size %s is cut short", f[0], f[2])
		}
		objs = append(objs, rest[:n])
		out = rest[min(n+1, len(rest)):]
	}
	return objs, nil
}

var generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

func generated(src []byte) bool {
	for _, line := range bytes.Split(src, []byte{'\n'}) {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if bytes.HasPrefix(line, []byte("package ")) {
			return false
		}
		if generatedMarker.Match(line) {
			return true
		}
	}
	return false
}

// File is one file of a fixture and its content.
type File struct {
	Path string
	Data []byte
}

// LinesCount is the thesis numbers of one fixture's files: lines of
// .volt files, lines of Go files carrying the Code generated marker
// before the package clause (the rule of go/ast.IsGenerated), and lines
// of every other Go file, tests included, since a person writes those.
// Other files are not counted.
func LinesCount(fixture string, files []File) app.ThesisCount {
	c := app.ThesisCount{Fixture: fixture}
	for _, f := range files {
		n := linesOf(f.Data)
		switch {
		case strings.HasSuffix(f.Path, ".volt"):
			c.VoltLines += n
		case !strings.HasSuffix(f.Path, ".go"):
		case generated(f.Data):
			c.GenLines += n
		default:
			c.HandLines += n
		}
	}
	return c
}

func linesOf(b []byte) int {
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}

// FixtureOf is the fixture a path lies in, the first of fixtures that
// is the path's directory or one above it.
func FixtureOf(fixtures []string, path string) (string, bool) {
	for _, fx := range fixtures {
		if strings.HasPrefix(path, strings.TrimSuffix(fx, "/")+"/") {
			return fx, true
		}
	}
	return "", false
}

// ThesisOf is the thesis upload of the counted commits: each commit
// with one count per fixture that has files at that commit, files
// holding each commit's files in the order of commits.
func ThesisOf(commits []app.ThesisCommit, fixtures []string, files [][]File) (app.ThesisUpload, error) {
	if len(files) != len(commits) {
		return app.ThesisUpload{}, errors.New("one file list per commit")
	}
	var up app.ThesisUpload
	for i, c := range commits {
		byFixture := map[string][]File{}
		for _, f := range files[i] {
			if fx, ok := FixtureOf(fixtures, f.Path); ok {
				byFixture[fx] = append(byFixture[fx], f)
			}
		}
		c.Counts = nil
		for _, fx := range fixtures {
			if fs := byFixture[fx]; len(fs) > 0 {
				c.Counts = append(c.Counts, LinesCount(fx, fs))
			}
		}
		up.Commits = append(up.Commits, c)
	}
	return up, nil
}

// Targets are the rows a timed call reads: the stored run and its
// machine, a tag, a fixture, a table count, one of the member's boards
// and a budget. A zero id names no row and its call answers 404.
type Targets struct {
	Run     int
	Machine int
	Tag     string
	Fixture string
	Tables  int
	Board   int
	Budget  int
}

// Call is one read route to time: the method, the route as the router
// table spells it ("GET /api/runs/{id}"), and the path to ask.
type Call struct {
	Method string
	Route  string
	Path   string
}

// ReadCallsExcluded are the safe routes ReadCalls leaves out, each with
// why: the event stream answers until the client hangs up, so it has
// no latency to time.
var ReadCallsExcluded = map[string]string{
	"GET /events": "a stream that answers until the client hangs up",
}

// ReadCalls is one call per read route of the router table, every GET,
// HEAD and OPTIONS route and any route asked with GET, aimed at t, but
// the routes in ReadCallsExcluded.
func ReadCalls(t Targets) []Call {
	q := func(k string, v any) volt.URLOption { return volt.Query(k, fmt.Sprint(v)) }
	get := func(route, path string) Call { return Call{Method: "GET", Route: "GET " + route, Path: path} }
	return []Call{
		get("/{$}", app.PathRoot()),
		get("/ping", app.PathPing()),
		get("/assets/{path...}", app.PathAsset("voltmeter.html")),
		get("/api/thesis", app.PathAPILineCountThesis(q("fixture", t.Fixture))),
		get("/api/commits", app.PathAPICommitMatching(q("pattern", "%"))),
		get("/api/probe", app.PathAPIProbe(q("steps", 1000))),
		get("/api/runs", app.PathAPIRuns(q("limit", 20))),
		get("/api/runs/{id}", app.PathAPIRun(t.Run)),
		{Method: "OPTIONS", Route: "OPTIONS /api/runs", Path: app.PathAPIRunsAllowed()},
		get("/api/runs/listed", app.PathAPIRunListed()),
		get("/api/runs/untagged", app.PathAPIRunUntagged()),
		get("/api/runs/{id}/successors", app.PathAPIRunSuccessors(t.Run)),
		get("/api/runs/{run}/latency", app.PathAPISampleLatency(t.Run)),
		get("/api/runs/{run}/routes", app.PathAPISamplePerRoute(t.Run)),
		get("/api/runs/{run}/failures", app.PathAPISampleFailures(t.Run)),
		get("/api/machines", app.PathAPIMachines()),
		get("/api/machines/{id}", app.PathAPIHost(t.Machine)),
		get("/api/fleet", app.PathAPIMachineFleet()),
		get("/api/tags", app.PathAPITags()),
		get("/api/tags/{slug}", app.PathAPITag(t.Tag)),
		get("/api/users", app.PathAPIUsers()),
		get("/api/budgets", app.PathAPIBudgets()),
		get("/api/budgets/{id}", app.PathAPIBudget(t.Budget)),
		get("/api/phases/load", app.PathAPIPhaseMsLoadRows(q("run", t.Run))),
		get("/api/phases/check", app.PathAPIPhaseMsCheckRows(q("run", t.Run))),
		get("/api/phases/vet", app.PathAPIPhaseMsVetRows(q("run", t.Run))),
		get("/api/phases/generate", app.PathAPIPhaseMsGenerateRows(q("run", t.Run))),
		get("/api/trend/load", app.PathAPITrendMsLoadTrend(q("tables", t.Tables))),
		get("/api/trend/check", app.PathAPITrendMsCheckTrend(q("tables", t.Tables))),
		get("/api/trend/vet", app.PathAPITrendMsVetTrend(q("tables", t.Tables))),
		get("/api/trend/generate", app.PathAPITrendMsGenerateTrend(q("tables", t.Tables))),
		get("/api/density/load", app.PathAPIDensityMsLoadDensity(q("run", t.Run))),
		get("/api/density/check", app.PathAPIDensityMsCheckDensity(q("run", t.Run))),
		get("/api/density/vet", app.PathAPIDensityMsVetDensity(q("run", t.Run))),
		get("/api/me", app.PathAPIMeWho()),
		get("/api/me/dashboards", app.PathAPIMeDashboards()),
		get("/api/me/dashboards/{board}", app.PathAPIMeDashboard(t.Board)),
		{Method: "HEAD", Route: "HEAD /reports/runs/{id}", Path: app.PathReportProbe(t.Run)},
		get("/reports/runs/{id}", app.PathReportShow(t.Run)),
		get("/reports/runs/{id}/csv", app.PathReportExport(t.Run)),
	}
}

// SampleOf is one timed call as the samples route stores it: the route,
// the status it answered, and how long it took from start, in
// milliseconds.
func SampleOf(c Call, status int, start time.Time, took time.Duration) app.SampleTimed {
	return app.SampleTimed{Route: c.Route, Status: status, MS: float64(took.Nanoseconds()) / 1e6, At: int(start.UnixMilli())}
}

// SamplesBatched splits samples into batches of at most size, so no
// batch nears the server's body limit.
func SamplesBatched(samples []app.SampleTimed, size int) []app.SampleBatch {
	var bs []app.SampleBatch
	for len(samples) > 0 {
		n := min(size, len(samples))
		bs = append(bs, app.SampleBatch{Samples: samples[:n:n]})
		samples = samples[n:]
	}
	return bs
}
