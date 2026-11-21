package assert

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// recoverFailure runs f and returns the failure it panicked with.
func recoverFailure(t *testing.T, f func()) (got failure) {
	t.Helper()
	defer func() {
		v := recover()
		var ok bool
		got, ok = v.(failure)
		if !ok {
			t.Fatalf("want an assertion failure, got %#v", v)
		}
	}()
	f()
	return got
}

func TestThatPasses(t *testing.T) {
	That(true, "must not trip")
}

func TestThatTrips(t *testing.T) {
	f := recoverFailure(t, func() { That(false, "balance must not be negative") })
	if f.msg != "balance must not be negative" {
		t.Fatalf("msg = %q", f.msg)
	}
}

func TestThatReportsCallerLine(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	f := recoverFailure(t, func() {
		That(false, "location") // line + 2
	})
	if filepath.Base(f.file) != "assert_test.go" || f.line != line+2 {
		t.Fatalf("location = %s:%d, want assert_test.go:%d", f.file, f.line, line+2)
	}
}

func TestFailFormatsAndReportsCallerLine(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	f := recoverFailure(t, func() {
		Fail("balance %d is negative", -5) // line + 2
	})
	if f.msg != "balance -5 is negative" {
		t.Fatalf("msg = %q", f.msg)
	}
	if f.line != line+2 {
		t.Fatalf("line = %d, want %d", f.line, line+2)
	}
}

func TestString(t *testing.T) {
	got := failure{msg: "x", file: "/a/b.go", line: 7}.String()
	if got != "assertion failed: x (/a/b.go:7)" {
		t.Fatalf("String() = %q", got)
	}
	if got := (failure{msg: "x"}).String(); got != "assertion failed: x" {
		t.Fatalf("String() without location = %q", got)
	}
}

func TestIsFailure(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"failure", failure{msg: "x"}, true},
		{"nil", nil, false},
		{"string", "boom", false},
		{"error", errors.New("boom"), false},
		{"pointer", &failure{msg: "x"}, false},
	}
	for _, c := range cases {
		if got := IsFailure(c.v); got != c.want {
			t.Errorf("%s: IsFailure = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCrashServesNormalRequests(t *testing.T) {
	h := Crash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestCrashPassesOtherPanicsOn(t *testing.T) {
	for _, want := range []any{"boom", http.ErrAbortHandler} {
		h := Crash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(want)
		}))
		func() {
			defer func() {
				if got := recover(); got != want {
					t.Fatalf("recovered %#v, want %#v", got, want)
				}
			}()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		}()
	}
}

// TestCrashExits runs a real server in a child process and checks that a
// failed assertion in a handler kills it with status 2.
func TestCrashExits(t *testing.T) {
	if os.Getenv("ASSERT_CRASH_CHILD") == "1" {
		srv := httptest.NewServer(Crash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			That(false, "invariant broken in handler")
		})))
		http.Get(srv.URL)
		select {} // only os.Exit ends this
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashExits$")
	cmd.Env = append(os.Environ(), "ASSERT_CRASH_CHILD=1")
	out, err := cmd.CombinedOutput()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("want exit status 2, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "assertion failed: invariant broken in handler") {
		t.Fatalf("missing message:\n%s", out)
	}
	if !strings.Contains(string(out), "assert_test.go:") {
		t.Fatalf("missing location:\n%s", out)
	}
}

// balances is filled at run time, so the compiler cannot prove anything about
// the values and remove the assertion.
var balances = func() []int {
	s := make([]int, 1024)
	for i := range s {
		s[i] = (i * 7919) % 1000
	}
	return s
}()

// sink keeps the loop's result alive so the compiler cannot delete the work.
var sink int

// These benchmarks use a b.N loop on purpose, so the gopls bloop hint does not
// apply. b.Loop keeps every value in its body alive, and at this scale that
// costs more than the assertion being measured: on Go 1.27, a passing That
// shows up as about 0.55 ns over the baseline with b.Loop and 0.09 ns with b.N.
// Before Go 1.26, b.Loop also stops That from being inlined. The bloop
// analyzer is off by default in go fix for this reason, see golang/go#74967.
//
// The difference between the two benchmarks is the cost of a passing That.

func BenchmarkBaseline(b *testing.B) {
	sum := 0
	for i := range b.N {
		sum += balances[i&1023]
	}
	sink = sum
}

func BenchmarkThatPasses(b *testing.B) {
	sum := 0
	for i := range b.N {
		v := balances[i&1023]
		That(v >= 0, "balance must not be negative")
		sum += v
	}
	sink = sum
}
