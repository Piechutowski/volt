package itest

import (
	"context"
	"os"
	"testing"

	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/rt"
)

// TestMigrateBuildsTheSchema builds a fresh database through the
// generated Migrate alone (spec §migration-tooling rule 2), proves it
// has the structure of nao_schema.sql, and runs a query on it.
func TestMigrateBuildsTheSchema(t *testing.T) {
	ctx := context.Background()
	db, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	status, err := rt.MigrateStatus(ctx, db, Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != len(Migrations) {
		t.Fatalf("status %+v, want one entry per migration", status)
	}
	for _, s := range status {
		if !s.Applied {
			t.Fatalf("status %+v, want every migration applied", status)
		}
	}

	target, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	ddl, err := os.ReadFile("nao_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	fold, err := migrate.Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	want, err := migrate.Read(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if drifts := migrate.Diff(want, fold); len(drifts) > 0 {
		for _, d := range drifts {
			t.Errorf("drift: %s\n%s", d.Reason, d.Volt)
		}
	}

	q := New(db)
	u, err := q.UserCreate(ctx, UserNew().EmailSet("ann@example.com").NameSet("Ann"))
	if err != nil {
		t.Fatalf("UserCreate on the migrated database: %v", err)
	}
	got, err := q.UserGet(ctx, u.ID)
	if err != nil || got.Email != "ann@example.com" {
		t.Fatalf("UserGet: %+v %v", got, err)
	}
}
