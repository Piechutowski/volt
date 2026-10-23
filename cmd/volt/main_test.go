package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

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
  id    int [pk, increment]
  title string
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
	run(root, "generate", "./api", "./gui")

	for _, want := range []string{
		"api/volt_models.go", "api/volt_queries.go", "api/volt_dyn.go", "api/volt_selects.go",
		"api/volt_handlers.go", "api/volt_router.go", "api/volt_paths.go", "api/volt_routes.go",
		"gui/volt_models.go", "gui/volt_client.go",
	} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("%s not written", want)
		}
	}
	for _, absent := range []string{
		"api/volt_client.go", "api/client", "gui/volt_queries.go", "gui/volt_router.go",
		"volt_models.go", "volt_router.go",
	} {
		if _, err := os.Stat(filepath.Join(root, absent)); err == nil {
			t.Errorf("%s written, but no part asked for it", absent)
		}
	}
	run(filepath.Join(root, "api"), "build", "./...")
	run(filepath.Join(root, "gui"), "build", "./...")
	run(filepath.Join(root, "gui"), "vet", "./...")
}

func TestStressWritesABuildableProject(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt"}, args...))
	}
	dir := filepath.Join(t.TempDir(), "big")
	if err := volt("stress", "-tables", "3", "-columns", "9", "-volt", repoRoot, dir); err != nil {
		t.Fatalf("stress: %v", err)
	}
	pr, err := lang.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("stress project does not check:\n%v", diags)
	}
	if len(pr.Packages) != 1 {
		t.Fatalf("%d packages, want one at the root", len(pr.Packages))
	}
	for _, pkg := range pr.Packages {
		if pkg.Name != "main" || len(pkg.Schema().Tables) != 3 || len(pkg.Routes) < 15 {
			t.Fatalf("package %s: %d tables, %d routes", pkg.Name, len(pkg.Schema().Tables), len(pkg.Routes))
		}
	}
	if err := volt("gen", "--sql", dir); err != nil {
		t.Fatalf("gen: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stress project does not build:\n%s", out)
	}
	err = volt("stress", "-tables", "1", "-columns", "1", dir)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("stress over %s: %v, want a refusal", dir, err)
	}
}

