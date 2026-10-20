package check_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

var wantRE = regexp.MustCompile(`(?m)^// want: (.+)$`)

var update = flag.Bool("update", false, "rewrite the golden of the invalid schema corpus")

var pkgClauseRE = regexp.MustCompile(`(?im)^package\s+\w+`)

func schemaSnippets(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !pkgClauseRE.Match(src) {
			out = append(out, file)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no schema snippets in %s", dir)
	}
	return out
}

func TestConformanceInvalidDiagnosticsPinned(t *testing.T) {
	var b strings.Builder
	for _, file := range schemaSnippets(t, filepath.Join("..", "conformance", "snippets", "invalid")) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(file)
		f, diags := parser.ParseFile(name, string(src))
		_, semDiags := check.File(f)
		diags = append(diags, semDiags...)
		diag.Sort(diags)
		fmt.Fprintf(&b, "## %s\n", name)
		for _, d := range diags {
			b.WriteString(d.String() + "\n")
		}
		b.WriteString("\n")
	}
	golden := filepath.Join("..", "conformance", "invalid_schema.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if string(want) != b.String() {
		wl, gl := strings.Split(string(want), "\n"), strings.Split(b.String(), "\n")
		for i := range gl {
			if i >= len(wl) || wl[i] != gl[i] {
				w := "<end>"
				if i < len(wl) {
					w = wl[i]
				}
				t.Fatalf("%s differs at line %d:\n--- golden\n%s\n--- got\n%s\n(run with -update after reading the diff)", golden, i+1, w, gl[i])
			}
		}
		t.Fatalf("%s: got is a prefix of the golden", golden)
	}
}

func TestConformanceCorpus(t *testing.T) {
	root := filepath.Join("..", "conformance", "snippets")
	for _, group := range []struct {
		dir       string
		wantError bool
	}{
		{"valid", false},
		{"invalid", true},
	} {
		for _, file := range schemaSnippets(t, filepath.Join(root, group.dir)) {
			file := file
			t.Run(group.dir+"/"+filepath.Base(file), func(t *testing.T) {
				src, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				f, diags := parser.ParseFile(file, string(src))
				_, semDiags := check.File(f)
				diags = append(diags, semDiags...)
				gotError := diag.HasErrors(diags)
				if gotError != group.wantError {
					t.Errorf("want error=%v, got error=%v; diagnostics: %v", group.wantError, gotError, diags)
				}
				if m := wantRE.FindSubmatch(src); m != nil && group.wantError {
					want := strings.TrimSpace(string(m[1]))
					found := false
					for _, d := range diags {
						if strings.Contains(d.Msg, want) {
							found = true
						}
					}
					if !found {
						t.Errorf("rejected, but not for the pinned reason %q; diagnostics: %v", want, diags)
					}
				}
			})
		}
	}
}
