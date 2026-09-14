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

// TestConformanceCorpus runs the schema half of the spec's snippet
// corpus (the .dbml entries of lang/conformance/snippets) through the
// single-file front end: every valid snippet must produce zero errors,
// every invalid snippet at least one. The verdicts were pinned against
// the upstream @dbml/parse compiler while the cross-check existed
// (retired at 0 disagreements, D54).
var wantRE = regexp.MustCompile(`(?m)^// want: (.+)$`)

var update = flag.Bool("update", false, "rewrite the golden of the invalid schema corpus")

// TestConformanceInvalidDiagnosticsPinned holds every diagnostic of
// every invalid .dbml snippet, recovery included (§3.2 rule 6); see
// the project half's test of the same name. Refresh with -update
// after reading the diff.
func TestConformanceInvalidDiagnosticsPinned(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "conformance", "snippets", "invalid", "*.dbml"))
	if err != nil || len(files) == 0 {
		t.Fatal("no invalid .dbml snippets")
	}
	var b strings.Builder
	for _, file := range files {
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
	golden := filepath.Join("..", "conformance", "invalid_dbml.golden")
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
		files, err := filepath.Glob(filepath.Join(root, group.dir, "*.dbml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no snippets in %s", filepath.Join(root, group.dir))
		}
		for _, file := range files {
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
				// `// want: <text>` pins the reason an invalid snippet is
				// rejected: the rule its comment names, not an accident.
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
