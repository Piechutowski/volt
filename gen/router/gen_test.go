package router

import (
	"bytes"
	"flag"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixture(t *testing.T) *lang.Project {
	t.Helper()
	pr, err := lang.Load(disk.Source{}, filepath.Join("testdata", "blog"))
	if err != nil {
		t.Fatal(err)
	}
	diags := lang.Check(pr)
	if diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	return pr
}

func generate(t *testing.T) map[string][]byte {
	t.Helper()
	files, err := Generate(fixture(t).Packages["app"], Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func goldenPath(name string) string {
	base := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), "volt_"), ".go")
	return filepath.Join("testdata", "blog_app_"+base+".go.golden")
}

func TestGolden(t *testing.T) {
	files := generate(t)
	if len(files) != len(Files) {
		t.Fatalf("generated %d files, want %d", len(files), len(Files))
	}
	for _, name := range Files {
		golden := goldenPath(name)
		if *update {
			if err := os.WriteFile(golden, files[name], 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run 'go test ./gen/router -update' to create)", err)
		}
		if !bytes.Equal(files[name], want) {
			t.Errorf("%s differs from %s (re-run with -update after reviewing)\n--- generated ---\n%s", name, golden, files[name])
		}
	}
}

func TestGoldenGofmtStable(t *testing.T) {
	for _, name := range Files {
		src, err := os.ReadFile(goldenPath(name))
		if err != nil {
			t.Fatal(err)
		}
		formatted, err := format.Source(src)
		if err != nil {
			t.Fatalf("%s does not parse: %v", name, err)
		}
		if !bytes.Equal(src, formatted) {
			t.Errorf("%s is not gofmt-stable", name)
		}
	}
}

func TestGeneratedHeader(t *testing.T) {
	re := regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)
	for _, name := range Files {
		src, err := os.ReadFile(goldenPath(name))
		if err != nil {
			t.Fatal(err)
		}
		first := strings.SplitN(string(src), "\n", 2)[0]
		if !re.MatchString(first) {
			t.Errorf("%s first line %q lacks the generated-code marker", name, first)
		}
	}
}

