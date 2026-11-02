package vet_test

import (
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
			want := map[int][]string{}
			for i, ln := range lines {
				if m := wantRE.FindStringSubmatch(ln); m != nil {
					for _, n := range strings.Split(m[1], ",") {
						want[i+1] = append(want[i+1], n)
					}
				}
			}
			for _, w := range vet.RunGo("testdata", []vet.GoSource{{Name: filepath.Base(file), Text: string(src)}}, analyzers...) {
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
