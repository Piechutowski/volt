package itest

import (
	"context"
	"github.com/Piechutowski/volt/internal/dbtest"
	"github.com/Piechutowski/volt/nao/run"
	"os"
	"testing"

	"github.com/Piechutowski/volt/nao/migrate"
)

// TestMigrateBuildsTheSchema builds a fresh database through the
// generated Migrate alone (spec §migration-tooling), proves it
// has the structure of volt_schema.sql, and runs a query on it.
func TestMigrateBuildsTheSchema(t *testing.T) {
	ctx := context.Background()
	db := dbtest.Open(t, "")
	if _, err := run.MigrateUp(ctx, db, Migrations); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if applied, err := run.MigrateUp(ctx, db, Migrations); err != nil || len(applied) != 0 {
		t.Fatalf("second MigrateUp = %v, %v; want nothing applied", applied, err)
	}
	status, err := run.MigrateStatus(ctx, db, Migrations)
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

	ddl, err := os.ReadFile("volt_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	target := dbtest.Open(t, string(ddl))
	fold, err := migrate.Read(ctx, db.Pool())
	if err != nil {
		t.Fatal(err)
	}
	want, err := migrate.Read(ctx, target.Pool())
	if err != nil {
		t.Fatal(err)
	}
	if drifts := migrate.Diff(want, fold); len(drifts) > 0 {
		for _, d := range drifts {
			t.Errorf("drift: %s\n%s", d.Reason, d.Volt)
		}
	}

	u, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@example.com").NameSet("Ann")))
	if err != nil {
		t.Fatalf("UserCreate on the migrated database: %v", err)
	}
	got, err := run.Run(ctx, db, UserGet(u.ID))
	if err != nil || got.Email != "ann@example.com" {
		t.Fatalf("UserGet: %+v %v", got, err)
	}
}
