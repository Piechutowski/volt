package golang_test

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/naming"
)

// TestColumnsNamedLikeLocalsCompile proves that no key column and no
// select parameter can take a name a generated data-file method declares
// for itself (spec §generated-query-surface): the snippet keys a table on
// columns named err, res, n, out, rows and q, reads another by single
// keys of those names and selects with parameters named rows, err and out,
// and its generated files build.
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
// has the form naming.LocalName reports (spec §generated-query-surface),
// across every generator golden and generated fixture.
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
	checked := 0
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
				t.Errorf("%s: local %s does not take the generated locals' form", fset.Position(id.Pos()), id.Name)
			}
		}
		for _, d := range f.Decls {
			fd, ok := d.(*goast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
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
	if checked < 1000 {
		t.Fatalf("only %d locals checked; the walk misses the generated bodies", checked)
	}
}
