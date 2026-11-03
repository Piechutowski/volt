// Package e2e drives Voltmeter end to end: the production handler of
// package server on a migrated SQLite file behind an httptest server,
// through the generated client in GOB and through raw HTTP in JSON.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/app/client"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/server"
	"github.com/Piechutowski/volt/nao/run"
	_ "github.com/mattn/go-sqlite3"
)

var exercised = struct {
	sync.Mutex
	routes map[string]bool
}{routes: map[string]bool{}}

// TestMain fails the run when the whole suite ran and some route of the
// generated table was never served: every route is exercised end to end.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && flag.Lookup("test.run").Value.String() == "" && flag.Lookup("test.skip").Value.String() == "" {
		var missed []string
		exercised.Lock()
		for _, r := range app.Table {
			key := r.Pattern
			if r.Method != "" {
				key = r.Method + " " + r.Pattern
			}
			if !exercised.routes[key] {
				missed = append(missed, key)
			}
		}
		exercised.Unlock()
		if len(missed) > 0 {
			sort.Strings(missed)
			fmt.Fprintf(os.Stderr, "routes no test served: %s\n", strings.Join(missed, ", "))
			code = 1
		}
	}
	os.Exit(code)
}

type env struct {
	t      *testing.T
	srv    *httptest.Server
	db     run.DB
	broker *volt.Broker
}

func start(t *testing.T) *env {
	t.Helper()
	d, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "voltmeter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := run.MigrateUp(context.Background(), d, db.Migrations); err != nil {
		t.Fatal(err)
	}
	b := volt.NewBroker()
	h := server.Handler(d, b)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		if r.Pattern != "" {
			exercised.Lock()
			exercised.routes[r.Pattern] = true
			exercised.Unlock()
		}
	}))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv, db: d, broker: b}
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

func (e *env) gob(user int) *client.Client {
	c := client.New(e.srv.URL)
	c.Format = volt.FormatGOB
	c.HTTP = e.httpClient(user)
	return c
}

func (e *env) json(user int) *client.Client {
	c := client.New(e.srv.URL)
	c.HTTP = e.httpClient(user)
	return c
}

func (e *env) httpClient(user int) *http.Client {
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if user > 0 {
		hc.Transport = member{id: user, base: e.srv.Client().Transport}
	}
	return hc
}

type reply struct {
	*http.Response
	body []byte
}

func (r reply) problem(t *testing.T) volt.Problem {
	t.Helper()
	var p volt.Problem
	f, ok := volt.FormatOf(r.Header.Get("Content-Type"))
	if !ok {
		t.Fatalf("%d with %q is no Problem: %s", r.StatusCode, r.Header.Get("Content-Type"), r.body)
	}
	if err := volt.DecodeBody(f, bytes.NewReader(r.body), &p); err != nil {
		t.Fatalf("decoding the Problem: %v (%s)", err, r.body)
	}
	return p
}

func (e *env) raw(method, path, body string, header ...string) reply {
	e.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := e.httpClient(0).Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return reply{resp, b}
}

func (e *env) rawJSON(method, path, body string, out any, header ...string) reply {
	e.t.Helper()
	r := e.raw(method, path, body, header...)
	if out != nil && r.StatusCode < 300 {
		if err := json.Unmarshal(r.body, out); err != nil {
			e.t.Fatalf("%s %s: %v (%s)", method, path, err, r.body)
		}
	}
	return r
}

func read(t *testing.T, resp *http.Response, err error, want int, out any) reply {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		t.Fatal(rerr)
	}
	r := reply{resp, b}
	if resp.StatusCode != want {
		t.Fatalf("%s %s = %d, want %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, b)
	}
	if out != nil {
		f, ok := volt.FormatOf(resp.Header.Get("Content-Type"))
		if !ok {
			t.Fatalf("reply type %q", resp.Header.Get("Content-Type"))
		}
		if err := volt.DecodeBody(f, bytes.NewReader(b), out); err != nil {
			t.Fatalf("decoding %T: %v", out, err)
		}
	}
	return r
}

var machine = app.UploadMachine{CPU: "Test CPU 3000", Cores: 8, MemoryGB: 16, OS: "linux 6.1", Arch: "amd64", Go: "go1.27.0"}

func sweep(commit string, sizes []int, scale float64) app.Upload {
	u := app.Upload{Commit: commit, Machine: machine}
	for i, ph := range db.Phases {
		for _, n := range sizes {
			p := app.UploadPhase{Phase: ph, Tables: n, WallMS: scale * float64((i+1)*n), AllocMB: float64(n) / 2, HeapMB: 1, Allocs: 100 * n, SourceKB: 1.5 * float64(n)}
			if ph == db.PhaseGenerate {
				p.OutputMB = float64(n) / 10
			}
			u.Phases = append(u.Phases, p)
		}
	}
	for _, n := range sizes {
		u.Keystrokes = append(u.Keystrokes, app.UploadKeystroke{Tables: n, MSPerOp: 0.1 * float64(n), MBPerOp: 0.01, AllocsPerOp: 40})
	}
	return u
}

func upload(t *testing.T, c *client.Client, u app.Upload) app.UploadResult {
	t.Helper()
	resp, err := c.APIUpload(context.Background(), u)
	var out app.UploadResult
	read(t, resp, err, http.StatusCreated, &out)
	return out
}
