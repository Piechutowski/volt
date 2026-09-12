package lang_test

// Generated Go is gofmt-canonical by construction (D75): gofmt must be
// the identity on every file the generators write. The goldens prove it
// for their fixtures; this test proves it across the conformance corpus
// and the scaling fixtures, so a new emitter shape cannot slip past.

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// canonicalCheck generates every file of a checked project and asserts
// gofmt leaves each Go file unchanged.
func canonicalCheck(t *testing.T, pr *lang.Project) {
	t.Helper()
	for path, pkg := range pr.Packages {
		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true})
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, f := range files {
				canonicalAssert(t, path+"/"+f.Name, f.Code)
			}
		}
		if pkg.HasRouting() {
			files, err := router.Generate(pkg, router.Options{Source: "package " + path})
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for name, code := range files {
				canonicalAssert(t, path+"/"+name, code)
			}
		}
	}
}

func canonicalAssert(t *testing.T, name string, code []byte) {
	t.Helper()
	if !strings.HasSuffix(name, ".go") {
		return
	}
	formatted, err := format.Source(code)
	if err != nil {
		t.Errorf("%s does not parse: %v", name, err)
		return
	}
	if !bytes.Equal(code, formatted) {
		t.Errorf("%s is not gofmt-canonical; first difference:\n%s", name, firstDiff(string(code), string(formatted)))
	}
}

// firstDiff shows the first differing line of two texts with context.
func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			from := max(i-2, 0)
			var sb strings.Builder
			for j := from; j <= i && j < len(al); j++ {
				sb.WriteString("  " + strings.ReplaceAll(al[j], "\t", "→") + "\n")
			}
			sb.WriteString("gofmt wants:\n")
			for j := from; j <= i && j < len(bl); j++ {
				sb.WriteString("  " + strings.ReplaceAll(bl[j], "\t", "→") + "\n")
			}
			return sb.String()
		}
	}
	return "(identical)"
}

func TestGeneratedGoIsCanonical(t *testing.T) {
	// The scaling fixtures: wide tables, default resources, controllers.
	for _, def := range []bool{true, false} {
		canonicalCheck(t, perfLoad(t, perfFixture(t, 3, 5, def)))
	}

	// Every valid conformance snippet that is a project directory.
	entries, err := os.ReadDir(filepath.Join("conformance", "snippets", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := filepath.Join("conformance", "snippets", "valid", e.Name())
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			dirs, err := lang.PackageDirs(root, root)
			if err != nil {
				t.Fatal(err)
			}
			pr, err := lang.LoadDirs(root, dirs, nil)
			if err != nil {
				t.Fatal(err)
			}
			if diags := lang.Check(pr); diag.HasErrors(diags) {
				t.Fatalf("snippet does not check: %v", diags)
			}
			canonicalCheck(t, pr)
		})
	}
}
