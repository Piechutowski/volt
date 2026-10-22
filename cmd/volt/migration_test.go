package main

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// voltRun runs the CLI in process and returns what it printed and the
// error it returned; an exit code travels in the error.
func voltRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := command()
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		io.Copy(&b, r)
		done <- b.String()
	}()
	runErr := cmd.Run(context.Background(), append([]string{"volt"}, args...))
	w.Close()
	os.Stdout = old
	return <-done, runErr
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec cli.ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return -1
}

// fixtureCopy copies testdata/migrations into a fresh directory, so
// that new and gen write beside a throwaway schema.
func fixtureCopy(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "blog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join("testdata", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join("testdata", "migrations", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func schemaDrift(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "schema.volt")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(src), "  body    string\n", "  body    string\n  flagged bool   [default: false]\n", 1)
	if drifted == string(src) {
		t.Fatal("the fixture schema changed; update schemaDrift")
	}
	if err := os.WriteFile(path, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationCheck(t *testing.T) {
	dir := fixtureCopy(t)
	out, err := voltRun(t, "migration", "check", dir)
	if err != nil || !strings.Contains(out, "matches the schema") {
		t.Fatalf("check on a clean fixture: %v\n%s", err, out)
	}
	schemaDrift(t, dir)
	out, err = voltRun(t, "migration", "check", dir)
	if exitCode(err) != 1 {
		t.Fatalf("check on drift: exit %d (%v), want 1", exitCode(err), err)
	}
	for _, want := range []string{"1 difference(s)", "// column comments.flagged is in the schema but not in the fold", "alter table comments {\n  add flagged int [default: 0]\n}"} {
		if !strings.Contains(out, want) {
			t.Errorf("drift report lacks %q:\n%s", want, out)
		}
	}
	_, err = voltRun(t, "migration", "check", t.TempDir())
	if exitCode(err) != 2 {
		t.Errorf("check outside a project: exit %d (%v), want 2", exitCode(err), err)
	}
	broken := fixtureCopy(t)
	if err := os.WriteFile(filepath.Join(broken, "0003__bad.volt"), []byte("package blog\n\nMigration 0003__bad [irreversible] {\n  exec `DROP TABLE missing`\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "check", broken)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "0003__bad") || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("check over a migration SQLite refuses: exit %d (%v), want 1 with SQLite's message", exitCode(err), err)
	}
	_, err = voltRun(t, "gen", broken)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "0003__bad") || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("gen over a migration SQLite refuses: exit %d (%v), want 1 with SQLite's message", exitCode(err), err)
	}
	viewed := fixtureCopy(t)
	rebuild := "package blog\n\nMigration 0003__view [irreversible] {\n  exec `CREATE VIEW post_titles AS SELECT title FROM posts`\n  rebuild table posts {\n    id    int    [pk, increment]\n    title string\n    body  string [null]\n    views int    [index, default: 0]\n  }\n}\n"
	if err := os.WriteFile(filepath.Join(viewed, "0003__view.volt"), []byte(rebuild), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"migration", "check", viewed}, {"gen", viewed}} {
		_, err = voltRun(t, args...)
		if exitCode(err) != 1 {
			t.Errorf("%s over a rebuild under a view: exit %d (%v), want 1", args[0], exitCode(err), err)
			continue
		}
		for _, want := range []string{"0003__view", "error in view post_titles", "RENAME TO posts", viewOrTriggerHint} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s over a rebuild under a view: error %q lacks %q", args[0], err, want)
			}
		}
	}
}

