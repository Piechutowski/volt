package e2e

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/collector"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/run"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata")

var voltBuilt struct {
	sync.Mutex
	bin string
}

func voltBin(t *testing.T) string {
	t.Helper()
	voltBuilt.Lock()
	defer voltBuilt.Unlock()
	if voltBuilt.bin != "" {
		if _, err := os.Stat(voltBuilt.bin); err == nil {
			return voltBuilt.bin
		}
	}
	repo, err := collector.RepoFind(".")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "volt")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "./cmd/volt")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building volt from the checkout: %v\n%s", err, out)
	}
	voltBuilt.bin = bin
	return bin
}

type voltRun struct {
	stdout, stderr string
	code           int
}

func voltIn(t *testing.T, dir string, args ...string) voltRun {
	t.Helper()
	cmd := exec.Command(voltBin(t), args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	r := voltRun{stdout: out.String(), stderr: errOut.String()}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		r.code = exit.ExitCode()
	case err != nil:
		t.Fatalf("volt %s: %v", strings.Join(args, " "), err)
	}
	return r
}

func (r voltRun) String() string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
}

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

var streamcountRE = regexp.MustCompile(`^\S+/app/report\.go:\d+:\d+: warning: vet/streamcount: `)

// TestCLICheckVetGenRoutes runs the volt binary built from this checkout
// over the whole module as a person would: check is silent, vet reports
// the CSV export's one volt.Stream and nothing else, gen on a copy of the
// module rewrites no byte of the committed output, and routes prints the
// expanded table the golden pins (spec §projects-and-packages,
// §generation-contract-informative, §scopes-and-routes).
func TestCLICheckVetGenRoutes(t *testing.T) {
	mod := moduleDir(t)

	if r := voltIn(t, mod, "check", "./..."); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("volt check ./... is not silent:\n%s", r)
	}

	r := voltIn(t, mod, "vet", "./...")
	lines := strings.Split(strings.TrimSpace(r.stdout+r.stderr), "\n")
	if r.code != 0 || len(lines) != 1 || !streamcountRE.MatchString(lines[0]) {
		t.Errorf("volt vet ./... reports more or less than the CSV export's streamcount:\n%s", r)
	}

	copied := t.TempDir()
	if err := os.CopyFS(copied, os.DirFS(mod)); err != nil {
		t.Fatal(err)
	}
	r = voltIn(t, copied, "gen", "--sql", "./...")
	written := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if r.code != 0 || r.stderr != "" || len(written) < 12 {
		t.Fatalf("volt gen --sql ./... on a copy:\n%s", r)
	}
	for _, line := range written {
		if !strings.HasSuffix(line, " (unchanged)") {
			t.Errorf("volt gen rewrote %s", line)
		}
	}
	treesEqual(t, mod, copied)

	routes := voltIn(t, mod, "routes", "./app")
	if routes.code != 0 || routes.stderr != "" {
		t.Fatalf("volt routes ./app:\n%s", routes)
	}
	golden(t, filepath.Join("testdata", "routes.golden"), routes.stdout)
}

