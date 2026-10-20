package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/rt"
	_ "github.com/mattn/go-sqlite3"
)

var update = flag.Bool("update", false, "rewrite golden files")

func parseChecked(t *testing.T, name, src string) (*ast.File, *check.Info) {
	t.Helper()
	f, diags := parser.ParseFile(name, src)
	info, semDiags := check.File(f)
	diags = append(diags, semDiags...)
	if diag.HasErrors(diags) {
		t.Fatalf("test input must be valid: %v", diags)
	}
	return f, info
}

func fixture(t *testing.T) (*ast.File, *check.Info) {
	t.Helper()
	path := filepath.Join("testdata", "blog.volt")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return parseChecked(t, filepath.Base(path), string(src))
}

func memoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestLowerGolden(t *testing.T) {
	_, info := fixture(t)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	got := []byte(SQLText(ms))
	golden := filepath.Join("testdata", "blog.sql.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden file (run 'go test ./nao/migrate -update'): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s:\n--- got ---\n%s", golden, got)
	}
}

func TestLowerFlags(t *testing.T) {
	_, info := fixture(t)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]rt.Migration{}
	for _, m := range ms {
		byName[m.Name] = m
	}
	cases := []struct {
		name                          string
		reversible, noTx, foreignKeys bool
	}{
		{"0001__create_posts", true, false, false},
		{"0002__comments_and_tags", true, false, true},
		{"0003__posts_created_at", false, false, true},
		{"0004__vacuum", false, true, false},
		{"0005__comments_flagged", true, false, false},
	}
	for _, tc := range cases {
		m, ok := byName[tc.name]
		if !ok {
			t.Errorf("%s not lowered", tc.name)
			continue
		}
		if (m.Down != nil) != tc.reversible || m.NoTransaction != tc.noTx || m.ForeignKeysOff != tc.foreignKeys {
			t.Errorf("%s: down %v, no transaction %v, foreign keys off %v; want %v %v %v",
				tc.name, m.Down != nil, m.NoTransaction, m.ForeignKeysOff, tc.reversible, tc.noTx, tc.foreignKeys)
		}
	}
	if len(ms) != 5 || ms[0].Name != "0001__create_posts" || ms[4].Name != "0005__comments_flagged" {
		t.Errorf("order: %v", names(ms))
	}
}

func names(ms []rt.Migration) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

// TestFoldMatchesTheSchema is the fold of §the-fold over the fixture:
// the migrations applied to one empty database, the schema's DDL to
// another, no drift between them, and down all the way back.
func TestFoldMatchesTheSchema(t *testing.T) {
	ctx := context.Background()
	f, info := fixture(t)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	foldDB := memoryDB(t)
	if _, err := rt.MigrateUp(ctx, foldDB, ms); err != nil {
		t.Fatalf("the fold stopped: %v", err)
	}
	ddl, err := sqlite.Generate(f, info, sqlite.Options{Source: "blog.volt"})
	if err != nil {
		t.Fatal(err)
	}
	targetDB := memoryDB(t)
	if _, err := targetDB.ExecContext(ctx, string(ddl)); err != nil {
		t.Fatalf("schema DDL: %v", err)
	}
	fold, err := Read(ctx, foldDB)
	if err != nil {
		t.Fatal(err)
	}
	target, err := Read(ctx, targetDB)
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(target, fold); len(drifts) > 0 {
		for _, d := range drifts {
			t.Errorf("drift: %s\n%s", d.Reason, d.Volt)
		}
	}
	if len(target.Tables) != 3 {
		t.Errorf("%d tables read, want 3", len(target.Tables))
	}
	for _, m := range ms {
		if m.Down == nil {
			break
		}
	}
	name, err := rt.MigrateDown(ctx, foldDB, ms)
	if err != nil || name != "0005__comments_flagged" {
		t.Fatalf("down: %q %v", name, err)
	}
	_, err = rt.MigrateDown(ctx, foldDB, ms)
	if err == nil || !strings.Contains(err.Error(), "0004__vacuum") {
		t.Fatalf("down over the irreversible 0004: %v", err)
	}
}

func TestReversibleFlatBodyRoundTrips(t *testing.T) {
	ctx := context.Background()
	f, info := fixture(t)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	db := memoryDB(t)
	first := ms[:2]
	if _, err := rt.MigrateUp(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	before, err := Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	for range first {
		if _, err := rt.MigrateDown(ctx, db, first); err != nil {
			t.Fatal(err)
		}
	}
	empty, err := Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Tables) != 0 {
		t.Errorf("after down of every reversible migration: %d tables left", len(empty.Tables))
	}
	if _, err := rt.MigrateUp(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	after, err := Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(before, after); len(drifts) > 0 {
		t.Errorf("down then up changed the structure: %+v", drifts)
	}
	_ = f
}

func TestLowerRefusesAForeignKeyInTheOtherTable(t *testing.T) {
	src := `package db

Table users {
  id int [pk, increment]
}

Migration 0001__init {
  create table users {
    id int [pk, increment, ref: < orders.user_id]
  }
}
`
	_, info := parseChecked(t, "t.volt", src)
	_, err := Lower(info)
	if err == nil || !strings.Contains(err.Error(), "0001__init") || !strings.Contains(err.Error(), "ref: <") {
		t.Fatalf("Lower: %v, want a refusal naming the migration and the operator", err)
	}
}

func TestLowerRebuildSources(t *testing.T) {
	src := `package db

Table users {
  id    int    [pk, increment]
  email string [not null]
  hits  int    [not null, default: 0]
  note  string
}

Migration 0001__init {
  create table users {
    id   int    [pk, increment]
    mail string [not null]
  }
}

Migration 0002__rebuild [irreversible] {
  rebuild table users {
    id    int    [pk, increment]
    email string [not null, from: ` + "`mail`" + `]
    hits  int    [not null, default: 0]
    note  string
  }
}
`
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO users__new (id, email, hits, note)\nSELECT id, mail, 0, NULL\nFROM users"
	if got := ms[1].Up[1]; got != want {
		t.Errorf("copy statement:\n%s\nwant:\n%s", got, want)
	}
	if ms[1].Down != nil {
		t.Error("an irreversible migration has a down")
	}
	if !ms[1].ForeignKeysOff {
		t.Error("a rebuild runs without the foreign-key bracket")
	}
}