// TestMigrationExitCodes pins the two exit codes of the database
// commands (spec §migration-tooling rule 1): 1 for an error in the
// migrations themselves and 2 for a database that fails. A migration
// the checker refuses, a shape body with two indexes of one name
// (§indexes), never reaches the lowering: every command refuses its
// package first, with the diagnostic and exit 1, as gen does.
func TestMigrationExitCodes(t *testing.T) {
	unlowerable := filepath.Join(t.TempDir(), "db")
	if err := os.MkdirAll(unlowerable, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":      "module unlowerable\n\ngo 1.27\n",
		"schema.volt": "package db\n\nTable posts {\n  id int [pk, increment]\n}\n\nMigration 0001__init {\n  create table posts {\n    id int [pk, increment]\n    n  int [null, unique]\n\n    Indexes {\n      n [where: `n > 0`]\n    }\n  }\n}\n",
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(unlowerable, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fresh := filepath.Join(t.TempDir(), "fresh.db")
	for _, args := range [][]string{{"migration", "sql", unlowerable}, {"migration", "check", unlowerable}, {"migration", "up", fresh, unlowerable}, {"gen", unlowerable}} {
		out, err := voltRun(t, args...)
		if exitCode(err) != 1 || !strings.Contains(out, `has two indexes named "idx_posts_n"`) {
			t.Errorf("%s over a migration with two indexes of one name, which the checker refuses before anything lowers it: exit %d (%v), want 1 naming the index\n%s", strings.Join(args[:2], " "), exitCode(err), err, out)
		}
	}

	dir := fixtureCopy(t)
	dsn := filepath.Join(t.TempDir(), "blog.db")
	_, err := voltRun(t, "migration", "down", dsn, dir)
	if exitCode(err) != 2 {
		t.Errorf("down over a missing database: exit %d (%v), want 2", exitCode(err), err)
	}
	if out, err := voltRun(t, "migration", "up", dsn, dir); err != nil {
		t.Fatalf("up: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "0003__bad.volt"), []byte("package blog\n\nMigration 0003__bad [irreversible] {\n  exec `DROP TABLE missing`\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "up", dsn, dir)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "0003__bad") || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("up over a migration SQLite refuses: exit %d (%v), want 1 with SQLite's message", exitCode(err), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0003__bad.volt"), []byte("package blog\n\nMigration 0003__bump [irreversible] {\n  exec `PRAGMA user_version = 3`\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := voltRun(t, "migration", "up", dsn, dir); err != nil || out != "applied 0003__bump\n" {
		t.Fatalf("up over the irreversible migration: %v\n%s", err, out)
	}
	_, err = voltRun(t, "migration", "down", dsn, dir)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "0003__bump") || !strings.Contains(err.Error(), "irreversible") {
		t.Errorf("down over an irreversible migration: exit %d (%v), want 1 naming it", exitCode(err), err)
	}

	broken := filepath.Join(t.TempDir(), "broken.db")
	ledgerWrite(t, broken, "CREATE TABLE volt_migrations (name TEXT PRIMARY KEY)")
	for _, sub := range []string{"status", "up", "down"} {
		_, err := voltRun(t, "migration", sub, broken, dir)
		if exitCode(err) != 2 || !strings.Contains(err.Error(), "applied_at") {
			t.Errorf("%s over a database whose ledger cannot be read: exit %d (%v), want 2 with SQLite's message", sub, exitCode(err), err)
		}
	}
}

func TestMigrationCheckRunsEveryDown(t *testing.T) {
	dir := fixtureCopy(t)
	src := "package blog\n\nMigration 0003__bad_down {\n  up {\n    exec `CREATE TABLE scratch (id INTEGER)`\n    exec `DROP TABLE scratch`\n  }\n  down {\n    exec `DROP TABLE missing`\n  }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "0003__bad_down.volt"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := voltRun(t, "migration", "check", dir)
	if exitCode(err) != 1 {
		t.Fatalf("check over a down block SQLite refuses: exit %d (%v), want 1", exitCode(err), err)
	}
	for _, want := range []string{"on the way down", "0003__bad_down", "no such table: missing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	_, err = voltRun(t, "gen", dir)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "on the way down") {
		t.Errorf("gen over a down block SQLite refuses: exit %d (%v), want 1", exitCode(err), err)
	}
}

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub")
}

func TestSQLiteGuard(t *testing.T) {
	if err := sqliteGuard(sqliteDriver); err != nil {
		t.Fatalf("the guard refuses a working driver: %v", err)
	}
	sql.Register("sqlite3_stub", stubDriver{})
	err := sqliteGuard("sqlite3_stub")
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "cgo") || !strings.Contains(err.Error(), "C compiler") {
		t.Fatalf("the guard over the stub: exit %d (%v), want 2 naming cgo and a C compiler", exitCode(err), err)
	}
}

func TestMigrationSQLGolden(t *testing.T) {
	out, err := voltRun(t, "migration", "sql", filepath.Join("testdata", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "migrations", "sql.golden")
	if *updateLSP {
		if err := os.WriteFile(golden, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden (run 'go test ./cmd/volt -run TestMigrationSQLGolden -update'): %v", err)
	}
	if out != string(want) {
		t.Errorf("sql output differs from %s:\n--- got ---\n%s", golden, out)
	}
}

func TestMigrationNew(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":      "module newfixture\n\ngo 1.27\n",
		"schema.volt": "package db\n\nTable posts {\n  id int [pk, increment]\n}\n",
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := voltRun(t, "migration", "new", "init", dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	first := filepath.Join(dir, "0001__init.volt")
	if strings.TrimSpace(out) != first {
		t.Errorf("printed %q, want %s", out, first)
	}
	src, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != "package db\n\nMigration 0001__init {\n}\n" {
		t.Errorf("0001__init.volt:\n%s", src)
	}
	if _, err := voltRun(t, "migration", "new", "add_views", dir); err != nil {
		t.Fatalf("second new: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "0002__add_views.volt")); err != nil {
		t.Error("second new did not mint 0002")
	}
	if err := os.WriteFile(filepath.Join(dir, "0003__again.volt"), []byte("package db\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "new", "again", dir)
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "refusing") {
		t.Errorf("new over an existing file: %v, want a refusal", err)
	}
	wider := map[string]string{
		"0001__init.volt":      "package db\n\nMigration 00001__init {\n}\n",
		"0002__add_views.volt": "package db\n\nMigration 00010__wide {\n}\n",
	}
	for name, src := range wider {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err = voltRun(t, "migration", "new", "next", dir)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), "00011__next.volt") {
		t.Errorf("new after a wider run: %q %v, want 00011__next.volt", out, err)
	}
	out, err = voltRun(t, "migration", "new", "été_2026", dir)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), "00012__été_2026.volt") {
		t.Errorf("new with a Unicode label: %q %v, want 00012__été_2026.volt", out, err)
	}
	for _, label := range []string{"bad-label", "a.b", "", "two words"} {
		_, err = voltRun(t, "migration", "new", label, dir)
		if exitCode(err) != 2 {
			t.Errorf("new with label %q: exit %d (%v), want 2", label, exitCode(err), err)
		}
	}
	full := filepath.Join(t.TempDir(), "full")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "go.mod"), []byte("module full\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "schema.volt"), []byte("package db\n\nTable posts {\n  id int [pk, increment]\n}\n\nMigration 9999__last {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "new", "overflow", full)
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "10000") || !strings.Contains(err.Error(), "widen every migration name to 5 digits") {
		t.Errorf("new past the run's width: exit %d (%v), want 2 telling the user to widen every name", exitCode(err), err)
	}
	if entries, _ := os.ReadDir(full); len(entries) != 2 {
		t.Errorf("new past the run's width wrote a file: %v", entries)
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "go.mod"), []byte("module empty\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "api.volt"), []byte("package api\n\nType kinds.Color string from \"example.com/kinds\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "new", "init", empty)
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "no tables") {
		t.Errorf("new in a package without tables: %v, want a refusal", err)
	}
}

func TestMigrationUpStatusDown(t *testing.T) {
	dir := fixtureCopy(t)
	dsn := filepath.Join(t.TempDir(), "blog.db")
	for _, sub := range []string{"status", "down"} {
		_, err := voltRun(t, "migration", sub, dsn, dir)
		if exitCode(err) != 2 || !strings.Contains(err.Error(), dsn+": unable to open database file") {
			t.Errorf("%s over a missing database: exit %d (%v), want 2 reporting the open", sub, exitCode(err), err)
		}
		if _, err := os.Stat(dsn); err == nil {
			t.Fatalf("%s created the missing database", sub)
		}
	}
	out, err := voltRun(t, "migration", "up", dsn, dir)
	if err != nil || out != "applied 0001__create_posts\napplied 0002__comments\n" {
		t.Fatalf("up: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "up", dsn, dir)
	if err != nil || !strings.Contains(out, "nothing to apply") {
		t.Fatalf("second up: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "status", dsn, dir)
	if err != nil || strings.Count(out, "applied 20") != 2 {
		t.Fatalf("status after up: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "down", dsn, dir)
	if err != nil || out != "reverted 0002__comments\n" {
		t.Fatalf("down: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "status", dsn, dir)
	if err != nil || !strings.Contains(out, "0002__comments      pending") || !strings.Contains(out, "0001__create_posts  applied") {
		t.Fatalf("status after down: %v\n%s", err, out)
	}
	ledgerWrite(t, dsn, "INSERT INTO volt_migrations (name, applied_at) VALUES ('0005__elsewhere', '2026-01-01T00:00:00Z')")
	out, err = voltRun(t, "migration", "status", dsn, dir)
	if err != nil || !strings.Contains(out, "0005__elsewhere     applied 2026-01-01T00:00:00Z (not in the package)") {
		t.Fatalf("status with a ledger row no migration names: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "status", "file:"+dsn, dir)
	if err != nil || !strings.Contains(out, "0005__elsewhere") {
		t.Fatalf("status over a file: URI: %v\n%s", err, out)
	}
	_, err = voltRun(t, "migration", "up", dir)
	if exitCode(err) != 2 {
		t.Errorf("up without a DSN: exit %d (%v), want 2", exitCode(err), err)
	}
	for _, sub := range []string{"up", "down", "status"} {
		for _, memory := range []string{":memory:", "file::memory:?cache=shared", "file:x.db?mode=memory"} {
			_, err = voltRun(t, "migration", sub, memory, dir)
			if exitCode(err) != 2 || !strings.Contains(err.Error(), "holds nothing") {
				t.Errorf("%s over %s: exit %d (%v), want 2", sub, memory, exitCode(err), err)
			}
		}
	}
	_, err = voltRun(t, "migration", "up", filepath.Join(t.TempDir(), "absent", "blog.db"), dir)
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "unable to open database file") {
		t.Errorf("up over a database that cannot be opened: exit %d (%v), want 2", exitCode(err), err)
	}
}

func TestMigrationStatusLeavesTheDatabaseAlone(t *testing.T) {
	dir := fixtureCopy(t)
	dsn := filepath.Join(t.TempDir(), "plain.db")
	ledgerWrite(t, dsn, "CREATE TABLE unrelated (id INTEGER)")
	out, err := voltRun(t, "migration", "status", dsn, dir)
	if err != nil || strings.Count(out, "pending") != 2 {
		t.Fatalf("status over a database without the ledger: %v\n%s", err, out)
	}
	db, err := sql.Open(sqliteDriver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("journal_mode %q %v after status, want delete: a read-only question changed the database", mode, err)
	}
	var tables int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&tables); err != nil || tables != 1 {
		t.Errorf("%d tables %v after status, want the one unrelated table: status created the ledger", tables, err)
	}
}

func ledgerWrite(t *testing.T, dsn, stmt string) {
	t.Helper()
	db, err := sql.Open(sqliteDriver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatal(err)
	}
}

func TestDSNWithMode(t *testing.T) {
	cases := map[string]string{
		"blog.db":                        "file:blog.db?mode=ro",
		"/var/lib/app/blog.db":           "file:/var/lib/app/blog.db?mode=ro",
		"odd name#1.db":                  "file:odd%20name%231.db?mode=ro",
		"file:blog.db":                   "file:blog.db?mode=ro",
		"file:blog.db?_txlock=immediate": "file:blog.db?_txlock=immediate&mode=ro",
	}
	for dsn, want := range cases {
		if got := dsnWithMode(dsn, "ro"); got != want {
			t.Errorf("dsnWithMode(%q) = %q, want %q", dsn, got, want)
		}
	}
}

func TestGenWritesMigrationsAndRefusesDrift(t *testing.T) {
	dir := fixtureCopy(t)
	out, err := voltRun(t, "gen", dir)
	if err != nil {
		t.Fatalf("gen: %v\n%s", err, out)
	}
	generated := filepath.Join(dir, "volt_migrations.go")
	src, err := os.ReadFile(generated)
	if err != nil {
		t.Fatalf("gen did not write volt_migrations.go: %v\n%s", err, out)
	}
	for _, want := range []string{"// Code generated by nao gen from package .; DO NOT EDIT.", "var Migrations = []rt.Migration{", `Name: "0002__comments"`, "func Migrate(ctx context.Context, db *sql.DB) error"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("volt_migrations.go lacks %q", want)
		}
	}
	out, err = voltRun(t, "gen", "-parts", "models", dir)
	if err != nil || strings.Contains(out, "volt_migrations.go") {
		t.Errorf("gen -parts models: %v\n%s", err, out)
	}
	schemaDrift(t, dir)
	out, err = voltRun(t, "gen", dir)
	if exitCode(err) != 1 || !strings.Contains(err.Error(), "volt migration check") || !strings.Contains(out, "add flagged int") {
		t.Fatalf("gen on drift: exit %d (%v), want 1 with the report\n%s", exitCode(err), err, out)
	}
	for _, name := range []string{"0001__create_posts.volt", "0002__comments.volt"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	out, err = voltRun(t, "gen", dir)
	if err != nil || !strings.Contains(out, "volt_migrations.go (removed: no longer generated)") {
		t.Fatalf("gen without migrations: %v\n%s", err, out)
	}
	if _, err := os.Stat(generated); err == nil {
		t.Error("a package without migrations keeps a stale volt_migrations.go")
	}
}

func TestItestMigrationsFold(t *testing.T) {
	out, err := voltRun(t, "migration", "check", filepath.Join("..", "..", "nao", "itest"))
	if err != nil {
		t.Fatalf("nao/itest drifts from its migrations: %v\n%s", err, out)
	}
}
