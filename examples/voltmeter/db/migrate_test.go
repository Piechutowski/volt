package db_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func schemaDB(t *testing.T) run.DB {
	t.Helper()
	target, err := run.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	ddl, err := os.ReadFile("volt_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.Pool().ExecContext(context.Background(), string(ddl)); err != nil {
		t.Fatalf("volt_schema.sql: %v", err)
	}
	return target
}

func drift(t *testing.T, d run.DB) {
	t.Helper()
	ctx := context.Background()
	target := schemaDB(t)
	want, err := migrate.Read(ctx, target.Pool())
	if err != nil {
		t.Fatal(err)
	}
	got, err := migrate.Read(ctx, d.Pool())
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range migrate.Diff(want, got) {
		t.Errorf("drift: %s\n%s", x.Reason, x.Volt)
	}
}

func names(ms []rt.Migration) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

// TestMigrationsBuildTheSchema proves the generated Migrations alone build
// a database whose structure is the schema's DDL (spec §the-fold,
// §migration-tooling), apply once, and seed through their exec statements
// the rows the schema's records put in volt_schema.sql (spec
// §records-sample-data), which reach no migration.
func TestMigrationsBuildTheSchema(t *testing.T) {
	ctx := context.Background()
	d, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	applied, err := run.MigrateUp(ctx, d, db.Migrations)
	if err != nil || !slices.Equal(applied, names(db.Migrations)) {
		t.Fatalf("MigrateUp = %v, %v", applied, err)
	}
	if again, err := run.MigrateUp(ctx, d, db.Migrations); err != nil || len(again) != 0 {
		t.Fatalf("second MigrateUp = %v, %v", again, err)
	}
	drift(t, d)
	records := schemaDB(t)
	users, seeded := do(t, d, db.UserList()), do(t, records, db.UserList())
	if len(users) != 3 || len(seeded) != 3 {
		t.Fatalf("users = %+v, records = %+v", users, seeded)
	}
	for i := range users {
		if users[i].ID != seeded[i].ID || users[i].Email != seeded[i].Email || users[i].Name != seeded[i].Name {
			t.Errorf("the migration seeded %+v where the records say %+v", users[i], seeded[i])
		}
	}
	if tags, want := do(t, d, db.TagList()), do(t, records, db.TagList()); len(tags) != 1 || tags[0] != want[0] {
		t.Errorf("tags = %+v, records = %+v", tags, want)
	}
	m := db.Migrations
	if m[5].Down != nil || !m[5].ForeignKeysOff || !m[6].NoTransaction || m[6].Down == nil || m[4].Down == nil {
		t.Errorf("flags: 0006 irreversible with the bracket, 0007 outside a transaction and reversible: %+v %+v", m[5], m[6])
	}
}

