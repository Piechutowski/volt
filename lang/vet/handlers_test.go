package vet_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/vet"
)

func TestGoAnalyzers(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go testdata files")
	}
	for _, file := range files {
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
			var bodies []vet.Body
			for _, ln := range lines {
				spec, ok := strings.CutPrefix(strings.TrimSpace(ln), "// bodies:")
				if !ok {
					continue
				}
				for _, item := range strings.Split(spec, ",") {
					var b vet.Body
					if _, err := fmt.Sscan(item, &b.Action, &b.At, &b.Type); err != nil {
						t.Fatalf("bodies header item %q: %v", item, err)
					}
					bodies = append(bodies, b)
				}
			}
			want := map[int][]string{}
			for i, ln := range lines {
				if m := wantRE.FindStringSubmatch(ln); m != nil {
					for _, n := range strings.Split(m[1], ",") {
						want[i+1] = append(want[i+1], n)
					}
				}
			}
			for _, w := range vet.RunGo("testdata", []vet.GoSource{{Name: filepath.Base(file), Text: string(src)}}, bodies, analyzers...) {
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

func TestGoRulesReadThePackage(t *testing.T) {
	handler := vet.GoSource{Name: "home.go", Text: "package app\n\nimport \"github.com/Piechutowski/volt\"\n\ntype Home struct{}\n\nfunc (Home) Index(r volt.Request) volt.Response { return volt.Render(stamp()) }\n"}
	helper := vet.GoSource{Name: "stamp.go", Text: "package app\n\nimport (\n\t\"os\"\n\t\"time\"\n)\n\nfunc stamp() string { return os.Getenv(\"HOST\") + time.Now().String() }\n"}
	analyzers := []vet.Analyzer{vet.ByName("handlerimports"), vet.ByName("handlerclock")}
	var got []string
	for _, w := range vet.RunGo("app", []vet.GoSource{handler, helper}, nil, analyzers...) {
		got = append(got, fmt.Sprintf("%s:%d %s", w.Pos.Filename(), w.Pos.Line(), w.Code))
	}
	want := []string{"app/stamp.go:4 vet/handlerimports", "app/stamp.go:8 vet/handlerclock"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("a helper of a handler's package is held pure as the handler is:\n got %v\nwant %v", got, want)
	}
	if ws := vet.RunGo("app", []vet.GoSource{helper}, nil, analyzers...); len(ws) != 0 {
		t.Errorf("a package that declares no handler is not held pure: %v", ws)
	}
}
