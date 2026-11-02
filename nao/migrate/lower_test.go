package migrate

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/Piechutowski/volt/nao/run"
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

func memoryDB(t *testing.T) run.DB {
	t.Helper()
	db, err := run.Open("sqlite3", ":memory:")
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
		{"0006__nicknames_and_details", true, false, false},
	}
	for _, tc := range cases {
		m, ok := byName[tc.name]
		if !ok {
			t.Errorf("%s not lowered", tc.name)
			continue
		}
		if (m.Down != nil) != tc.reversible || m.NoTransaction != tc.noTx || m.ForeignKeysOff != tc.foreignKeys {
			t.Errorf("%s: down %v, no_transaction %v, foreign keys off %v; want %v %v %v",
				tc.name, m.Down != nil, m.NoTransaction, m.ForeignKeysOff, tc.reversible, tc.noTx, tc.foreignKeys)
		}
	}
	if len(ms) != 6 || ms[0].Name != "0001__create_posts" || ms[5].Name != "0006__nicknames_and_details" {
		t.Errorf("order: %v", names(ms))
	}
}

func TestLowerNoTransactionDisablesTheBracket(t *testing.T) {
	const body = "\n  create table posts {\n    id int [pk]\n  }\n  drop table scratch\n}\n"
	for _, tc := range []struct {
		settings          string
		noTx, foreignKeys bool
	}{
		{"[irreversible]", false, true},
		{"[irreversible, no_transaction]", true, false},
	} {
		_, info := parseChecked(t, "t.volt", "package db\n\ntable posts {\n  id int [pk]\n}\n\nmigration 0001__init "+tc.settings+" {"+body)
		ms, err := Lower(info)
		if err != nil {
			t.Fatal(err)
		}
		if ms[0].NoTransaction != tc.noTx || ms[0].ForeignKeysOff != tc.foreignKeys {
			t.Errorf("%s: no_transaction %v, foreign keys off %v; want %v %v", tc.settings, ms[0].NoTransaction, ms[0].ForeignKeysOff, tc.noTx, tc.foreignKeys)
		}
	}
}

