package vet_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/vet"
)

var wantRE = regexp.MustCompile(`//WANT ([a-z,]+)`)

func TestAnalyzers(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no testdata files")
	}
	for _, file := range files {
		file := file
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(src), "\n")
			header := strings.TrimPrefix(strings.TrimSpace(lines[0]), "// analyzers:")
			var analyzers []vet.Analyzer
			for _, name := range strings.Split(header, ",") {
				a := vet.ByName(strings.TrimSpace(name))
				if a == nil {
					t.Fatalf("unknown analyzer %q in header", name)
				}
				analyzers = append(analyzers, a)
			}

			want := map[int][]string{}
			for i, ln := range lines {
				if m := wantRE.FindStringSubmatch(ln); m != nil {
					for _, n := range strings.Split(m[1], ",") {
						want[i+1] = append(want[i+1], n)
					}
				}
			}

			f, diags := parser.ParseFile(file, string(src))
			if diag.HasErrors(diags) {
				t.Fatalf("test input must check clean; parse errors: %v", diags)
			}
			info, semDiags := check.File(f)
			if diag.HasErrors(semDiags) {
				t.Fatalf("test input must check clean; check errors: %v", semDiags)
			}

			for _, w := range vet.Run(f, info, analyzers...) {
				name := strings.TrimPrefix(w.Code, "vet/")
				matched := false
				rest := want[int(w.Pos.Line())][:0]
				for _, n := range want[int(w.Pos.Line())] {
					if !matched && n == name {
						matched = true
						continue
					}
					rest = append(rest, n)
				}
				want[int(w.Pos.Line())] = rest
				if !matched {
					t.Errorf("unexpected warning at line %d: %s", w.Pos.Line(), w)
				}
			}
			for line, names := range want {
				for _, n := range names {
					t.Errorf("line %d: expected %s warning, got none", line, n)
				}
			}
		})
	}
}

func TestCleanFile(t *testing.T) {
	src := `
Project shop { database_type: 'PostgreSQL' }
Enum status { active
  retired }
Table users as U {
  id int [pk, increment]
  status status [not null]
}
Table orders {
  id int [pk]
  user_id int [not null, ref: > U.id]
}
TableGroup commerce { users
  orders }
`
	f, diags := parser.ParseFile("clean.volt", src)
	info, semDiags := check.File(f)
	diags = append(diags, semDiags...)
	if diag.HasErrors(diags) {
		t.Fatalf("clean file has errors: %v", diags)
	}
	if ws := vet.Run(f, info); len(ws) > 0 {
		t.Errorf("clean file produced warnings: %v", ws)
	}
}

func TestAnalyzerDocs(t *testing.T) {
	for _, a := range vet.All() {
		_, rule := a.(vet.Rule)
		_, fold := a.(vet.Fold)
		if a.Name() == "" || a.Doc() == "" || (!rule && !fold) {
			t.Errorf("analyzer %+v is missing name, doc or a check", a)
		}
	}
}

func TestRefTypeSkipsUnknownTypes(t *testing.T) {
	src := `
Table authors {
  id int64 [pk, increment]
}
Table posts {
  id int [pk, increment]
  author_id int [not null, ref: > authors.id]
  editor_id string [ref: > authors.id]
}
`
	f, diags := parser.ParseFile("unknown.volt", src)
	if diag.HasErrors(diags) {
		t.Fatalf("parse errors: %v", diags)
	}
	info, semDiags := check.File(f)
	if len(semDiags) != 1 || !strings.Contains(semDiags[0].Msg, `unknown type "int64"`) {
		t.Fatalf("want the one unknown-type error, got %v", semDiags)
	}
	if ws := vet.Run(f, info, vet.ByName("reftype")); len(ws) > 0 {
		t.Errorf("reftype compared a column of unknown type: %v", ws)
	}
}