// TestMigrateDownStopsAtTheIrreversibleRebuild proves down reverts the
// migrations above the rebuild one by one, newest first, refuses the
// rebuild itself with an *rt.MigrationError, and that up applies the
// reverted ones again onto the data that survived (spec §migration-ledger,
// §migrations reversibility).
func TestMigrateDownStopsAtTheIrreversibleRebuild(t *testing.T) {
	ctx := context.Background()
	d := dbOpen(t)
	w := worldSeed(t, d)

	reversible := names(db.Migrations[6:])
	slices.Reverse(reversible)
	for _, want := range reversible {
		got, err := run.MigrateDown(ctx, d, db.Migrations)
		if err != nil || got != want {
			t.Fatalf("MigrateDown = %q, %v; want %q", got, err, want)
		}
	}
	_, err := run.MigrateDown(ctx, d, db.Migrations)
	var me *rt.MigrationError
	if !errors.As(err, &me) || me.Name != "0006__run_ids_never_reused" || !errors.Is(err, rt.ErrIrreversible) {
		t.Fatalf("MigrateDown past the rebuild = %v", err)
	}
	status, err := run.MigrateStatus(ctx, d, db.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range status {
		if s.Applied != (i < 6) {
			t.Errorf("status %s applied = %v", s.Name, s.Applied)
		}
	}
	if _, err := d.Pool().ExecContext(ctx, `SELECT 1 FROM budgets`); err == nil {
		t.Error("budgets survived the down of 0010")
	}

	applied, err := run.MigrateUp(ctx, d, db.Migrations)
	if err != nil || len(applied) != 5 {
		t.Fatalf("MigrateUp after down = %v, %v", applied, err)
	}
	drift(t, d)
	if got := do(t, d, db.RunGet(w.runs[1].ID)); got.Commit != "bbbb2222" || got.BaselineID.Valid {
		t.Errorf("a run after down and up = %+v; the baseline column came back empty", got)
	}
	if n := do(t, d, db.SampleCount()); n != 0 {
		t.Errorf("samples came back with %d rows; down dropped the table", n)
	}
}

// TestRebuildCopiesThroughFrom applies the history up to the rebuild,
// writes rows the way the older schema allowed, and proves the rebuild
// copies them through from:, lower-casing the commit, and keeps runs'
// ids so the first id it assigns is above every one copied (spec
// §migrations rebuild).
func TestRebuildCopiesThroughFrom(t *testing.T) {
	ctx := context.Background()
	d, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := run.MigrateUp(ctx, d, db.Migrations[:5]); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO machines (id, cpu, cores, memory_gb, os, arch, go) VALUES (1, 'Old CPU', 4, 8, 'linux', 'amd64', 'go1.26.0')`,
		`INSERT INTO runs (id, "commit", machine_id, note) VALUES (7, 'ABCDEF12', 1, 'before the rebuild')`,
		`INSERT INTO ms_load (run_id, tables, wall_ms, alloc_mb, heap_mb, allocs, source_kb) VALUES (7, 2, 1.5, 1, 1, 10, 0.5)`,
	} {
		if _, err := d.Pool().ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := run.MigrateUp(ctx, d, db.Migrations); err != nil {
		t.Fatal(err)
	}
	old := do(t, d, db.RunGet(7))
	if old.Commit != "abcdef12" || old.Note.V != "before the rebuild" || old.Dirty {
		t.Errorf("the copied run = %+v", old)
	}
	var raw string
	if err := d.Pool().QueryRowContext(ctx, `SELECT "commit" FROM runs WHERE id = 7`).Scan(&raw); err != nil || raw != "abcdef12" {
		t.Errorf("stored commit = %q, %v; want from: to have lowered it", raw, err)
	}
	if rows := do(t, d, db.MsLoadRows(7)); len(rows) != 1 || rows[0].OutputMb != 0 {
		t.Errorf("the phase row the rebuild kept pointing at run 7 = %+v", rows)
	}
	next := do(t, d, db.RunCreate(db.RunNew().CommitSet("1234abcd").MachineIDSet(1)))
	if next.ID != 8 {
		t.Errorf("the first id AUTOINCREMENT assigns after the rebuild = %d, want 8", next.ID)
	}
	drift(t, d)
}

// TestEarlyDownsRevertTheirStructure walks the history below the
// irreversible rebuild back to nothing, one down at a time, over rows
// written in the shape 0005 leaves (spec §migrations, reversibility):
// after each down the structure is the fold of the migrations still
// applied, a renamed table and renamed columns carry their rows back to
// their old names, the typed drop of profile comes back as the column it
// named, and every down, the exec seeds' down: among them, runs without
// error. Up then builds the schema again.
func TestEarlyDownsRevertTheirStructure(t *testing.T) {
	ctx := context.Background()
	d, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "early.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	early := db.Migrations[:5]
	if _, err := run.MigrateUp(ctx, d, early); err != nil {
		t.Fatal(err)
	}
	exec := func(q string) {
		t.Helper()
		if _, err := d.Pool().ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	read := func(q string) string {
		t.Helper()
		var v string
		if err := d.Pool().QueryRowContext(ctx, q).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v
	}
	exec(`INSERT INTO machines (id, cpu, cores, memory_gb, os, arch, go) VALUES (1, 'Old CPU', 4, 8.5, 'linux', 'amd64', 'go1.26.0')`)
	exec(`INSERT INTO runs (id, "commit", machine_id, note, dirty, tag) VALUES (7, 'abcdef12', 1, 'kept through the downs', 1, 'baseline')`)
	exec(`INSERT INTO ms_load (run_id, tables, wall_ms, alloc_mb, heap_mb, allocs, source_kb, output_mb) VALUES (7, 2, 1.5, 1, 1, 10, 0.5, 0)`)
	exec(`INSERT INTO ms_generate (run_id, tables, wall_ms, alloc_mb, heap_mb, allocs, source_kb, output_mb) VALUES (7, 2, 3.5, 2, 1, 20, 0.5, 0.25)`)

	structure := func(db *sql.DB) migrate.Schema {
		t.Helper()
		s, err := migrate.Read(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for k := len(early); k > 0; k-- {
		got, err := run.MigrateDown(ctx, d, early)
		if err != nil || got != early[k-1].Name {
			t.Fatalf("MigrateDown = %q, %v; want %s", got, err, early[k-1].Name)
		}
		ref, err := run.Open("sqlite3", filepath.Join(t.TempDir(), fmt.Sprintf("ref%d.db", k)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := run.MigrateUp(ctx, ref, early[:k-1]); err != nil {
			t.Fatal(err)
		}
		for _, x := range migrate.Diff(structure(ref.Pool()), structure(d.Pool())) {
			t.Errorf("after the down of %s: %s\n%s", early[k-1].Name, x.Reason, x.Volt)
		}
		ref.Close()
		switch k {
		case 5:
			if n := read(`SELECT count(*) FROM sqlite_master WHERE name = 'tags'`); n != "0" {
				t.Errorf("tags survived the down of 0005")
			}
			if got := read(`SELECT note || ' ' || dirty FROM runs WHERE id = 7`); got != "kept through the downs 1" {
				t.Errorf("the run after dropping its tag column = %q", got)
			}
		case 4:
			if got := read(`SELECT coalesce(profile, 'none') || ' ' || wall_ms FROM ms_load WHERE run_id = 7`); got != "none 1.5" {
				t.Errorf("ms_load after the typed drop was reverted = %q; want profile back as a NULL column", got)
			}
		case 3:
			if got := read(`SELECT wall_ms FROM ms_gen WHERE run_id = 7`); got != "3.5" {
				t.Errorf("the generate row after the table rename was reverted = %q", got)
			}
			if got := read(`SELECT comment FROM runs WHERE id = 7`); got != "kept through the downs" {
				t.Errorf("runs.comment after the rename was reverted = %q", got)
			}
			if got := read(`SELECT mem_gb FROM machines WHERE id = 1`); got != "8.5" {
				t.Errorf("machines.mem_gb after the rename was reverted = %q", got)
			}
		case 2:
			if got := read(`SELECT count(*) FROM users`); got != "3" {
				t.Errorf("%s users after the down of 0002; 0001 seeds three", got)
			}
		case 1:
			if got := read(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('users', 'machines')`); got != "0" {
				t.Errorf("%s of users and machines survived the down of 0001", got)
			}
		}
	}
	if _, err := run.MigrateUp(ctx, d, db.Migrations); err != nil {
		t.Fatal(err)
	}
	drift(t, d)
}
