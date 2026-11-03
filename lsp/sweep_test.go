package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const aggregateSchema = "package db\n\n" +
	"table ms_load {\n\trun_id int [pk]\n\ttables int [pk]\n\talloc_mb float64\n}\n\n" +
	"table ms_check {\n\trun_id int [pk]\n\ttables int [pk]\n\talloc_mb float64\n}\n\n" +
	"group phases {\n\tms_load\n\tms_check\n}\n\n" +
	"pred at_size { tables in :sizes }\n\n" +
	"select per_size (tables, count() as n, avg(alloc_mb) as mean_alloc) for phases where at_size [group: (tables), order: (tables asc)]\n"

// TestAggregateSelectNavigates proves a select whose items are derived
// (aggregates under aliases, §selects-over-groups) is analyzed: its
// group target and predicate resolve to their declarations and its
// hover shows the row type with the derived fields. A derived item has
// no column, and reading one's settings once panicked the analysis, so
// nothing in the file resolved.
func TestAggregateSelectNavigates(t *testing.T) {
	root := voltProject(t, map[string]string{"go.mod": "module g\n", "db/schema.volt": aggregateSchema})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), aggregateSchema)
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	for _, c := range []struct {
		name string
		use  int
	}{{"phases", 1}, {"at_size", 1}} {
		loc := d.Definition(posOf(t, aggregateSchema, c.name, c.use))
		if loc == nil || loc.Range.Start != posOf(t, aggregateSchema, c.name, 0) {
			t.Errorf("%s in the select resolved to %+v, want its declaration", c.name, loc)
		}
	}
	h := d.Hover(posOf(t, aggregateSchema, "per_size", 0))
	if h == nil {
		t.Fatal("no hover for the aggregate select")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"N ", "MeanAlloc "} {
		if !strings.Contains(md, want) {
			t.Errorf("hover lacks the derived field %q:\n%s", want, md)
		}
	}
}

var (
	sweepPackage = regexp.MustCompile(`(?m)^package\s+(\w+)`)
	sweepTarget  = regexp.MustCompile(`\bfor\s+(\w+)`)
)

// TestSweepAnswersEveryIdentifier asks the language server for the
// definition, hover and references at every identifier of every Volt
// project in the repository, the valid conformance corpus included, so
// a construct the inline tests leave out cannot take the analysis down
// in an editor. In the projects that check clean, every select's target
// resolves to its declaration.
func TestSweepAnswersEveryIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("the sweep opens every Volt file of the repository")
	}
	clean := map[string]bool{}
	var files []string
	for _, dir := range []string{"../examples", "../itest/blog", "../nao/itest"} {
		found := voltFiles(t, dir)
		for _, f := range found {
			clean[f] = true
		}
		files = append(files, found...)
	}
	snippets, err := filepath.Glob("../lang/conformance/snippets/valid/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range snippets {
		info, err := os.Stat(s)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			files = append(files, voltFiles(t, s)...)
			continue
		}
		src, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		pkg := "db"
		if m := sweepPackage.FindSubmatch(src); m != nil {
			pkg = string(m[1])
		}
		root := voltProject(t, map[string]string{"go.mod": "module s\n", pkg + "/" + filepath.Base(s): string(src)})
		files = append(files, filepath.Join(root, pkg, filepath.Base(s)))
	}
	if len(files) < 100 {
		t.Fatalf("the sweep found only %d Volt files", len(files))
	}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		d := NewDocument("file://"+abs, text)
		diags := d.LSPDiagnostics()
		if clean[path] && len(diags) != 0 {
			t.Errorf("%s: unexpected diagnostics %v", path, diags)
		}
		for _, off := range identStarts(text) {
			pos := posAt(text, off)
			d.Definition(pos)
			d.Hover(pos)
			d.References(pos, true)
		}
		if !clean[path] {
			continue
		}
		for _, m := range sweepTarget.FindAllStringSubmatchIndex(text, -1) {
			line := text[strings.LastIndex(text[:m[0]], "\n")+1:]
			if !strings.HasPrefix(strings.TrimSpace(line), "select ") {
				continue
			}
			if d.Definition(posAt(text, m[2])) == nil {
				t.Errorf("%s:%d: the select target %s does not resolve", path, strings.Count(text[:m[0]], "\n")+1, text[m[2]:m[3]])
			}
		}
	}
}

func voltFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) && path != dir {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".volt") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func identStarts(text string) []int {
	var out []int
	prev := rune(0)
	for i, r := range text {
		isIdent := r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
		wasIdent := prev == '_' || unicode.IsLetter(prev) || unicode.IsDigit(prev)
		if isIdent && !wasIdent && !unicode.IsDigit(r) {
			out = append(out, i)
		}
		prev = r
	}
	return out
}

func posAt(text string, off int) protocol.Position {
	start := strings.LastIndex(text[:off], "\n") + 1
	col := 0
	for _, r := range text[start:off] {
		if r > 0xFFFF {
			col += 2
		} else {
			col++
		}
	}
	return protocol.Position{Line: protocol.UInteger(strings.Count(text[:off], "\n")), Character: protocol.UInteger(col)}
}