// TestLowerRefusesTwoIndexesOfOneName pins that two indexes of a shape
// body that derive one name, a column's unique and a partial index on
// it, are the checker's error (spec §indexes, §migrations) and,
// should an unchecked package reach it, the lowering's refusal naming
// the migration.
func TestLowerRefusesTwoIndexesOfOneName(t *testing.T) {
	src := "package db\n\ntable posts {\n  id int [pk]\n}\n\nmigration 0001__init {\n  create table posts {\n    id int [pk]\n    n  int [null, unique]\n\n    indexes {\n      n [where: `n > 0`]\n    }\n  }\n}\n"
	f, diags := parser.ParseFile("t.volt", src)
	if diag.HasErrors(diags) {
		t.Fatalf("test input must parse: %v", diags)
	}
	info, semDiags := check.File(f)
	found := false
	for _, d := range semDiags {
		if strings.Contains(d.Msg, `has two indexes named "idx_posts_n"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("check: %v, want the two indexes of one name reported", semDiags)
	}
	_, err := Lower(info)
	if err == nil || !strings.Contains(err.Error(), "0001__init") || !strings.Contains(err.Error(), `duplicate index name "idx_posts_n"`) {
		t.Fatalf("Lower: %v, want a refusal naming the migration and the index name", err)
	}
}

func TestRebuildForgetsAnIndexNotRestated(t *testing.T) {
	ctx := context.Background()
	src := "package db\n\ntable posts {\n  id int [pk]\n  n  int [null]\n}\n\nmigration 0001__init {\n  create table posts {\n    id int [pk]\n    n  int [null, index]\n  }\n}\n\nmigration 0002__rebuild [irreversible] {\n  rebuild table posts {\n    id int [pk]\n    n  int [null]\n  }\n}\n"
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms[1].Up) != 4 {
		t.Errorf("the rebuild has %d statements, want the new table, the copy, the drop and the rename: 4\n%s", len(ms[1].Up), strings.Join(ms[1].Up, ";\n"))
	}
	db := memoryDB(t)
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatal(err)
	}
	s, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if ix := s.Table("posts").Indexes; len(ix) != 0 {
		t.Errorf("after the rebuild posts has indexes %+v, want none", ix)
	}
}

// TestTypedDropOfAnIndexedColumnRoundTrips pins that a typed drop whose
// shape carries unique or index drops that index before the column, as
// the inverse of an add does, so the flat body folds to the schema and
// runs down and up again on a real SQLite (spec §migrations).
func TestTypedDropOfAnIndexedColumnRoundTrips(t *testing.T) {
	ctx := context.Background()
	src := "package db\n\ntable accounts {\n  id int [pk]\n}\n\nmigration 0001__init {\n  create table accounts {\n    id     int    [pk]\n    email  string [null, unique]\n    region string [null, index]\n  }\n}\n\nmigration 0002__drop {\n  alter table accounts {\n    drop email string [null, unique]\n    drop region string [null, index]\n  }\n}\n"
	f, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	wantUp := []string{
		"DROP INDEX idx_accounts_email",
		"ALTER TABLE accounts DROP COLUMN email",
		"DROP INDEX idx_accounts_region",
		"ALTER TABLE accounts DROP COLUMN region",
	}
	if got := strings.Join(ms[1].Up, ";\n"); got != strings.Join(wantUp, ";\n") {
		t.Errorf("up of 0002:\n%s\nwant:\n%s", got, strings.Join(wantUp, ";\n"))
	}
	db := memoryDB(t)
	if _, err := run.MigrateUp(ctx, db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	before, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatalf("the fold stopped: %v", err)
	}
	ddl, err := sqlite.Generate(f, info, sqlite.Options{Source: "t.volt"})
	if err != nil {
		t.Fatal(err)
	}
	targetDB := memoryDB(t)
	if _, err := targetDB.Pool().ExecContext(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	target, err := Read(ctx, targetDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	fold, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(target, fold); len(drifts) > 0 {
		t.Errorf("the fold drifts from the schema: %+v", drifts)
	}
	if _, err := run.MigrateDown(ctx, db, ms); err != nil {
		t.Fatalf("down of 0002: %v", err)
	}
	after, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(before, after); len(drifts) > 0 {
		t.Errorf("down of 0002 left another structure than 0001 made: %+v", drifts)
	}
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// TestRebuildUnderAViewOrTriggerFailsAtTheRename pins where the five
// steps stop when a view, or a trigger on another table, reads the
// table (spec §migrations): SQLite refuses the rename and names
// the view or trigger, which exec (rule 11) drops and recreates around
// the rebuild.
func TestRebuildUnderAViewOrTriggerFailsAtTheRename(t *testing.T) {
	ctx := context.Background()
	const schema = "package db\n\ntable posts {\n  id    int    [pk]\n  title string\n}\n\ntable comments {\n  id      int [pk]\n  post_id int\n}\n\n"
	const tables = "  create table posts {\n    id    int [pk]\n    title string [null]\n  }\n  create table comments {\n    id      int [pk]\n    post_id int\n  }\n"
	const rebuild = "migration 0002__rebuild [irreversible] {\n  rebuild table posts {\n    id    int    [pk]\n    title string [default: \"\"]\n  }\n}\n"
	cases := []struct{ object, exec string }{
		{"view post_titles", "exec `CREATE VIEW post_titles AS SELECT title FROM posts` [down: `DROP VIEW post_titles`]"},
		{"trigger comments_touch_posts", "exec `CREATE TRIGGER comments_touch_posts AFTER INSERT ON comments BEGIN UPDATE posts SET title = title WHERE id = NEW.post_id; END` [down: `DROP TRIGGER comments_touch_posts`]"},
	}
	for _, tc := range cases {
		t.Run(tc.object, func(t *testing.T) {
			_, info := parseChecked(t, "t.volt", schema+"migration 0001__init {\n"+tables+"  "+tc.exec+"\n}\n\n"+rebuild)
			ms, err := Lower(info)
			if err != nil {
				t.Fatal(err)
			}
			db := memoryDB(t)
			applied, err := run.MigrateUp(ctx, db, ms)
			if err == nil {
				t.Fatalf("a rebuild under a %s ran through", tc.object)
			}
			for _, want := range []string{"0002__rebuild", "statement 4", tc.object, "RENAME TO posts"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			if len(applied) != 1 {
				t.Errorf("applied %v, want 0001__init alone", applied)
			}
		})
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
	if _, err := run.MigrateUp(ctx, foldDB, ms); err != nil {
		t.Fatalf("the fold stopped: %v", err)
	}
	ddl, err := sqlite.Generate(f, info, sqlite.Options{Source: "blog.volt"})
	if err != nil {
		t.Fatal(err)
	}
	targetDB := memoryDB(t)
	if _, err := targetDB.Pool().ExecContext(ctx, string(ddl)); err != nil {
		t.Fatalf("schema DDL: %v", err)
	}
	fold, err := Read(ctx, foldDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	target, err := Read(ctx, targetDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(target, fold); len(drifts) > 0 {
		for _, d := range drifts {
			t.Errorf("drift: %s\n%s", d.Reason, d.Volt)
		}
	}
	if len(target.Tables) != 4 {
		t.Errorf("%d tables read, want 4", len(target.Tables))
	}
	for _, want := range []string{"0006__nicknames_and_details", "0005__comments_flagged"} {
		name, err := run.MigrateDown(ctx, foldDB, ms)
		if err != nil || name != want {
			t.Fatalf("down: %q %v, want %s", name, err, want)
		}
	}
	_, err = run.MigrateDown(ctx, foldDB, ms)
	if err == nil || !strings.Contains(err.Error(), "0004__vacuum") {
		t.Fatalf("down over the irreversible 0004: %v", err)
	}
}

// foldDiff is the fold of §the-fold over one source: its migrations
// applied to one empty database, its schema's DDL to another, and the
// drift between them.
func foldDiff(t *testing.T, name, src string) []Drift {
	t.Helper()
	ctx := context.Background()
	f, info := parseChecked(t, name, src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	foldDB := memoryDB(t)
	if _, err := run.MigrateUp(ctx, foldDB, ms); err != nil {
		t.Fatalf("the fold stopped: %v", err)
	}
	ddl, err := sqlite.Generate(f, info, sqlite.Options{Source: name})
	if err != nil {
		t.Fatal(err)
	}
	targetDB := memoryDB(t)
	if _, err := targetDB.Pool().ExecContext(ctx, string(ddl)); err != nil {
		t.Fatalf("schema DDL: %v", err)
	}
	fold, err := Read(ctx, foldDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	target, err := Read(ctx, targetDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	return Diff(target, fold)
}

// TestFoldAutoincrement folds the conformance snippet whose migrations
// create and rebuild tables with and without autoincrement (spec
// §the-fold, §migrations): it agrees with its schema, and the same
// migrations without the rebuild's autoincrement drift, closed by a
// rebuild.
func TestFoldAutoincrement(t *testing.T) {
	path := filepath.Join("..", "..", "lang", "conformance", "snippets", "valid", "v105_migration_autoincrement.volt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, d := range foldDiff(t, "v105.volt", src) {
		t.Errorf("drift: %s\n%s", d.Reason, d.Volt)
	}
	const rebuilt = "  rebuild table events {\n    id   int    [pk, autoincrement]\n"
	if !strings.Contains(src, rebuilt) {
		t.Fatalf("the snippet changed; update TestFoldAutoincrement")
	}
	drifts := foldDiff(t, "v105.volt", strings.Replace(src, rebuilt, "  rebuild table events {\n    id   int    [pk]\n", 1))
	if len(drifts) != 1 || drifts[0].Table != "events" || !strings.HasPrefix(drifts[0].Volt, "rebuild table events {\n  id int [pk, autoincrement]\n") ||
		!strings.Contains(drifts[0].Reason, "column events.id is AUTOINCREMENT in the schema, without AUTOINCREMENT in the fold") {
		t.Errorf("drifts %+v, want the rebuild of events that adds autoincrement", drifts)
	}
}

// TestRebuildKeepsTheHighWaterMark runs rebuilds of an autoincrement
// table on SQLite (spec §migrations): one that keeps
// autoincrement carries sqlite_sequence over, so the id of a deleted
// row, the largest included, is never assigned again, and one that
// adds it starts the mark at the largest copied id, since a table
// without it keeps none.
func TestRebuildKeepsTheHighWaterMark(t *testing.T) {
	ctx := context.Background()
	src := "package db\n\ntable events {\n  id   int    [pk, autoincrement]\n  name string\n  note string [null]\n}\n\ntable points {\n  id    int     [pk, autoincrement]\n  value float64\n}\n\nmigration 0001__init {\n  create table events {\n    id   int    [pk, autoincrement]\n    name string\n  }\n  create table points {\n    id    int     [pk]\n    value float64\n  }\n}\n\nmigration 0002__rebuild [irreversible] {\n  rebuild table events {\n    id   int    [pk, autoincrement]\n    name string\n    note string [null]\n  }\n  rebuild table points {\n    id    int     [pk, autoincrement]\n    value float64\n  }\n}\n"
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	db := memoryDB(t)
	if _, err := run.MigrateUp(ctx, db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	pool := db.Pool()
	for _, stmt := range []string{
		"INSERT INTO events (name) VALUES ('a'), ('b'), ('c')",
		"DELETE FROM events WHERE id = 3",
		"INSERT INTO points (value) VALUES (1), (2), (3)",
		"DELETE FROM points WHERE id = 3",
	} {
		if _, err := pool.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatal(err)
	}
	if drifts := foldDiff(t, "t.volt", src); len(drifts) > 0 {
		t.Errorf("drift: %+v", drifts)
	}
	for _, c := range []struct {
		insert string
		table  string
		want   int64
	}{
		{"INSERT INTO events (name) VALUES ('d')", "events", 4},
		{"INSERT INTO points (value) VALUES (4)", "points", 3},
	} {
		res, err := pool.ExecContext(ctx, c.insert)
		if err != nil {
			t.Fatal(err)
		}
		if id, err := res.LastInsertId(); err != nil || id != c.want {
			t.Errorf("%s after the rebuild got id %d (%v), want %d", c.table, id, err, c.want)
		}
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
	if _, err := run.MigrateUp(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	before, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	for range first {
		if _, err := run.MigrateDown(ctx, db, first); err != nil {
			t.Fatal(err)
		}
	}
	empty, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Tables) != 0 {
		t.Errorf("after down of every reversible migration: %d tables left", len(empty.Tables))
	}
	if _, err := run.MigrateUp(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	after, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(before, after); len(drifts) > 0 {
		t.Errorf("down then up changed the structure: %+v", drifts)
	}
	_ = f
}

func TestLowerRebuildSources(t *testing.T) {
	src := `package db

table users {
  id    int    [pk]
  email string
  hits  int    [default: 0]
  note  string [null]
}

migration 0001__init {
  create table users {
    id   int    [pk]
    mail string
  }
}

migration 0002__rebuild [irreversible] {
  rebuild table users {
    id    int    [pk]
    email string [from: ` + "`mail`" + `]
    hits  int    [default: 0]
    note  string [null]
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

// TestLowerRebuildSourcesOfAnUnknownTable pins the source of every
// column of a rebuild of a table no create table made: the old column
// of its name, since the lowering has no fold to consult.
func TestLowerRebuildSourcesOfAnUnknownTable(t *testing.T) {
	src := `package db

table users {
  id   int    [pk]
  hits int    [default: 0]
  note string [null]
}

migration 0001__rebuild [irreversible] {
  rebuild table users {
    id   int    [pk]
    hits int    [default: 0]
    note string [null]
  }
}
`
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO users__new (id, hits, note)\nSELECT id, hits, note\nFROM users"
	if got := ms[0].Up[1]; got != want {
		t.Errorf("copy statement:\n%s\nwant:\n%s", got, want)
	}
}

// TestLowerRebuildIntroducingARowidAlias pins NULL as the source of an
// int [pk] column the old table lacks, the one NOT NULL
// column the checker admits without a source (spec §migrations).
func TestLowerRebuildIntroducingARowidAlias(t *testing.T) {
	src := "package db\n\ntable posts {\n  id int [pk]\n  n  int [null]\n}\n\nmigration 0001__init {\n  create table posts {\n    n int [null]\n  }\n}\n\nmigration 0002__rebuild [irreversible] {\n  rebuild table posts {\n    id int [pk]\n    n  int [null]\n  }\n}\n"
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO posts__new (id, n)\nSELECT NULL, n\nFROM posts"
	if got := ms[1].Up[1]; got != want {
		t.Errorf("copy statement:\n%s\nwant:\n%s", got, want)
	}
	db := memoryDB(t)
	if _, err := run.MigrateUp(context.Background(), db, ms); err != nil {
		t.Fatal(err)
	}
}

// TestExpressionDefaultAddFillsRowsWithoutARebuild pins the add of a
// column whose default is not a constant (spec §migrations): the
// five statements, every existing row filled once, the definition the
// schema declares, read at once by a connection that loaded the old
// schema, no drift from the target, and down and up again.
func TestExpressionDefaultAddFillsRowsWithoutARebuild(t *testing.T) {
	ctx := context.Background()
	src := "package db\n\ntable posts {\n  id         int       [pk]\n  title      string\n  created_at time.Time [default: `CURRENT_TIMESTAMP`]\n  stamp      int       [null, index, default: `strftime('%s', 'now')`]\n}\n\nmigration 0001__init {\n  create table posts {\n    id    int    [pk]\n    title string\n  }\n}\n\nmigration 0002__stamps {\n  alter table posts {\n    add created_at time.Time [default: `CURRENT_TIMESTAMP`]\n    add stamp int [null, index, default: `strftime('%s', 'now')`]\n  }\n}\n"
	f, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	wantUp := []string{
		"ALTER TABLE posts ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT 0",
		"UPDATE posts SET created_at = (CURRENT_TIMESTAMP)",
		"PRAGMA writable_schema = ON",
		"UPDATE sqlite_schema SET sql = replace(sql, ', created_at TIMESTAMP NOT NULL DEFAULT 0', ', created_at TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP)')\nWHERE type = 'table' AND name = 'posts' COLLATE NOCASE",
		"PRAGMA writable_schema = RESET",
		"ALTER TABLE posts ADD COLUMN stamp INTEGER",
		"UPDATE posts SET stamp = (strftime('%s', 'now'))",
		"PRAGMA writable_schema = ON",
		"UPDATE sqlite_schema SET sql = replace(sql, ', stamp INTEGER', ', stamp INTEGER DEFAULT (strftime(''%s'', ''now''))')\nWHERE type = 'table' AND name = 'posts' COLLATE NOCASE",
		"PRAGMA writable_schema = RESET",
		"CREATE INDEX idx_posts_stamp ON posts (stamp)",
	}
	if got := strings.Join(ms[1].Up, ";\n"); got != strings.Join(wantUp, ";\n") {
		t.Errorf("up of 0002:\n%s\nwant:\n%s", got, strings.Join(wantUp, ";\n"))
	}
	if ms[1].ForeignKeysOff {
		t.Error("an add takes no foreign-key bracket")
	}

	path := filepath.Join(t.TempDir(), "posts.db")
	db, err := run.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	other, err := run.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := run.MigrateUp(ctx, db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	before, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().ExecContext(ctx, "INSERT INTO posts (title) VALUES ('one'), ('two')"); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Pool().ExecContext(ctx, "INSERT INTO posts (title) VALUES ('three')"); err != nil {
		t.Fatal(err)
	}
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatalf("0002 on a table with rows: %v", err)
	}

	var filled string
	if err := db.Pool().QueryRowContext(ctx, "SELECT group_concat(typeof(created_at) || '/' || typeof(stamp)) FROM posts").Scan(&filled); err != nil {
		t.Fatal(err)
	}
	if filled != "text/integer,text/integer,text/integer" {
		t.Errorf("existing rows hold %q, want every row filled", filled)
	}
	for _, conn := range []run.DB{db, other} {
		if _, err := conn.Pool().ExecContext(ctx, "INSERT INTO posts (title) VALUES ('later')"); err != nil {
			t.Fatalf("insert after 0002: %v", err)
		}
		if _, err := conn.Pool().ExecContext(ctx, "INSERT INTO posts (title, created_at) VALUES ('null', NULL)"); err == nil || !strings.Contains(err.Error(), "NOT NULL") {
			t.Errorf("explicit NULL after 0002: %v, want NOT NULL refused", err)
		}
	}
	var nulls int
	if err := other.Pool().QueryRowContext(ctx, "SELECT count(*) FROM posts WHERE created_at IS NULL OR stamp IS NULL").Scan(&nulls); err != nil {
		t.Fatal(err)
	}
	if nulls != 0 {
		t.Errorf("%d rows without a default after 0002", nulls)
	}
	var integrity string
	if err := db.Pool().QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Errorf("integrity_check: %q %v", integrity, err)
	}

	ddl, err := sqlite.Generate(f, info, sqlite.Options{Source: "t.volt"})
	if err != nil {
		t.Fatal(err)
	}
	targetDB := memoryDB(t)
	if _, err := targetDB.Pool().ExecContext(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	target, err := Read(ctx, targetDB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	fold, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(target, fold); len(drifts) > 0 {
		t.Errorf("0002 drifts from the schema: %+v", drifts)
	}
	if _, err := run.MigrateDown(ctx, db, ms); err != nil {
		t.Fatalf("down of 0002: %v", err)
	}
	after, err := Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := Diff(before, after); len(drifts) > 0 {
		t.Errorf("down of 0002 left another structure than 0001 made: %+v", drifts)
	}
	if _, err := run.MigrateUp(ctx, db, ms); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// TestExpressionDefaultAddYieldingNullRollsBack pins the one failure
// only rows reveal (spec §the-fold): a NOT NULL column whose
// expression default yields NULL fails at the backfill, and the
// migration leaves the table as it was.
func TestExpressionDefaultAddYieldingNullRollsBack(t *testing.T) {
	ctx := context.Background()
	src := "package db\n\ntable posts {\n  id   int [pk]\n  seen int [default: `nullif(1, 1)`]\n}\n\nmigration 0001__init {\n  create table posts {\n    id int [pk]\n  }\n}\n\nmigration 0002__seen {\n  alter table posts {\n    add seen int [default: `nullif(1, 1)`]\n  }\n}\n"
	_, info := parseChecked(t, "t.volt", src)
	ms, err := Lower(info)
	if err != nil {
		t.Fatal(err)
	}
	db := memoryDB(t)
	if _, err := run.MigrateUp(ctx, db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool().ExecContext(ctx, "INSERT INTO posts DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	_, err = run.MigrateUp(ctx, db, ms)
	var me *rt.MigrationError
	if !errors.As(err, &me) || me.Index != 2 || !strings.Contains(err.Error(), "NOT NULL") {
		t.Fatalf("0002 over a row: %v, want the backfill refused", err)
	}
	var cols int
	if err := db.Pool().QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('posts') WHERE name = 'seen'").Scan(&cols); err != nil || cols != 0 {
		t.Errorf("seen after the rollback: %d %v, want no column", cols, err)
	}
}
