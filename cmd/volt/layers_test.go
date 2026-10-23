package main

import (
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"golang.org/x/tools/go/packages"
)

const rtPath = module + "/nao/rt"

var networkPackages = []string{module + "/gen/router", module + "/gen/model", module + "/lsp", module + "/cmd"}

func stdlibPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

func networkPath(path string) bool {
	if path == module {
		return true
	}
	for _, p := range networkPackages {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func closureOf(pkg *packages.Package) map[string][]string {
	via := map[string][]string{pkg.PkgPath: {pkg.PkgPath}}
	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		deps := make([]*packages.Package, 0, len(p.Imports))
		for _, q := range p.Imports {
			deps = append(deps, q)
		}
		sort.Slice(deps, func(i, j int) bool { return deps[i].PkgPath < deps[j].PkgPath })
		for _, q := range deps {
			if _, seen := via[q.PkgPath]; seen {
				continue
			}
			via[q.PkgPath] = append(append([]string(nil), via[p.PkgPath]...), q.PkgPath)
			walk(q)
		}
	}
	walk(pkg)
	return via
}

// TestLayersDataUsesNoNetwork proves the layer rule (spec
// §data-and-network-layers): no package of the data layer under nao/
// reaches the root volt package, the route generator, the generation
// driver, the language server or the tool, directly or through anything
// it imports.
func TestLayersDataUsesNoNetwork(t *testing.T) {
	checked := 0
	packages.Visit(gatePackages(t), nil, func(pkg *packages.Package) {
		if !strings.HasPrefix(pkg.PkgPath, module+"/nao/") {
			return
		}
		checked++
		for path, chain := range closureOf(pkg) {
			if networkPath(path) {
				t.Errorf("%s reaches %s: %s", pkg.PkgPath, path, strings.Join(chain, " -> "))
			}
		}
	})
	if checked < 8 {
		t.Fatalf("only %d packages under nao/ loaded", checked)
	}
}

// TestLayersCheckersImportNoGenerator proves no checker imports a
// generator (spec §data-and-network-layers): no package of the front end
// lang/ and of the language server reaches nao/gen/ or gen/, directly or
// through anything it imports.
func TestLayersCheckersImportNoGenerator(t *testing.T) {
	checked := 0
	packages.Visit(gatePackages(t), nil, func(pkg *packages.Package) {
		p := pkg.PkgPath
		if p != module+"/lang" && !strings.HasPrefix(p, module+"/lang/") && p != module+"/lsp" && !strings.HasPrefix(p, module+"/lsp/") {
			return
		}
		checked++
		for path, chain := range closureOf(pkg) {
			if strings.HasPrefix(path, module+"/nao/gen/") || strings.HasPrefix(path, module+"/gen/") {
				t.Errorf("%s reaches %s: %s", p, path, strings.Join(chain, " -> "))
			}
		}
	})
	if checked < 9 {
		t.Fatalf("only %d front-end packages loaded", checked)
	}
}

// TestLayersRuntimeIsStdlib proves nao/rt, the one package a data-only
// program links from Volt (spec §data-and-network-layers), imports the
// standard library alone and never net/http, through anything it imports
// either.
func TestLayersRuntimeIsStdlib(t *testing.T) {
	var rt *packages.Package
	packages.Visit(gatePackages(t), nil, func(pkg *packages.Package) {
		if pkg.PkgPath == rtPath {
			rt = pkg
		}
	})
	if rt == nil {
		t.Fatal("nao/rt not loaded")
	}
	if len(rt.Imports) < 5 {
		t.Fatalf("nao/rt imports only %d packages; the walk sees too little", len(rt.Imports))
	}
	for path, chain := range closureOf(rt) {
		if path == rtPath {
			continue
		}
		if !stdlibPath(path) || path == "net/http" {
			t.Errorf("nao/rt reaches %s: %s", path, strings.Join(chain, " -> "))
		}
	}
}

// TestLayersGeneratedDataImports proves every generated data file, in the
// generator goldens and the generated fixtures, imports only the standard
// library other than net/http, nao/rt and the packages of its schema's
// declared types (spec §data-and-network-layers, §types).
func TestLayersGeneratedDataImports(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	type unit struct {
		files   []string
		schemas []string
	}
	var units []unit
	goldens, err := filepath.Glob(filepath.Join(root, "nao", "gen", "golang", "testdata", "*.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range goldens {
		base := strings.TrimSuffix(filepath.Base(g), ".go.golden")
		for _, kind := range []string{"_queries", "_dyn", "_validate", "_selects", "_migrations"} {
			base = strings.TrimSuffix(base, kind)
		}
		schema := filepath.Join(root, "nao", "gen", "testdata", base+".volt")
		if base == "blog" {
			schema = filepath.Join(root, "nao", "migrate", "testdata", "blog.volt")
		}
		if _, err := os.Stat(schema); err != nil {
			t.Fatalf("%s: no schema source: %v", g, err)
		}
		units = append(units, unit{[]string{g}, []string{schema}})
	}
	for _, dir := range []string{filepath.Join(root, "nao", "itest"), filepath.Join(root, "itest", "blog", "db")} {
		var u unit
		for _, kind := range []string{"models", "queries", "dyn", "validate", "selects", "migrations"} {
			if f := filepath.Join(dir, "volt_"+kind+".go"); fileExists(f) {
				u.files = append(u.files, f)
			}
		}
		u.schemas, _ = filepath.Glob(filepath.Join(dir, "*.volt"))
		if len(u.files) < 5 || len(u.schemas) == 0 {
			t.Fatalf("%s: %d generated data files, %d schema files", dir, len(u.files), len(u.schemas))
		}
		units = append(units, u)
	}
	files, declared := 0, 0
	for _, u := range units {
		allowed := map[string]bool{rtPath: true}
		for _, s := range u.schemas {
			for _, path := range declaredTypePaths(t, s) {
				allowed[path] = true
				declared++
			}
		}
		for _, f := range u.files {
			files++
			for _, path := range fileImports(t, f) {
				if (!stdlibPath(path) && !allowed[path]) || path == "net/http" {
					t.Errorf("%s imports %s, neither the standard library other than net/http, nao/rt nor a declared type's package", f, path)
				}
			}
		}
	}
	if files < 60 || declared == 0 {
		t.Fatalf("checked %d generated data files and %d declared-type packages; the walk sees too little", files, declared)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	f, err := goparser.ParseFile(gotoken.NewFileSet(), path, nil, goparser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func declaredTypePaths(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := parser.ParseFile(filepath.Base(path), string(src))
	var out []string
	for _, d := range f.Decls {
		if td, ok := d.(*ast.TypeDecl); ok && td.From != nil {
			out = append(out, td.From.Tok.Val)
		}
	}
	return out
}

// TestLayersDataOnlyProgram proves the data layer stands alone (spec
// §data-and-network-layers): nao/itest, and a program generated here over
// a package of schema files and a pipeline but no scope, link nothing from
// Volt but nao/rt and nothing of net/http, and the program builds.
func TestLayersDataOnlyProgram(t *testing.T) {
	var fixture *packages.Package
	packages.Visit(gatePackages(t), nil, func(pkg *packages.Package) {
		if pkg.PkgPath == module+"/nao/itest" {
			fixture = pkg
		}
	})
	if fixture == nil {
		t.Fatal("nao/itest not loaded")
	}
	dataOnly(t, fixture, module+"/nao/itest")

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module dataonly\n\ngo 1.27\n\nrequire github.com/Piechutowski/volt v0.0.0\n\nreplace github.com/Piechutowski/volt => " + root + "\n",
		"db/schema.volt": "package db\n\nTable authors {\n  id    int    [pk, increment]\n  email string [unique]\n  name  string [required]\n}\n\n" +
			"Table posts {\n  id        int    [pk, increment]\n  author_id int    [ref: authors.id]\n  title     string\n  views     int    [default: 0]\n}\n\n" +
			"Select popular for posts where views >= :least [order: (views desc)]\n",
		"db/pipe.volt": "package db\n\nPipeline api {\n  use volt.RequestID\n}\n",
		"db/0001__init.volt": "package db\n\nMigration 0001__init {\n  create table authors {\n    id    int    [pk, increment]\n    email string [unique]\n    name  string\n  }\n" +
			"  create table posts {\n    id        int    [pk, increment]\n    author_id int    [ref: authors.id]\n    title     string\n    views     int    [default: 0]\n  }\n}\n",
		"cmd/app/main.go": "package main\n\nimport (\n\t\"context\"\n\t\"database/sql\"\n\t\"fmt\"\n\n\t\"dataonly/db\"\n)\n\n" +
			"func main() {\n\tctx := context.Background()\n\tconn, err := sql.Open(\"sqlite3\", \":memory:\")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n" +
			"\tif err := db.Migrate(ctx, conn); err != nil {\n\t\tfmt.Println(err)\n\t}\n\tq := db.New(conn)\n" +
			"\ta, err := q.AuthorCreate(ctx, db.AuthorNew().EmailSet(\"ann@example.com\").NameSet(\"Ann\"))\n\tfmt.Println(a, err)\n" +
			"\tposts, err := q.PostPopular(ctx, 10)\n\tfmt.Println(len(posts), err)\n}\n",
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr, err := lang.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("the data-only package has errors: %v", diags)
	}
	pkg := pr.PackageAt(filepath.Join(dir, "db"))
	if pkg == nil || !pkg.HasSchema() || pkg.HasRouting() {
		t.Fatal("the fixture is not a package of schema files without a scope")
	}
	gen, err := model.Generate(pkg, model.Options{Source: "package db"})
	if err != nil {
		t.Fatal(err)
	}
	if len(gen) < 6 {
		t.Fatalf("only %d data files generated", len(gen))
	}
	for _, f := range gen {
		if err := os.WriteFile(filepath.Join(pkg.Dir, f.Name), f.Code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the data-only program does not build:\n%s", out)
	}
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps, Dir: dir}, "./cmd/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Errors) > 0 {
		t.Fatalf("loading the program: %+v", loaded)
	}
	dataOnly(t, loaded[0], "dataonly/db")
}

func dataOnly(t *testing.T, pkg *packages.Package, data string) {
	t.Helper()
	via := closureOf(pkg)
	if _, ok := via[data]; !ok {
		t.Fatalf("%s does not reach %s", pkg.PkgPath, data)
	}
	if _, ok := via[rtPath]; !ok {
		t.Fatalf("%s does not reach nao/rt; the walk sees too little", pkg.PkgPath)
	}
	for path, chain := range via {
		if path == module || (strings.HasPrefix(path, module+"/") && path != rtPath && !strings.HasPrefix(path, data)) || path == "net/http" {
			t.Errorf("%s links %s: %s", pkg.PkgPath, path, strings.Join(chain, " -> "))
		}
	}
}
