package golang_test

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/naming"
)

// TestColumnsNamedLikeLocalsCompile proves that no key column and no
// select parameter can take a name a generated data-file method declares
// for itself or calls (spec §generated-query-surface): the snippet keys
// tables on columns named err, res, n, out, rows, q, nil, append and like
// the scan function and statement constants the generator once named
// ledgerScan and ledgerGetSQL, reads another by single keys of such names
// and selects with parameters named rows, err, out, entryScan and
// entryByHelpersSQL, and its generated files build.
func TestColumnsNamedLikeLocalsCompile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "lang", "conformance", "snippets", "valid", "v94_columns_named_like_locals", "db", "schema.volt")
	schema, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	schemaCompiles(t, repoRoot, schema)
}

// TestPredeclaredNamesCompile proves the names a key column's parameter
// and a select parameter avoid among Go's predeclared identifiers are
// enough (spec §generated-query-surface, §predicates): a table keyed on a
// column of every predeclared name, a unique column of every one, and a
// scalar and a list select parameter of every one but nil and append,
// which the checker refuses, generate files that build.
func TestPredeclaredNamesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	handleTakesConstructor := map[string]bool{"new": true}
	var b strings.Builder
	b.WriteString("package db\n\nTable posts {\n  id    int [pk]\n  label string\n  n     int\n}\n\n")
	var unique strings.Builder
	names := types.Universe.Names()
	params := 0
	for _, name := range names {
		if handleTakesConstructor[name] {
			continue
		}
		fmt.Fprintf(&b, "Table k_%s {\n  %s int [pk]\n  label string\n}\n\n", name, name)
		fmt.Fprintf(&unique, "  %s string [unique]\n", name)
	}
	fmt.Fprintf(&b, "Table uniques {\n  id int [pk]\n%s}\n\n", unique.String())
	for _, name := range names {
		if naming.BodyBuiltin(name) {
			continue
		}
		fmt.Fprintf(&b, "Select s_%s for posts where label = :%s\nSelect l_%s for posts where n in :%s\n", name, name, name, name)
		params += 2
	}
	if len(names) < 40 || params < 70 {
		t.Fatalf("only %d predeclared names and %d select parameters", len(names), params)
	}
	schemaCompiles(t, repoRoot, []byte(b.String()))
}

func schemaCompiles(t *testing.T, repoRoot string, schema []byte) {
	t.Helper()
	dir := t.TempDir()
	gomod := "module localscheck\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "schema.volt"), schema, 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("snippet has errors: %v", diags)
	}
	pkg := pr.PackageAt(pkgDir)
	if pkg == nil {
		t.Fatal("package db not loaded")
	}
	pl := pkg.Plan()
	fns := pkg.SelectFns()
	if len(fns) == 0 {
		t.Fatal("the snippet declares no select")
	}
	opts := golang.Options{Package: pkg.Name, Source: "schema.volt"}
	for name, gen := range map[string]func() ([]byte, error){
		"volt_models.go":   func() ([]byte, error) { return golang.Models(pl, opts) },
		"volt_queries.go":  func() ([]byte, error) { return golang.Queries(pl, opts) },
		"volt_dyn.go":      func() ([]byte, error) { return golang.Dyn(pl, opts) },
		"volt_validate.go": func() ([]byte, error) { return golang.Validators(pl, pkg.Validators, opts) },
		"volt_selects.go":  func() ([]byte, error) { return golang.Selects(pl, fns, opts) },
	} {
		code, err := gen()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, name), code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code over columns named like the locals does not compile:\n%s", out)
	}
}

// TestGeneratedLocalsTakeTheLocalForm proves every local a generated data
// file declares inside a function, function literal parameters included,
// and every unexported package-level name it declares, has the form
// naming.LocalName reports (spec §generated-query-surface), across every
// generator golden and generated fixture.
func TestGeneratedLocalsTakeTheLocalForm(t *testing.T) {
	var files []string
	for _, pattern := range []string{
		filepath.Join("testdata", "*.go.golden"),
		filepath.Join("..", "..", "itest", "volt_*.go"),
		filepath.Join("..", "..", "..", "itest", "blog", "db", "volt_*.go"),
	} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 20 {
		t.Fatalf("only %d generated data files found", len(files))
	}
	checked, helpers := 0, 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fset := gotoken.NewFileSet()
		f, err := goparser.ParseFile(fset, path, src, goparser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		bad := func(id *goast.Ident) {
			checked++
			if id.Name != "_" && !naming.LocalName(id.Name) {
				t.Errorf("%s: %s does not take the form of the generated locals and helpers", fset.Position(id.Pos()), id.Name)
			}
		}
		for _, d := range f.Decls {
			if gd, ok := d.(*goast.GenDecl); ok {
				for _, spec := range gd.Specs {
					switch sp := spec.(type) {
					case *goast.ValueSpec:
						for _, id := range sp.Names {
							if id.Name != "_" && !id.IsExported() {
								bad(id)
								helpers++
							}
						}
					case *goast.TypeSpec:
						if !sp.Name.IsExported() {
							bad(sp.Name)
							helpers++
						}
					}
				}
			}
			fd, ok := d.(*goast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if fd.Recv == nil && !fd.Name.IsExported() {
				bad(fd.Name)
				helpers++
			}
			goast.Inspect(fd.Body, func(n goast.Node) bool {
				switch x := n.(type) {
				case *goast.AssignStmt:
					if x.Tok == gotoken.DEFINE {
						for _, l := range x.Lhs {
							if id, ok := l.(*goast.Ident); ok {
								bad(id)
							}
						}
					}
				case *goast.ValueSpec:
					for _, id := range x.Names {
						bad(id)
					}
				case *goast.RangeStmt:
					if x.Tok == gotoken.DEFINE {
						for _, e := range []goast.Expr{x.Key, x.Value} {
							if id, ok := e.(*goast.Ident); ok {
								bad(id)
							}
						}
					}
				case *goast.FuncLit:
					for _, field := range x.Type.Params.List {
						for _, id := range field.Names {
							bad(id)
						}
					}
				}
				return true
			})
		}
	}
	if checked < 1000 || helpers < 100 {
		t.Fatalf("only %d locals and %d package-level helpers checked; the walk misses the generated files", checked, helpers)
	}
}
