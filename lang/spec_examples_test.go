package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

type specExample struct {
	line    int
	heading string
	marker  string
	text    string
}

var examplePkgRE = regexp.MustCompile(`(?m)^package[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

func specExamples(t *testing.T) []specExample {
	t.Helper()
	lines := strings.Split(specText(t), "\n")
	var out []specExample
	heading := ""
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "#") {
			heading = strings.TrimLeft(l, "# ")
		}
		info, ok := strings.CutPrefix(strings.TrimSpace(l), "```volt")
		if !ok {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) != "```" {
			j++
		}
		out = append(out, specExample{line: i + 1, heading: heading, marker: strings.TrimSpace(info), text: strings.Join(lines[i+1:j], "\n") + "\n"})
		i = j
	}
	if len(out) < 20 {
		t.Fatalf("spec looks truncated: %d volt examples", len(out))
	}
	return out
}

func exampleDiagnostics(t *testing.T, text string) []diag.Diagnostic {
	t.Helper()
	m := examplePkgRE.FindStringSubmatch(text)
	if m == nil {
		f, diags := parser.ParseFile("example.volt", text)
		for _, d := range f.Decls {
			switch d.(type) {
			case *ast.Project, *ast.Table, *ast.TablePartial, *ast.TypeDecl, *ast.Ref, *ast.Records:
			default:
				diags = append(diags, diag.Errorf(d.Pos(), "spec/conformance-and-the-proof-chain", "a declaration of Part II is checked in its package: give the example a package clause or mark it fragment"))
			}
		}
		_, sem := check.File(f)
		return append(diags, sem...)
	}
	root := t.TempDir()
	dir := filepath.Join(root, m[1])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ModFile), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.volt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return Check(pr)
}

// Every volt example of the spec runs: a plain block is a whole file
// the front end accepts (the schema pass without a package clause, the
// project pass with one), a block marked invalid is one it rejects, and
// a block marked fragment is a piece of a file that parses alone.
func TestSpecExamplesRun(t *testing.T) {
	for _, ex := range specExamples(t) {
		where := fmt.Sprintf("docs/spec.md:%d (%s)", ex.line, ex.heading)
		switch ex.marker {
		case "":
			for _, d := range exampleDiagnostics(t, ex.text) {
				if d.Severity == diag.Error {
					t.Errorf("%s: the example is rejected: %s", where, d)
				}
			}
		case "invalid":
			if !diag.HasErrors(exampleDiagnostics(t, ex.text)) {
				t.Errorf("%s: the example is marked invalid, and the front end accepts it", where)
			}
		case "fragment":
			if _, diags := parser.ParseFile("example.volt", ex.text); len(diags) > 0 {
				t.Errorf("%s: the fragment does not parse: %s", where, diags[0])
			}
		default:
			t.Errorf("%s: unknown example marker %q; a volt block is plain, invalid or fragment", where, ex.marker)
		}
	}
}
