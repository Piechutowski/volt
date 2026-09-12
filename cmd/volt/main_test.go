package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWorkspaceLayout proves the layout `-o` and `-parts` exist for
// (D77): one schema.volt at the root of a go.work workspace, a server
// module that is package main and gets models, queries and router, a
// client module that is package main and gets models and the client
// beside them — each wired by its own go:generate line. The binary is
// built, `go generate` runs it, and both modules build.
func TestWorkspaceLayout(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if out, err := exec.Command("go", "build", "-o", filepath.Join(binDir, "volt"), ".").CombinedOutput(); err != nil {
		t.Fatalf("building volt: %v\n%s", err, out)
	}

	root := t.TempDir()
	files := map[string]string{
		"go.work": "go 1.27\n\nuse (\n\t.\n\t./api\n\t./gui\n\t" + repoRoot + "\n)\n",
		"go.mod":  "module shop\n\ngo 1.27\n",
		"main.go": "package main\n\nfunc main() {}\n",
		"schema.volt": `package main

Table posts {
  id    integer [pk, increment]
  title text    [not null]
}

Select picked for posts where id in :ids [order: (id asc)]

Scope /api [name: api] {
	resources posts [default]
	get /picked PostPicked
}
`,
		"api/go.mod": "module shop/api\n\ngo 1.27\n",
		"api/main.go": `package main

//go:generate volt gen -o . -parts models,queries,router ../schema.volt

import "net/http"

func main() { http.ListenAndServe(":0", NewRouter(Controllers{Queries: New(nil)})) }
`,
		"gui/go.mod": "module shop/gui\n\ngo 1.27\n",
		"gui/main.go": `package main

//go:generate volt gen -o . -parts models,client ../schema.volt

func main() { _ = NewClient("http://localhost:8888") }
`,
	}
	for name, src := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	run(root, "generate", "./api", "./gui") // ./... stops at the root module in a workspace

	// The parts landed where their go:generate lines put them, and
	// nowhere else.
	for _, want := range []string{
		"api/nao_models.go", "api/nao_queries.go", "api/nao_dyn.go", "api/nao_selects.go",
		"api/volt_handlers.go", "api/volt_router.go", "api/volt_paths.go", "api/volt_routes.go",
		"gui/nao_models.go", "gui/volt_client.go",
	} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("%s not written", want)
		}
	}
	for _, absent := range []string{
		"api/volt_client.go", "api/client", "gui/nao_queries.go", "gui/volt_router.go",
		"nao_models.go", "volt_router.go",
	} {
		if _, err := os.Stat(filepath.Join(root, absent)); err == nil {
			t.Errorf("%s written, but no part asked for it", absent)
		}
	}
	run(filepath.Join(root, "api"), "build", "./...")
	run(filepath.Join(root, "gui"), "build", "./...")
	run(filepath.Join(root, "gui"), "vet", "./...")
}
