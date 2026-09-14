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
	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
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

// TestSessionVetMemo proves Session.Vet answers from the memo exactly
// when Session.Check did: the warnings match a fresh Vet round by
// round, and a round that re-checked n packages re-vets n (D81).
func TestSessionVetMemo(t *testing.T) {
	root := scheduleFixture(t)
	var s lang.Session
	round := func(name string, overlay map[string]string, checked int) {
		t.Helper()
		before := s.Stats()
		pr, err := s.Load(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		s.Check(pr)
		got := diagsRender(s.Vet(pr))
		fresh, err := lang.LoadOverlay(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		lang.Check(fresh)
		if want := diagsRender(lang.Vet(fresh)); got != want {
			t.Fatalf("%s: session vet differs from a fresh vet:\n--- session\n%s--- fresh\n%s", name, got, want)
		}
		after := s.Stats()
		if vetted, ch := after.PackagesVetted-before.PackagesVetted, after.PackagesChecked-before.PackagesChecked; vetted != checked || ch != checked {
			t.Errorf("%s: vetted %d, checked %d packages; want %d (vet reused %d)", name, vetted, ch, checked, after.PackagesVetReused-before.PackagesVetReused)
		}
	}
	round("first", nil, 12)
	round("no change", nil, 0)
	d2 := filepath.Join(root, "d2", "schema.volt")
	src, err := os.ReadFile(d2)
	if err != nil {
		t.Fatal(err)
	}
	round("edit", map[string]string{d2: string(src) + "\n// touched\n"}, 2)
	round("same edit", map[string]string{d2: string(src) + "\n// touched\n"}, 0)
}

// TestSessionReparsesOneDeclaration proves an edit inside one
// declaration of a one-file project re-parses that declaration alone
// (D83): the session's diagnostics still match a fresh analysis, and
// the work counters say one declaration parsed, the rest reused.
func TestSessionReparsesOneDeclaration(t *testing.T) {
	root := t.TempDir()
	if err := corpus.Write(root, corpus.Spec{Tables: 12, Columns: 6, Single: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	_, _, _, whole := parser.ParseFileReuse(path, text, nil)
	var s lang.Session
	// work is what one round did, in declarations parsed and in tables
	// whose schema check, model and lowered checks were run afresh.
	type work struct{ parsed, tables, models, checks, selects int }
	round := func(name, text string, want work) {
		t.Helper()
		before := s.Stats()
		overlay := map[string]string{path: text}
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
			t.Fatalf("%s: session differs from fresh\n--- session\n%s--- fresh\n%s", name, got, want)
		}
		after := s.Stats()
		did := work{after.DeclsParsed - before.DeclsParsed, after.TablesChecked - before.TablesChecked,
			after.ModelsBuilt - before.ModelsBuilt, after.ChecksLowered - before.ChecksLowered,
			after.SelectsChecked - before.SelectsChecked}
		if did != want {
			t.Errorf("%s: did %+v, want %+v (reused: %d declarations, %d tables, %d models, %d checks, %d selects)", name, did, want,
				after.DeclsReused-before.DeclsReused, after.TablesReused-before.TablesReused,
				after.ModelsReused-before.ModelsReused, after.ChecksReused-before.ChecksReused,
				after.SelectsReused-before.SelectsReused)
		}
	}
	// 12 tables, each with a projected select, and the group select
	// over all of them: 13 selects.
	all := work{whole.Parsed, 12, 12, 12, 13}
	round("first", text, all)
	// One table: its own select and the group select see a new member.
	one := work{1, 1, 1, 1, 2}
	round("edit one table", strings.Replace(text, "c002 text [not null]", "c002 text [not null, note: 'edited']", 1), one)
	round("break it", strings.Replace(text, "c002 text [not null]", "c002 text [not null", 1), one)
	round("fix it", text, one)
	// Edits accumulate from here, so each round changes one chunk.
	cur := strings.Replace(text, "get /events        volt.Events", "get /stream        volt.Events", 1)
	round("edit a route", cur, work{1, 0, 0, 0, 0}) // touches no table
	// The partial every table injects: every table's columns change,
	// so every table is checked again, its model rebuilt, its checks
	// lowered again, every select re-checked.
	cur = strings.Replace(cur, "created_at timestamp", "created_at timestamp [note: 'stamped']", 1)
	round("edit the partial", cur, work{1, 12, 12, 12, 13})
	// An enum's note: no table and no model depends on it.
	cur = strings.Replace(cur, "retired [note: 'no longer written']", "retired [note: 'gone']", 1)
	round("edit the enum's note", cur, work{1, 0, 0, 0, 0})
	// A new enum: the enum set is an input of every table's check (the
	// required rule asks whether a column type is an enum) and of every
	// model, so every table is checked again and every model rebuilt;
	// the lowered checks and the selects follow their models.
	cur = strings.Replace(cur, "TablePartial stamped", "Enum kind {\n\tplain\n}\n\nTablePartial stamped", 1)
	round("add an enum", cur, work{1, 12, 12, 12, 13})
	// The predicate every select names: every select is checked again.
	cur = strings.Replace(cur, "Pred fresh { c001 >= :since }", "Pred fresh { c001 > :since }", 1)
	round("edit the pred", cur, work{1, 0, 0, 0, 13})
	// A Go file of the package: the tables and models stand, the Go
	// reference checks are lowered again.
	if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package main\n\nfunc Extra() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	round("add a Go file", cur, work{0, 0, 0, 12, 0})
}
