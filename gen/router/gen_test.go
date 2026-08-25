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

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/nao/edbml/diag"
)

var update = flag.Bool("update", false, "rewrite golden files")

// generate loads the fixture project and generates the app package.
func generate(t *testing.T) map[string][]byte {
	t.Helper()
	pr, err := lang.Load(filepath.Join("testdata", "fadn"))
	if err != nil {
		t.Fatal(err)
	}
	diags := lang.Check(pr)
	if diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	files, err := Generate(pr.Packages["app"], Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func goldenPath(name string) string {
	return filepath.Join("testdata", "fadn_app_"+strings.TrimSuffix(strings.TrimPrefix(name, "volt_"), ".go")+".go.golden")
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

// TestGoldenGofmtStable proves the generated code is gofmt-clean:
// applying gofmt must be the identity function.
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

// TestGeneratedHeader pins the machine-readable generated-code marker
// (https://golang.org/s/generatedcode) on the first line.
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

// TestNoNamedRoutesCompiles: a routing package whose routes all lack
// helpers (resources create/update/delete carry none) must still yield
// a compilable volt_paths.go — the volt import is omitted when nothing
// references it.
func TestNoNamedRoutesCompiles(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "volt.mod"), []byte("module nohelpers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	routes := "package app\n\nScope / {\n\tresources users [only: (create)]\n}\n"
	if err := os.WriteFile(filepath.Join(src, "app", "r.volt"), []byte(routes), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(src)
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
	if !strings.Contains(paths, "No named routes") {
		t.Errorf("volt_paths.go missing the no-helpers note:\n%s", paths)
	}

	// The real proof: the whole output compiles.
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module nohelpers\n\ngo 1.24\n\n" +
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

func (stub) Create(w http.ResponseWriter, r *volt.Request) error { return nil }

var _ http.Handler = New(Controllers{Users: stub{}})
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

// TestGoldenCompiles builds the goldens with the real Go toolchain,
// alongside the user-side companion file the routes reference
// (controllers are stubbed where the itest exercises them for real).
func TestGoldenCompiles(t *testing.T) {
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module goldencheck\n\ngo 1.24\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
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
		if err := os.WriteFile(filepath.Join(pkgDir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The user-side code the generated files call by declared name:
	// the middleware and the error handler, plus controller stubs.
	user := `package app

import (
	"net/http"

	"github.com/Piechutowski/volt"
)

func BearerAuth(next http.Handler) http.Handler { return next }

func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	volt.DefaultErrorHandler(w, r, err)
}

type stub struct{}

func (stub) Index(w http.ResponseWriter, r *volt.Request) error              { return nil }
func (stub) About(w http.ResponseWriter, r *volt.Request) error              { return nil }
func (stub) Ping(w http.ResponseWriter, r *volt.Request) error               { return nil }
func (stub) Stats(w http.ResponseWriter, r *volt.Request) error              { return nil }
func (stub) Serve(w http.ResponseWriter, r *volt.Request, path string) error { return nil }
func (stub) Show(w http.ResponseWriter, r *volt.Request, id int32) error     { return nil }
func (stub) Create(w http.ResponseWriter, r *volt.Request) error             { return nil }
func (stub) Avatar(w http.ResponseWriter, r *volt.Request, id int32) error   { return nil }

// wire proves New accepts stub implementations of every interface.
var _ http.Handler = New(Controllers{
	Admin: stub{}, Files: stub{}, Home: stub{}, Users: stub{},
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
