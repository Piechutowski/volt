package main

import (
	"bytes"
	"context"
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
	drifted := strings.Replace(string(src), "  body    string [not null]\n", "  body    string [not null]\n  flagged bool   [not null, default: false]\n", 1)
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
	for _, want := range []string{"1 difference(s)", "// column comments.flagged is in the schema but not in the fold", "alter table comments {\n  add flagged int [not null, default: 0]\n}"} {
		if !strings.Contains(out, want) {
			t.Errorf("drift report lacks %q:\n%s", want, out)
		}
	}
	_, err = voltRun(t, "migration", "check", t.TempDir())
	if exitCode(err) != 2 {
		t.Errorf("check outside a project: exit %d (%v), want 2", exitCode(err), err)
	}
	broken := fixtureCopy(t)
	if err := os.WriteFile(filepath.Join(broken, "0003__bad.volt"), []byte("package blog\n\nMigration 0003__bad [irreversible] {\n  exec 'DROP TABLE missing'\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = voltRun(t, "migration", "check", broken)
	if exitCode(err) != 2 || !strings.Contains(err.Error(), "0003__bad") || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("check over a migration SQLite refuses: exit %d (%v), want 2 with SQLite's message", exitCode(err), err)
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
	if err := os.WriteFile(filepath.Join(dir, "0002__add_views.volt"), []byte("package db\n\nMigration 00010__wide {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = voltRun(t, "migration", "new", "next", dir)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), "00011__next.volt") {
		t.Errorf("new after a wider run: %q %v, want 00011__next.volt", out, err)
	}
	_, err = voltRun(t, "migration", "new", "bad-label", dir)
	if exitCode(err) != 2 {
		t.Errorf("new with a bad label: exit %d (%v), want 2", exitCode(err), err)
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "go.mod"), []byte("module empty\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "api.volt"), []byte("package api\n\nEnum color {\n  red\n}\n"), 0o644); err != nil {
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
	out, err := voltRun(t, "migration", "status", dsn, dir)
	if err != nil || !strings.Contains(out, "0001__create_posts  pending") || !strings.Contains(out, "0002__comments      pending") {
		t.Fatalf("status before up: %v\n%s", err, out)
	}
	out, err = voltRun(t, "migration", "up", dsn, dir)
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
	_, err = voltRun(t, "migration", "up", dir)
	if exitCode(err) != 2 {
		t.Errorf("up without a DSN: exit %d (%v), want 2", exitCode(err), err)
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