func TestNoNamedRoutesCompiles(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module nohelpers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	routes := "package app\n\ntable users {\n\tid int [pk]\n}\n\nscope / {\n\tresources users [only: (create)]\n}\n"
	if err := os.WriteFile(filepath.Join(src, "app", "r.volt"), []byte(routes), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(disk.Source{}, src)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	files, err := Generate(pr.Packages["app"], Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	paths := string(files["volt_paths.go"])
	if strings.Contains(paths, `"github.com/Piechutowski/volt"`) {
		t.Errorf("volt_paths.go imports volt with no helper to use it:\n%s", paths)
	}
	if strings.Contains(paths, "func ") {
		t.Errorf("volt_paths.go declares a helper with no named route:\n%s", paths)
	}

	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module nohelpers\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(pkgDir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	user := `package app

import (
	"net/http"

	"github.com/Piechutowski/volt"
)

type stub struct{}

func (stub) Create(r volt.Request) volt.Response { return volt.NoContent() }

var _ http.Handler = NewRouter(Controllers{Users: stub{}})
`
	if err := os.WriteFile(filepath.Join(pkgDir, "user.go"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper-less package does not compile:\n%s", out)
	}
}

func TestGoldenCompiles(t *testing.T) {
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module blog\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbFiles, err := model.Generate(fixture(t).Packages["db"], model.Options{Source: "package db"})
	if err != nil {
		t.Fatal(err)
	}
	for voltName, voltCode := range dbFiles {
		if err := os.WriteFile(filepath.Join(dbDir, voltName), voltCode, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hand, _ := filepath.Glob(filepath.Join("testdata", "blog", "db", "*.go"))
	for _, path := range hand {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dbDir, filepath.Base(path)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkgDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range Files {
		src, err := os.ReadFile(goldenPath(name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(pkgDir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	user := `package app

import (
	"net/http"

	"blog/db"
	"github.com/Piechutowski/volt"
)

func BearerAuth(next http.Handler) http.Handler { return next }

func Errors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err)
}

type stub struct{}

func (stub) Index(r volt.Request) volt.Response { return volt.NoContent() }
func (stub) About(r volt.Request) volt.Response { return volt.NoContent() }
func (stub) Ping(r volt.Request) volt.Response { return volt.NoContent() }
func (stub) Stats(r volt.Request, totals db.Totals, feed []db.UserWithPosts) volt.Response {
	return volt.NoContent()
}
func (stub) Serve(r volt.Request, path string) volt.Response { return volt.NoContent() }
func (stub) Show(r volt.Request, id int) volt.Response { return volt.NoContent() }
func (stub) Create(r volt.Request) volt.Response { return volt.NoContent() }
func (stub) Avatar(r volt.Request, id int) volt.Response { return volt.NoContent() }
func (stub) Profile(r volt.Request, id int, user db.User, picked, since []db.User) volt.Response {
	return volt.NoContent()
}
func (stub) ChangeEmail(r volt.Request, id int, body db.UserChangeset) volt.Response {
	return volt.NoContent()
}
func (stub) Signup(r volt.Request, body db.UserChangeset) volt.Response { return volt.NoContent() }
func (stub) Login(r volt.Request, body LoginForm) volt.Response        { return volt.NoContent() }

type me struct{}

func (me) Show(r volt.Request, user db.User) volt.Response { return volt.NoContent() }

func CurrentUser(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(1)) }

type LoginForm struct {
	Email    string
	Password string
}

// wire proves New accepts stub implementations of every interface.
var _ http.Handler = NewRouter(Controllers{
	Admin: stub{}, Files: stub{}, Home: stub{}, Me: me{}, Session: stub{}, Users: stub{},
})
`
	if err := os.WriteFile(filepath.Join(pkgDir, "user.go"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("golden files do not compile:\n%s", out)
	}
}

func TestSinglePackageCompiles(t *testing.T) {
	root := filepath.Join("..", "..", "lang", "conformance", "snippets", "valid", "v65_single_package")
	pr, err := lang.Load(disk.Source{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("snippet has errors: %v", diags)
	}
	pkg := pr.Packages["site"]
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	gomod := "module corpus\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(dir, "site")
	write := func(name string, code []byte) {
		t.Helper()
		path := filepath.Join(site, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	models, err := model.Generate(pkg, model.Options{Source: "package site"})
	if err != nil {
		t.Fatal(err)
	}
	for voltName, voltCode := range models {
		write(voltName, voltCode)
	}
	files, err := Generate(pkg, Options{Source: "package site"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range Files {
		code, ok := files[name]
		if !ok {
			t.Fatalf("%s not generated", name)
		}
		write(name, code)
	}
	write("user.go", []byte(`package site

import (
	"net/http"

	"github.com/Piechutowski/volt"
)

type stub struct{}

func (stub) Purge(r volt.Request, id int) volt.Response { return volt.NoContent() }

var _ http.Handler = NewRouter(Controllers{Tags: stub{}})
`))
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("single-package layout does not compile:\n%s", out)
	}
}

func TestMainPackageCompiles(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	gomod := "module smoke\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	files := map[string]string{
		"go.mod": gomod,
		"schema.volt": `package main

table posts {
  id    int [pk]
  title string
}

scope /api {
	resources posts [default]
	get /first PostGet [name: first]
}
`,
		"main.go": `package main

import "net/http"

func main() { http.ListenAndServe(":0", NewRouter(Controllers{})) }
`,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr, err := lang.Load(disk.Source{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("project has errors: %v", diags)
	}
	pkg := pr.Packages["."]
	models, err := model.Generate(pkg, model.Options{Source: "package ."})
	if err != nil {
		t.Fatal(err)
	}
	for voltName, voltCode := range models {
		if err := os.WriteFile(filepath.Join(dir, voltName), voltCode, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := Generate(pkg, Options{Source: "package ."})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := out[ClientFile]; has {
		t.Fatalf("a client was generated for package main, which Go cannot import")
	}
	for name, code := range out {
		if err := os.WriteFile(filepath.Join(dir, name), code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("main-package layout does not compile:\n%s", b)
	}
}

func TestClientBesideGolden(t *testing.T) {
	files, err := Generate(fixture(t).Packages["app"], Options{Source: "package app", ClientBeside: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := files[ClientFile]; has {
		t.Fatalf("%s generated alongside the beside client", ClientFile)
	}
	got, ok := files[ClientBesideFile]
	if !ok {
		t.Fatalf("%s not generated", ClientBesideFile)
	}
	golden := filepath.Join("testdata", "blog_app_client_beside.go.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from %s:\n%s", ClientBesideFile, golden, got)
	}
	if formatted, _ := format.Source(got); !bytes.Equal(got, formatted) {
		t.Errorf("%s is not gofmt-canonical", ClientBesideFile)
	}
}

func TestCorpusCompiles(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []corpus.Spec{{Packages: 2, Tables: 3, Columns: 9, Volt: repoRoot}, {Tables: 3, Columns: 9, Single: true, Volt: repoRoot}} {
		dir := t.TempDir()
		if err := corpus.Write(dir, spec); err != nil {
			t.Fatal(err)
		}
		pr, err := lang.Load(disk.Source{}, dir)
		if err != nil {
			t.Fatal(err)
		}
		if diags := lang.Check(pr); diag.HasErrors(diags) {
			t.Fatalf("%+v: %v", spec, diags)
		}
		for path, pkg := range pr.Packages {
			write := func(name string, code []byte) {
				t.Helper()
				full := filepath.Join(pkg.Dir, name)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, code, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if pkg.HasSchema() {
				files, err := model.Generate(pkg, model.Options{Source: "package " + path})
				if err != nil {
					t.Fatal(err)
				}
				for voltName, voltCode := range files {
					write(voltName, voltCode)
				}
			}
			if pkg.HasRouting() {
				files, err := Generate(pkg, Options{Source: "package " + path})
				if err != nil {
					t.Fatal(err)
				}
				for name, code := range files {
					write(name, code)
				}
			}
		}
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%+v does not compile:\n%s", spec, out)
		}
	}
}

func TestKeyNamedLikeClientLocalCompiles(t *testing.T) {
	snippetCompiles(t, "v93_key_named_out")
}

// TestKeyNamedLikeRouteNamesCompiles proves a table keyed on a column named
// like a name the route files' signatures take (c, w, r, opts, cs, volt,
// ctx) routes by its key parameter, which takes a trailing underscore, and
// the generated program builds (spec §generated-query-surface,
// §route-paths).
func TestKeyNamedLikeRouteNamesCompiles(t *testing.T) {
	snippetCompiles(t, "v95_key_named_like_route_names")
}

func snippetCompiles(t *testing.T, name string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "lang", "conformance", "snippets", "valid", name)
	dir := t.TempDir()
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "go.mod" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module corpus\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(disk.Source{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("%s has errors: %v", name, diags)
	}
	for path, pkg := range pr.Packages {
		write := func(file string, code []byte) {
			t.Helper()
			full := filepath.Join(pkg.Dir, file)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, code, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{Source: "package " + path})
			if err != nil {
				t.Fatal(err)
			}
			for voltName, voltCode := range files {
				write(voltName, voltCode)
			}
		}
		if pkg.HasRouting() {
			files, err := Generate(pkg, Options{Source: "package " + path})
			if err != nil {
				t.Fatal(err)
			}
			for file, code := range files {
				write(file, code)
			}
		}
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s does not compile:\n%s", name, out)
	}
}

// TestClientImportsTimeForTimeParamsAlone proves the client imports time
// only for a query parameter of that type: a route's timeout: is the
// router's business (spec §route-loads, §generated-client), and a client
// importing time for it alone does not compile.
func TestClientImportsTimeForTimeParamsAlone(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module timeouts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	routes := "package app\n\ntable metrics {\n\tid int [pk]\n}\n\nscope / {\n\tget /metrics MetricList [timeout: 5]\n\tget /slow Metrics.Slow [timeout: 2]\n}\n"
	if err := os.WriteFile(filepath.Join(src, "app", "r.volt"), []byte(routes), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(disk.Source{}, src)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	files, err := Generate(pr.Packages["app"], Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["volt_router.go"]), "\t\"time\"\n") {
		t.Errorf("volt_router.go does not import time for its timeouts:\n%s", files["volt_router.go"])
	}
	if strings.Contains(string(files[filepath.Join("client", "volt_client.go")]), "\t\"time\"\n") {
		t.Errorf("volt_client.go imports time with no time parameter to use it:\n%s", files[filepath.Join("client", "volt_client.go")])
	}
}

// TestLoadTimeParamCompiles proves a controller route whose load binds a
// time.Time from the query string imports time in its router, with no
// timeout to import it, and the generated program builds (spec
// §route-loads).
func TestLoadTimeParamCompiles(t *testing.T) {
	snippetCompiles(t, "v108_route_load_time_param")
}

// TestPackageNamedHTTPClientCompiles proves a package named http that
// routes its own tables through query routes alone generates a client
// that builds: the constructor's empty Header needs no net/http import
// to collide with the package's own (spec §generated-client,
// §projects-and-packages).
func TestPackageNamedHTTPClientCompiles(t *testing.T) {
	snippetCompiles(t, "v112_package_named_http")
}
