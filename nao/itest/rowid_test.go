// The database-assigned key against a real SQLite (spec §columns,
// §changesets, §mapping-to-sqlite-ddl): a plain int key is assigned by the database
// and may take the largest id again after its row is deleted, an
// autoincrement key never reuses one, and an explicit IDSet is stored.
package itest

import (
	"context"
	"testing"

	"github.com/Piechutowski/volt/nao/run"
)

func TestRowidAliasAssignsAndMayReuseTheLargestID(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedUser(t, db, "ann@example.com")
	second := seedUser(t, db, "bob@example.com")
	if second.ID != 2 {
		t.Fatalf("the database assigns the next rowid: got %d, want 2", second.ID)
	}
	if _, err := run.Run(ctx, db, UserDelete(second.ID)); err != nil {
		t.Fatal(err)
	}
	third := seedUser(t, db, "cid@example.com")
	if third.ID != 2 {
		t.Errorf("a database-assigned key without autoincrement takes the largest id plus one, so the deleted 2 comes back: got %d", third.ID)
	}
	explicit, err := run.Run(ctx, db, UserCreate(UserNew().IDSet(40).EmailSet("dee@example.com").NameSet("Dee")))
	if err != nil {
		t.Fatal(err)
	}
	if explicit.ID != 40 {
		t.Errorf("IDSet stores the id given: got %d, want 40", explicit.ID)
	}
	from, err := run.Run(ctx, db, UserCreate(UserNew().From(User{Email: "eve@example.com", Name: "Eve"})))
	if err != nil {
		t.Fatal(err)
	}
	if from.ID != 41 {
		t.Errorf("From leaves the database-assigned key unmarked, so a zero row is assigned the next id: got %d, want 41", from.ID)
	}
}

func TestAutoincrementNeverReusesAnID(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	create := func(name string) MetricEvent {
		t.Helper()
		e, err := run.Run(ctx, db, MetricEventCreate(MetricEventNew().NameSet(name)))
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	create("start")
	second := create("tick")
	if _, err := run.Run(ctx, db, MetricEventDelete(second.ID)); err != nil {
		t.Fatal(err)
	}
	if third := create("tock"); third.ID != 3 {
		t.Errorf("AUTOINCREMENT never takes the deleted largest id again: got %d, want 3", third.ID)
	}
	explicit, err := run.Run(ctx, db, MetricEventCreate(MetricEventNew().IDSet(10).NameSet("jump")))
	if err != nil {
		t.Fatal(err)
	}
	if explicit.ID != 10 {
		t.Errorf("IDSet stores the id given: got %d, want 10", explicit.ID)
	}
	if _, err := run.Run(ctx, db, MetricEventDelete(explicit.ID)); err != nil {
		t.Fatal(err)
	}
	if next := create("after"); next.ID != 11 {
		t.Errorf("AUTOINCREMENT continues past the largest id it ever held: got %d, want 11", next.ID)
	}
}
