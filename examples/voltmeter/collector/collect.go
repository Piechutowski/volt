package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/app/client"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/internal/perfsweep"
)

// Config says what one collection measures and where it stores it.
type Config struct {
	Server    string        // the Voltmeter base URL
	User      int           // the member sent as X-User on every call
	Repo      string        // the Volt checkout: its commit, its history, its lsp package
	Out       string        // where the sweep writes sweep.json and profiles; "" a temporary directory removed afterwards
	Sizes     []int         // table counts, ascending
	Columns   int           // columns per table
	Reps      int           // unprofiled timing runs per phase
	Profile   time.Duration // CPU profile per phase and size, kept under Out; zero skips it
	Keystroke bool          // run the language server's keystroke benchmark
	Count     int           // keystroke benchmark repetitions per size
	Rev       string        // the revision the thesis history starts from
	Commits   int           // commits touching the fixtures to count, newest first
	Fixtures  []string      // fixture directories, relative to Repo
	Samples   int           // timed calls per read route
	Tag       string
	Note      string
	Baseline  int
}

// FixturesDefault are the fixtures a collection counts the lines of
// unless told otherwise: the blog Volt tests itself with, and Voltmeter.
var FixturesDefault = []string{"itest/blog", "examples/voltmeter"}

// Collected is everything one collection measured and what the server
// answered when it was stored.
type Collected struct {
	Machine      perfsweep.Machine
	Rows         []perfsweep.Row
	Keystrokes   []perfsweep.Keystroke
	Upload       app.Upload
	Stored       wire.UploadResult
	Thesis       app.ThesisUpload
	ThesisStored wire.Stored
	Samples      []app.SampleTimed
	Calls        []Call
}

type member struct {
	id   int
	base http.RoundTripper
}

func (m member) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-User", strconv.Itoa(m.id))
	return m.base.RoundTrip(r)
}

// ClientNew is the generated client speaking GOB to base as member
// user: the client carries no default header, so X-User rides on the
// transport. It follows no redirect, so a timed call is the route's
// own.
func ClientNew(base string, user int) *client.Client {
	c := client.New(base)
	c.Format = volt.FormatGOB
	c.HTTP = &http.Client{
		Transport:     member{id: user, base: http.DefaultTransport},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c
}

// RepoFind is the nearest directory at or above dir holding go.work,
// the root of the Volt checkout.
func RepoFind(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.work")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no go.work at or above %s", dir)
		}
	}
}

// Collect runs one collection: the sweep and the keystroke benchmark
// on this machine, the upload, the thesis counts of the history, then
// cfg.Samples timed calls of every read route, stored as the run's
// samples. log reports progress and may be nil.
func Collect(ctx context.Context, cfg Config, log func(format string, args ...any)) (Collected, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	var out Collected
	out.Machine = perfsweep.Info(cfg.Repo)
	log("%s, %d cores, %s, %s, commit %s", out.Machine.CPU, out.Machine.Cores, out.Machine.OS, out.Machine.Go, out.Machine.Commit)
	dir := cfg.Out
	if dir == "" {
		tmp, err := os.MkdirTemp("", "voltmeter-sweep-")
		if err != nil {
			return out, err
		}
		defer os.RemoveAll(tmp)
		dir = tmp
	}
	var err error
	spec := perfsweep.Spec{Sizes: cfg.Sizes, Columns: cfg.Columns, Reps: cfg.Reps, Profile: cfg.Profile}
	if out.Rows, err = perfsweep.Run(dir, spec, log); err != nil {
		return out, err
	}
	if cfg.Keystroke {
		if out.Keystrokes, err = perfsweep.Keystrokes(cfg.Repo, cfg.Sizes, cfg.Columns, max(cfg.Count, 1), log); err != nil {
			return out, err
		}
	}
	if out.Upload, err = UploadOf(out.Machine, out.Rows, out.Keystrokes, cfg.Tag, cfg.Note, cfg.Baseline); err != nil {
		return out, err
	}
	c := ClientNew(cfg.Server, cfg.User)
	if err := expect(c.APIUpload(ctx, out.Upload))(http.StatusCreated, &out.Stored); err != nil {
		return out, fmt.Errorf("upload: %w", err)
	}
	log("stored run %d on machine %d with %d breach(es)", out.Stored.Run.ID, out.Stored.Run.MachineID, len(out.Stored.Breaches))

	if cfg.Commits > 0 {
		if out.Thesis, err = thesisCounted(ctx, cfg); err != nil {
			return out, err
		}
		if len(out.Thesis.Commits) > 0 {
			if err := expect(c.APIThesisRecord(ctx, out.Thesis))(0, &out.ThesisStored); err != nil {
				return out, fmt.Errorf("thesis: %w", err)
			}
		}
		log("thesis: %d commit(s) counted, %d row(s) written, %d already stored", len(out.Thesis.Commits), out.ThesisStored.Written, out.ThesisStored.Skipped)
	}

	if cfg.Samples > 0 {
		t, err := targetsFind(ctx, c, cfg, out.Stored.Run)
		if err != nil {
			return out, err
		}
		out.Calls = ReadCalls(t)
		for i := 0; i < cfg.Samples; i++ {
			for _, call := range out.Calls {
				s, err := timed(ctx, c, call)
				if err != nil {
					return out, err
				}
				out.Samples = append(out.Samples, s)
			}
		}
		for _, batch := range SamplesBatched(out.Samples, 1000) {
			var stored wire.Stored
			if err := expect(c.APISamplesRecord(ctx, out.Stored.Run.ID, batch))(http.StatusCreated, &stored); err != nil {
				return out, fmt.Errorf("samples: %w", err)
			}
		}
		log("timed %d call(s) of %d read route(s)", len(out.Samples), len(out.Calls))
	}
	return out, nil
}

