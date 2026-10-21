package lang_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	root := scheduleFixture(t)
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

	_, work = sessionRound(t, &s, root, nil, s.Stats())
	expect("no change", work, 0, 0)

	d2 := filepath.Join(root, "d2", "schema.volt")
	src, err := os.ReadFile(d2)
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(src), "  title string\n", "", 1)
	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed}, s.Stats())
	expect("data edit in the editor", work, 1, 2)

	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed}, s.Stats())
	expect("same overlay", work, 0, 0)

	r3 := filepath.Join(root, "r3", "routes.volt")
	src, err = os.ReadFile(r3)
	if err != nil {
		t.Fatal(err)
	}
	routed := strings.Replace(string(src), "Scope /api {\n", "Scope /api {\n\tget /extra Extra.Show\n", 1)
	_, work = sessionRound(t, &s, root, map[string]string{d2: fixed, r3: routed}, s.Stats())
	expect("route edit in the editor", work, 1, 1)

	if err := os.WriteFile(d2, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, work = sessionRound(t, &s, root, map[string]string{r3: routed}, s.Stats())
	expect("save of the edited file", work, 0, 0)

	if err := os.WriteFile(filepath.Join(root, "r1", "mw.go"), []byte("package r1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, work = sessionRound(t, &s, root, map[string]string{r3: routed}, s.Stats())
	expect("Go file added", work, 0, 1)
}

func TestSessionReadsWhatIsOnDisk(t *testing.T) {
	root := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module disk\n")
	write("db/schema.volt", "package db\n\nTable users {\n\tid int [pk, increment]\n\temail string\n\ttitle string\n\ttitle string\n\n\tvalidators {\n\t\tEmailValid(email)\n\t}\n}\n")
	write("db/checks.go", "package db\n\nfunc EmailValid(email string) error { return nil }\n")
	sameStamp := func(name, from, to string) {
		t.Helper()
		if len(from) != len(to) {
			t.Fatalf("%q and %q differ in length", from, to)
		}
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), from) {
			t.Fatalf("%s does not contain %q", name, from)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(src), from, to, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			t.Fatalf("precondition: %s's stamp moved", name)
		}
	}
	var s lang.Session
	expect := func(name string, got lang.SessionStats, parsed, checked int) {
		t.Helper()
		if got.FilesParsed != parsed || got.PackagesChecked != checked {
			t.Errorf("%s: parsed %d files and checked %d packages; want %d and %d", name, got.FilesParsed, got.PackagesChecked, parsed, checked)
		}
	}
	_, work := sessionRound(t, &s, root, nil, lang.SessionStats{})
	expect("first load", work, 1, 1)
	_, work = sessionRound(t, &s, root, nil, s.Stats())
	expect("no change", work, 0, 0)

	sameStamp("db/schema.volt", "\ttitle string\n\ttitle string\n", "\ttitle string\n\ttitel string\n")
	_, work = sessionRound(t, &s, root, nil, s.Stats())
	expect("same-length edit of the .volt file", work, 1, 1)

	sameStamp("db/checks.go", "email string", "email []byte")
	_, work = sessionRound(t, &s, root, nil, s.Stats())
	expect("same-length edit of the Go file", work, 0, 1)
}

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
	edited := strings.Replace(string(src), "Table m_t0001s {\n", "Table m_t0001s {\n  extra string\n", 1)
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
		if decls, answered := after.DeclsVetted-before.DeclsVetted, after.DeclsVetReused-before.DeclsVetReused; checked == 0 && (decls != 0 || answered != 0) {
			t.Errorf("%s: no package vetted, yet %d declarations judged and %d answered", name, decls, answered)
		} else if checked > 0 && decls == 0 {
			t.Errorf("%s: %d packages vetted, yet no declaration judged", name, checked)
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

func TestSessionBindsUnresolvedSelects(t *testing.T) {
	root := t.TempDir()
	write := func(name, src string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("go.mod", "module unresolved\n")
	schema := "package db\n\nTable authors {\n\tid int [pk, increment]\n\trank int32\n}\n\nTable tags {\n\tid int [pk, increment]\n\tname string\n}\n\nSelect ranked for authors where rank >= :min\n"
	schemaPath := write("db/schema.volt", schema)
	routes := "package app\n\nimport (\n\tdb\n)\n\nScope /api {\n\tget /ranked db.AuthorRanked [name: RankedRows]\n\tget /ranked/:min db.AuthorRanked [name: RankedFrom]\n\tdataset db.ranked\n}\n"
	routesPath := write("app/routes.volt", routes)

	var s lang.Session
	round := func(name string, overlay map[string]string, want ...string) {
		t.Helper()
		pr, _ := sessionRound(t, &s, root, overlay, s.Stats())
		if sels := pr.Packages["db"].Selects; len(sels) != 1 || !sels[0].Unresolved || !slices.Equal(sels[0].UnknownCols, []string{"rank"}) {
			t.Fatalf("%s: want the one select kept and unresolved over rank, got %+v", name, sels)
		}
		fresh, err := lang.LoadOverlay(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		var errs []string
		for _, d := range lang.Check(fresh) {
			if d.Severity == diag.Error {
				errs = append(errs, d.String())
			}
		}
		want = append([]string{`unknown type "int32"`}, want...)
		ok := len(errs) == len(want)
		for i := 0; ok && i < len(want); i++ {
			ok = strings.Contains(errs[i], want[i])
		}
		if !ok {
			t.Fatalf("%s: want errors containing %q, got:\n%s", name, want, strings.Join(errs, "\n"))
		}
	}
	round("first load", nil)
	routed := strings.Replace(routes, "Scope /api {\n", "Scope /api {\n\tget /tags db.TagList\n", 1)
	round("route edit", map[string]string{routesPath: routed})
	reused := s.Stats().SelectsReused
	noted := strings.Replace(schema, "name string\n", "name string [note: 'shown']\n", 1)
	round("data edit", map[string]string{schemaPath: noted, routesPath: routed})
	if s.Stats().SelectsReused == reused {
		t.Errorf("the data edit checked the unresolved select afresh; want it answered by the memo")
	}
	clash := strings.Replace(routed, "\tdataset db.ranked\n", "\tdataset db.ranked\n\tget /ranked Home.Index\n", 1)
	round("conflicting route", map[string]string{schemaPath: noted, routesPath: clash}, "conflicts with the route at")
}

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
	type work struct{ parsed, tables, models, checks, selects, routes, vetted int }
	round := func(name, text string, want work) {
		t.Helper()
		before := s.Stats()
		overlay := map[string]string{path: text}
		pr, err := s.Load(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		got := diagsRender(s.Check(pr))
		gotVet := diagsRender(s.Vet(pr))
		fresh, err := lang.LoadOverlay(root, overlay)
		if err != nil {
			t.Fatal(err)
		}
		if want := diagsRender(lang.Check(fresh)); got != want {
			t.Fatalf("%s: session differs from fresh\n--- session\n%s--- fresh\n%s", name, got, want)
		}
		if want := diagsRender(lang.Vet(fresh)); gotVet != want {
			t.Fatalf("%s: session vet differs from fresh\n--- session\n%s--- fresh\n%s", name, gotVet, want)
		}
		after := s.Stats()
		did := work{after.DeclsParsed - before.DeclsParsed, after.TablesChecked - before.TablesChecked,
			after.ModelsBuilt - before.ModelsBuilt, after.ChecksLowered - before.ChecksLowered,
			after.SelectsChecked - before.SelectsChecked, after.RoutesLowered - before.RoutesLowered,
			after.DeclsVetted - before.DeclsVetted}
		if did != want {
			t.Errorf("%s: did %+v, want %+v (reused: %d declarations, %d tables, %d models, %d checks, %d selects, %d vetted)", name, did, want,
				after.DeclsReused-before.DeclsReused, after.TablesReused-before.TablesReused,
				after.ModelsReused-before.ModelsReused, after.ChecksReused-before.ChecksReused,
				after.SelectsReused-before.SelectsReused, after.DeclsVetReused-before.DeclsVetReused)
		}
	}
	all := work{whole.Parsed, 12, 12, 12, 13, 17, whole.Parsed}
	round("first", text, all)
	one := work{1, 1, 1, 1, 2, 2, 2}
	round("edit one table", strings.Replace(text, "c002 string\n", "c002 string [note: 'edited']\n", 1), one)
	round("break it", strings.Replace(text, "c002 string\n", "c002 string [null\n", 1), one)
	round("fix it", text, one)
	cur := strings.Replace(text, "get /events        volt.Events", "get /stream        volt.Events", 1)
	round("edit a route", cur, work{1, 0, 0, 0, 0, 17, 1})
	cur = strings.Replace(cur, "created_at time.Time", "created_at time.Time [note: 'stamped']", 1)
	round("edit the partial", cur, work{1, 12, 12, 12, 13, 13, 13})
	cur = strings.Replace(cur, "retired [note: 'no longer written']", "retired [note: 'gone']", 1)
	round("edit the enum's note", cur, work{1, 0, 0, 0, 0, 0, 1})
	cur = strings.Replace(cur, "TablePartial stamped", "Enum kind {\n\tplain\n}\n\nTablePartial stamped", 1)
	round("add an enum", cur, work{1, 12, 12, 12, 13, 13, 13})
	cur = strings.Replace(cur, "Pred fresh { c001 >= :since }", "Pred fresh { c001 > :since }", 1)
	round("edit the pred", cur, work{1, 0, 0, 0, 13, 1, 1})
	if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package main\n\nfunc Extra() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	round("add a Go file", cur, work{0, 0, 0, 12, 0, 0, 0})
}

func TestSessionConcurrentCallersAgreeWithFreshAnalysis(t *testing.T) {
	root := scheduleFixture(t)
	d2 := filepath.Join(root, "d2", "schema.volt")
	src, err := os.ReadFile(d2)
	if err != nil {
		t.Fatal(err)
	}
	base := string(src)
	var s lang.Session
	const callers, rounds = 4, 6
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for c := 0; c < callers; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				text := base
				if (c+r)%2 == 1 {
					text = strings.Replace(base, "  title string\n", "", 1)
				}
				text = strings.Replace(text, "Table tags {", fmt.Sprintf("Table tags_%d_%d {", c, r), 1)
				overlay := map[string]string{d2: text}
				pr, err := s.Load(root, overlay)
				if err != nil {
					errs[c] = err
					return
				}
				got := diagsRender(s.Check(pr))
				fresh, err := lang.LoadOverlay(root, overlay)
				if err != nil {
					errs[c] = err
					return
				}
				if want := diagsRender(lang.Check(fresh)); got != want {
					errs[c] = fmt.Errorf("caller %d round %d: session differs from fresh\n--- session\n%s--- fresh\n%s", c, r, got, want)
					return
				}
			}
		}(c)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