func TestGenOutRefusesBareType(t *testing.T) {
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt"}, args...))
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module shop\n\ngo 1.27\n",
		"db/schema.volt": `package db

Type OrderState string

Table orders {
  id     int        [pk, increment]
  status OrderState [default: "pending"]
}
`,
		"db/order_state.go": "package db\n\n// OrderState is the state of an order (§types).\ntype OrderState string\n",
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
	out := filepath.Join(root, "api")
	err := volt("gen", "-o", out, "-parts", "models", filepath.Join(root, "db"))
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "Type OrderState of package db is declared in that package's own Go files") ||
		!strings.Contains(err.Error(), "Type pkg.OrderState string from") {
		t.Fatalf("gen -o with a bare Type: exit %d (%v), want 2 naming the fix", exitCode(err), err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("the refused gen wrote %s", out)
	}
	if err := volt("gen", "-o", out, "-parts", "sql", filepath.Join(root, "db")); err != nil {
		t.Fatalf("gen -o -parts sql writes no Go, so a bare Type does not matter: %v", err)
	}
	for _, own := range []string{filepath.Join(root, "db"), filepath.Join(root, "db") + string(filepath.Separator)} {
		if err := volt("gen", "-o", own, "-parts", "models", filepath.Join(root, "db")); err != nil {
			t.Fatalf("gen -o %s names the package's own directory, where its Go files are: %v", own, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "db", "volt_models.go")); err != nil {
		t.Errorf("gen -o into the package's own directory wrote no models: %v", err)
	}
	if err := volt("gen", filepath.Join(root, "db")); err != nil {
		t.Fatalf("gen beside the schema: %v", err)
	}
}

func TestGenRemovesOldDataFileNames(t *testing.T) {
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt"}, args...))
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod":            "module shop\n\ngo 1.27\n",
		"db/schema.volt":    "package db\n\nTable posts {\n  id    int    [pk, increment]\n  title string\n}\n",
		"db/nao_models.go":  "// Code generated by nao gen from package db; DO NOT EDIT.\n\npackage db\n",
		"db/nao_schema.sql": "-- Code generated by nao gen sqlite from package db; DO NOT EDIT.\n",
		"db/nao_dyn.go":     "package db\n\n// PostsHandWritten is not generated (§projects-and-packages).\nfunc PostsHandWritten() {}\n",
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
	if err := volt("gen", "--sql", filepath.Join(root, "db")); err != nil {
		t.Fatalf("gen: %v", err)
	}
	for _, gone := range []string{"nao_models.go", "nao_schema.sql"} {
		if _, err := os.Stat(filepath.Join(root, "db", gone)); err == nil {
			t.Errorf("%s carries the generated marker and should be removed under its volt_ name", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "db", "nao_dyn.go")); err != nil {
		t.Errorf("nao_dyn.go has no generated marker, so gen must leave it: %v", err)
	}
	for _, made := range []string{"volt_models.go", "volt_schema.sql", "volt_queries.go"} {
		if _, err := os.Stat(filepath.Join(root, "db", made)); err != nil {
			t.Errorf("gen wrote no %s: %v", made, err)
		}
	}
}

// TestGenRouteFilesFollowScopes proves a package's route files exist
// exactly while it declares a scope (spec §data-and-network-layers): a
// pipeline alone writes none, and once the last scope goes, gen
// removes the route files it wrote and leaves a hand-written one.
func TestGenRouteFilesFollowScopes(t *testing.T) {
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt"}, args...))
	}
	root := t.TempDir()
	db := filepath.Join(root, "db")
	write := func(name, src string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module blog\n\ngo 1.27\n")
	write("db/schema.volt", "package db\n\nTable authors {\n  id   int    [pk, increment]\n  name string\n}\n")
	write("db/pipe.volt", "package db\n\nPipeline api {\n  use volt.RequestID\n}\n")
	data := []string{"volt_models.go", "volt_queries.go", "volt_dyn.go"}
	routes := []string{"volt_handlers.go", "volt_router.go", "volt_paths.go", "volt_routes.go", filepath.Join("client", "volt_client.go")}
	exist := func(name string) bool {
		_, err := os.Stat(filepath.Join(db, name))
		return err == nil
	}
	if err := volt("gen", db); err != nil {
		t.Fatalf("gen over a pipeline and no scope: %v", err)
	}
	for _, name := range data {
		if !exist(name) {
			t.Errorf("gen over a pipeline and no scope wrote no %s", name)
		}
	}
	for _, name := range routes {
		if exist(name) {
			t.Errorf("gen over a pipeline and no scope wrote %s; a package without a scope has no routes", name)
		}
	}
	write("db/routes.volt", "package db\n\nScope /api [pipe: api] {\n  get /authors/:id(int) AuthorGet\n}\n")
	if err := volt("gen", db); err != nil {
		t.Fatalf("gen with a scope: %v", err)
	}
	for _, name := range routes {
		if !exist(name) {
			t.Fatalf("gen with a scope wrote no %s", name)
		}
	}
	if err := os.Remove(filepath.Join(db, "routes.volt")); err != nil {
		t.Fatal(err)
	}
	if err := volt("gen", db); err != nil {
		t.Fatalf("gen after the scope went: %v", err)
	}
	for _, name := range routes {
		if exist(name) {
			t.Errorf("%s outlived the package's last scope", name)
		}
	}
	for _, name := range data {
		if !exist(name) {
			t.Errorf("gen after the scope went removed %s", name)
		}
	}
	write("db/volt_router.go", "package db\n\n// RouterHandWritten is not generated (§projects-and-packages).\nfunc RouterHandWritten() {}\n")
	if err := volt("gen", db); err != nil {
		t.Fatalf("gen beside a hand-written volt_router.go: %v", err)
	}
	if !exist("volt_router.go") {
		t.Error("volt_router.go has no generated marker, so gen must leave it")
	}
}

func TestGenVerifyTypeChecks(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	volt := func(args ...string) error {
		cmd := command()
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		return cmd.Run(context.Background(), append([]string{"volt"}, args...))
	}
	dir := filepath.Join(t.TempDir(), "small")
	if err := volt("stress", "-tables", "2", "-columns", "4", "-volt", repoRoot, dir); err != nil {
		t.Fatalf("stress: %v", err)
	}
	if err := volt("gen", "--verify", dir); err != nil {
		t.Fatalf("gen --verify on a buildable project: %v", err)
	}
	broken := filepath.Join(dir, "broken.go")
	if err := os.WriteFile(broken, []byte("package main\n\nvar broken int = \"s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = volt("gen", "--verify", dir)
	if err == nil || !strings.Contains(err.Error(), "does not type-check") || !strings.Contains(err.Error(), "broken.go") {
		t.Fatalf("gen --verify beside a broken file: %v, want the type error at broken.go", err)
	}
}