func treesEqual(t *testing.T, want, got string) {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(got, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(got, p)
		seen[rel] = true
		g, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		w, err := os.ReadFile(filepath.Join(want, rel))
		if err != nil {
			t.Errorf("volt gen wrote %s, which the module does not hold", rel)
			return nil
		}
		if !bytes.Equal(g, w) {
			t.Errorf("volt gen changed %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(want, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if rel, _ := filepath.Rel(want, p); !seen[rel] {
			t.Errorf("volt gen removed %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./examples/voltmeter/e2e -run TestCLI -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs; read the change and refresh with -update:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

// TestCLIMigrations runs every migration command of the volt binary built
// from this checkout against the data package and a temporary SQLite file:
// check reports the fold equal to the schema, sql prints exactly the
// statements the generated Migrations run, new writes the next empty
// migration into a copy that still folds, up applies all of them once,
// down reverts from the top and refuses the irreversible rebuild with exit
// 1, status reads the ledger at each step, and up applies the reverted
// ones again onto a database the runtime then finds current and equal to
// the schema (spec §migration-tooling, §migration-ledger, §the-fold).
func TestCLIMigrations(t *testing.T) {
	mod := moduleDir(t)
	ms := db.Migrations
	var all []string
	for _, m := range ms {
		all = append(all, m.Name)
	}
	floor := -1
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Down == nil {
			floor = i
			break
		}
	}
	if floor < 0 || floor == len(ms)-1 {
		t.Fatalf("the history needs an irreversible migration below the top, got %d of %d", floor, len(ms))
	}

	if r := voltIn(t, mod, "migration", "check", "./db"); r.code != 0 || r.stdout != fmt.Sprintf("package db: the fold of %d migration(s) matches the schema\n", len(ms)) {
		t.Errorf("volt migration check ./db:\n%s", r)
	}
	if r := voltIn(t, mod, "migration", "sql", "./db"); r.code != 0 || r.stdout != migrate.SQLText(ms) {
		t.Errorf("volt migration sql ./db does not print the statements of db.Migrations:\n%s", r)
	}

	copied := t.TempDir()
	if err := os.CopyFS(copied, os.DirFS(mod)); err != nil {
		t.Fatal(err)
	}
	next := fmt.Sprintf("%04d__retention", len(ms)+1)
	if r := voltIn(t, copied, "migration", "new", "retention", "./db"); r.code != 0 || r.stdout != filepath.Join("db", next+".volt")+"\n" {
		t.Errorf("volt migration new retention ./db:\n%s", r)
	}
	if b, err := os.ReadFile(filepath.Join(copied, "db", next+".volt")); err != nil || string(b) != "package db\n\nmigration "+next+" {\n}\n" {
		t.Errorf("the new migration file = %q, %v", b, err)
	}
	if r := voltIn(t, copied, "migration", "check", "./db"); r.code != 0 || r.stdout != fmt.Sprintf("package db: the fold of %d migration(s) matches the schema\n", len(ms)+1) {
		t.Errorf("volt migration check after new:\n%s", r)
	}

	dsn := filepath.Join(t.TempDir(), "cli.db")
	for _, cmd := range []string{"status", "down"} {
		if r := voltIn(t, mod, "migration", cmd, dsn, "./db"); r.code != 2 || !strings.Contains(r.stderr, "unable to open database file") {
			t.Errorf("volt migration %s on a database that does not exist:\n%s", cmd, r)
		}
		if _, err := os.Stat(dsn); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("volt migration %s created %s: %v", cmd, dsn, err)
		}
	}

	r := voltIn(t, mod, "migration", "up", dsn, "./db")
	if r.code != 0 || r.stdout != prefixed("applied ", all) {
		t.Fatalf("volt migration up:\n%s", r)
	}
	if r := voltIn(t, mod, "migration", "up", dsn, "./db"); r.code != 0 || r.stdout != fmt.Sprintf("nothing to apply: %d migration(s) applied\n", len(ms)) {
		t.Errorf("a second volt migration up:\n%s", r)
	}
	statusIs(t, mod, dsn, all, len(ms))

	for i := len(ms) - 1; i > floor; i-- {
		if r := voltIn(t, mod, "migration", "down", dsn, "./db"); r.code != 0 || r.stdout != "reverted "+ms[i].Name+"\n" {
			t.Fatalf("volt migration down at %s:\n%s", ms[i].Name, r)
		}
	}
	r = voltIn(t, mod, "migration", "down", dsn, "./db")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, ms[floor].Name) || !strings.Contains(r.stderr, "irreversible") {
		t.Errorf("volt migration down past the irreversible %s:\n%s", ms[floor].Name, r)
	}
	statusIs(t, mod, dsn, all, floor+1)

	if r := voltIn(t, mod, "migration", "up", dsn, "./db"); r.code != 0 || r.stdout != prefixed("applied ", all[floor+1:]) {
		t.Fatalf("volt migration up after the downs:\n%s", r)
	}
	statusIs(t, mod, dsn, all, len(ms))

	d, err := run.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if applied, err := run.MigrateUp(context.Background(), d, ms); err != nil || len(applied) != 0 {
		t.Errorf("the runtime reads the tool's ledger as current, yet applied %v, %v", applied, err)
	}
	schemaDrift(t, d, filepath.Join(mod, "db", "volt_schema.sql"))
}

func statusIs(t *testing.T, mod, dsn string, all []string, applied int) {
	t.Helper()
	r := voltIn(t, mod, "migration", "status", dsn, "./db")
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if r.code != 0 || len(lines) != len(all) {
		t.Fatalf("volt migration status:\n%s", r)
	}
	for i, line := range lines {
		f := strings.Fields(line)
		want := "pending"
		if i < applied {
			want = "applied"
		}
		if len(f) < 2 || f[0] != all[i] || f[1] != want || (want == "applied") != (len(f) == 3) {
			t.Errorf("status line %d = %q, want %s %s", i, line, all[i], want)
		}
	}
}

func prefixed(prefix string, names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(prefix + n + "\n")
	}
	return b.String()
}

func schemaDrift(t *testing.T, d run.DB, ddlPath string) {
	t.Helper()
	ctx := context.Background()
	ddl, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatal(err)
	}
	target, err := run.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.Pool().ExecContext(ctx, string(ddl)); err != nil {
		t.Fatalf("%s: %v", ddlPath, err)
	}
	want, err := migrate.Read(ctx, target.Pool())
	if err != nil {
		t.Fatal(err)
	}
	got, err := migrate.Read(ctx, d.Pool())
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range migrate.Diff(want, got) {
		t.Errorf("drift after the tool's up, down and up: %s\n%s", x.Reason, x.Volt)
	}
}
