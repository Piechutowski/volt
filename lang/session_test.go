package lang_test

// A Session's Load and Check must be indistinguishable from a fresh
// LoadOverlay and Check, edit after edit (D79), while doing only the
// work an edit implies: one parse per changed file, one check per
// package that could see it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

func diagsRender(ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// sessionRound runs the session and a fresh analysis on the same
// overlay and compares the diagnostics; it returns the session's
// project and the work counters of this round.
func sessionRound(t *testing.T, s *lang.Session, root string, overlay map[string]string, before lang.SessionStats) (*lang.Project, lang.SessionStats) {
	t.Helper()
	pr, err := s.Load(root, overlay)
	if err != nil {
		t.Fatal(err)
	}
	got := diagsRender(s.Check(pr))
	fresh, err := lang.LoadOverlay(root, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if want := diagsRender(lang.Check(fresh)); got != want {
		t.Fatalf("session diagnostics differ from a fresh analysis:\n--- session\n%s--- fresh\n%s", got, want)
	}
	after := s.Stats()
	return pr, lang.SessionStats{
		FilesParsed: after.FilesParsed - before.FilesParsed, FilesReused: after.FilesReused - before.FilesReused,
		PackagesChecked: after.PackagesChecked - before.PackagesChecked, PackagesReused: after.PackagesReused - before.PackagesReused,
	}
}

func TestSessionMatchesFreshAnalysis(t *testing.T) {
	root := scheduleFixture(t) // 6 data packages, 6 routing packages, errors in all
	var s lang.Session
	expect := func(name string, got lang.SessionStats, parsed, checked int) {
		t.Helper()
		if got.FilesParsed != parsed || got.PackagesChecked != checked {
			t.Errorf("%s: parsed %d files and checked %d packages; want %d and %d (reused %d files, %d packages)",
				name, got.FilesParsed, got.PackagesChecked, parsed, checked, got.FilesReused, got.PackagesReused)
		}
	}
	_, work := sessionRound(t, &s, root, nil, lang.SessionStats{})
	expect("first load", work, 12, 12)

	// Nothing changed: nothing is parsed or checked.
	_, work = sessionRound(t, &s, root, nil, s.Stats())
	expect("no change", work, 0, 0)

	// An overlay edit in a data package: that file parses again; the
	// package and the one routing package importing it check again.
	d2 := filepath.Join(root, "d2", "schema.volt")
	src, err := os.ReadFile(d2)
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(src), "  title text\n", "", 1)
	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed}, s.Stats())
	expect("data edit in the editor", work, 1, 2)

	// The same overlay again: the parse and the memo both hit.
	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed}, s.Stats())
	expect("same overlay", work, 0, 0)

	// An overlay edit in a routing package touches that package alone.
	r3 := filepath.Join(root, "r3", "routes.volt")
	src, err = os.ReadFile(r3)
	if err != nil {
		t.Fatal(err)
	}
	routed := strings.Replace(string(src), "Scope /api {\n", "Scope /api {\n\tget /extra Extra.Show\n", 1)
	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed, r3: routed}, s.Stats())
	expect("route edit in the editor", work, 1, 1)

	// A save: the overlay goes, the disk changes. The file is read
	// again (its stat moved), and being the same text as the overlay,
	// its parse is reused; nothing checks again.
	if err := os.WriteFile(d2, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, work = sessionRound(t, &s, root, map[string]string{r3: routed}, s.Stats())
	expect("save of the edited file", work, 0, 0)

	// A Go file appearing beside a routing package's routes changes
	// its Go-function stamp: that package checks again.
	if err := os.WriteFile(filepath.Join(root, "r1", "mw.go"), []byte("package r1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, work = sessionRound(t, &s, root, map[string]string{r3: routed}, s.Stats())
	expect("Go file added", work, 0, 1)
}

// TestSessionGeneratesIdentically proves the restored package results
// carry everything the generators read: the bytes match a fresh run.
func TestSessionGeneratesIdentically(t *testing.T) {
	root := perfFixture(t, 6, 5, true)
	generate := func(pr *lang.Project) map[string]string {
		out := map[string]string{}
		for path, pkg := range pr.Packages {
			if pkg.HasSchema() {
				files, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true})
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range files {
					out[path+"/"+f.Name] = string(f.Code)
				}
			}
			if pkg.HasRouting() {
				files, err := router.Generate(pkg, router.Options{Source: "package " + path})
				if err != nil {
					t.Fatal(err)
				}
				for name, code := range files {
					out[path+"/"+name] = string(code)
				}
			}
		}
		return out
	}
	var s lang.Session
	schema := filepath.Join(root, "db", "schema.volt")
	src, err := os.ReadFile(schema)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(src), "Table m_t0001s {\n", "Table m_t0001s {\n  extra text\n", 1)
	for i, overlay := range []map[string]string{nil, {schema: edited}, {schema: edited}, nil} {
		pr, _ := sessionRound(t, &s, root, overlay, s.Stats())
		fresh, err := lang.LoadOverlay(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		if diags := lang.Check(fresh); diag.HasErrors(diags) {
			t.Fatal(diags)
		}
		got, want := generate(pr), generate(fresh)
		if len(got) != len(want) {
			t.Fatalf("round %d: %d files from the session, %d fresh", i, len(got), len(want))
		}
		for name, code := range want {
			if got[name] != code {
				t.Fatalf("round %d: %s differs between the session and a fresh run", i, name)
			}
		}
	}
	if st := s.Stats(); st.PackagesReused == 0 {
		t.Fatalf("the memo never hit: %+v", st)
	}
}