func expect(resp *http.Response, err error) func(status int, out any) error {
	return func(status int, out any) error {
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if (status != 0 && resp.StatusCode != status) || resp.StatusCode >= 300 {
			return fmt.Errorf("%s %s answered %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, bytes.TrimSpace(body))
		}
		f, ok := volt.FormatOf(resp.Header.Get("Content-Type"))
		if !ok {
			return fmt.Errorf("%s %s answered %q", resp.Request.Method, resp.Request.URL.Path, resp.Header.Get("Content-Type"))
		}
		return volt.DecodeBody(f, bytes.NewReader(body), out)
	}
}

func targetsFind(ctx context.Context, c *client.Client, cfg Config, run db.Run) (Targets, error) {
	t := Targets{Run: run.ID, Machine: run.MachineID, Tag: "baseline", Tables: cfg.Sizes[0]}
	if run.Tag.Valid {
		t.Tag = run.Tag.V
	}
	if len(cfg.Fixtures) > 0 {
		t.Fixture = cfg.Fixtures[0]
	}
	var boards []db.DashboardWithWidgets
	if err := expect(c.APIMeDashboards(ctx))(http.StatusOK, &boards); err != nil {
		return t, err
	}
	if len(boards) > 0 {
		t.Board = boards[0].ID
	}
	var budgets []db.Budget
	if err := expect(c.APIBudgets(ctx))(http.StatusOK, &budgets); err != nil {
		return t, err
	}
	if len(budgets) > 0 {
		t.Budget = budgets[0].ID
	}
	return t, nil
}

func timed(ctx context.Context, c *client.Client, call Call) (app.SampleTimed, error) {
	start := time.Now()
	resp, err := c.Raw(ctx, call.Method, call.Path, nil)
	if err != nil {
		return app.SampleTimed{}, fmt.Errorf("%s: %w", call.Route, err)
	}
	_, err = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	took := time.Since(start)
	if err != nil {
		return app.SampleTimed{}, fmt.Errorf("%s: %w", call.Route, err)
	}
	return SampleOf(call, resp.StatusCode, start, took), nil
}

func thesisCounted(ctx context.Context, cfg Config) (app.ThesisUpload, error) {
	args := []string{"log", "-n", strconv.Itoa(cfg.Commits), "--format=" + LogFormat, cfg.Rev, "--"}
	logOut, err := git(ctx, cfg.Repo, nil, append(args, cfg.Fixtures...)...)
	if err != nil {
		return app.ThesisUpload{}, err
	}
	commits, err := CommitsParse(logOut)
	if err != nil {
		return app.ThesisUpload{}, err
	}
	files := make([][]File, len(commits))
	for i, cm := range commits {
		tree, err := git(ctx, cfg.Repo, nil, append([]string{"ls-tree", "-r", "-z", "--full-tree", cm.SHA, "--"}, cfg.Fixtures...)...)
		if err != nil {
			return app.ThesisUpload{}, err
		}
		blobs, err := TreeParse(tree)
		if err != nil {
			return app.ThesisUpload{}, err
		}
		var ask bytes.Buffer
		for _, b := range blobs {
			ask.WriteString(b.Object + "\n")
		}
		batch, err := git(ctx, cfg.Repo, &ask, "cat-file", "--batch")
		if err != nil {
			return app.ThesisUpload{}, err
		}
		objs, err := BatchParse(batch)
		if err != nil {
			return app.ThesisUpload{}, err
		}
		if len(objs) != len(blobs) {
			return app.ThesisUpload{}, fmt.Errorf("git cat-file answered %d of %d objects of %s", len(objs), len(blobs), cm.SHA)
		}
		for j, b := range blobs {
			files[i] = append(files[i], File{Path: b.Path, Data: objs[j]})
		}
	}
	return ThesisOf(commits, cfg.Fixtures, files)
}

func git(ctx context.Context, repo string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git %v: %w: %s", args, err, bytes.TrimSpace(stderr.Bytes()))
		}
		return nil, err
	}
	return out, nil
}
