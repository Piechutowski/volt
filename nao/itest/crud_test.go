// Integration tests: the generated CRUD runs against a real SQLite through
// a real database/sql driver, no mocks, no canned rows. The driver is a
// test-only dependency; generated code and the rt runtime stay driver-free.
package itest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
	_ "github.com/mattn/go-sqlite3"
)

func newDB(t *testing.T) (*sql.DB, *Queries) {
	t.Helper()
	db, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ddl, err := os.ReadFile("volt_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(ddl)); err != nil {
		t.Fatalf("generated DDL rejected by SQLite: %v", err)
	}
	return db, New(db)
}

func seedUser(t *testing.T, q *Queries, email string) User {
	t.Helper()
	u, err := q.UserCreate(context.Background(), UserNew().EmailSet(email).NameSet("Ann"))
	if err != nil {
		t.Fatalf("UserCreate: %v", err)
	}
	return u
}

// invalidOf asserts err is an *rt.Invalid carrying exactly the given
// check on the given columns and returns it.
func invalidOf(t *testing.T, err error, check string, cols ...string) *rt.Invalid {
	t.Helper()
	var inv *rt.Invalid
	if !errors.As(err, &inv) {
		t.Fatalf("want *rt.Invalid with check %s, got %v", check, err)
	}
	if len(inv.Fields) != 1 || inv.Fields[0].Check != check || strings.Join(inv.Fields[0].Columns, ",") != strings.Join(cols, ",") {
		t.Fatalf("want one %s field error on %v, got %+v", check, cols, inv.Fields)
	}
	return inv
}

func TestUserCRUD(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)

	u, err := q.UserCreate(ctx, UserNew().EmailSet("ann@example.com").NameSet("Ann"))
	if err != nil {
		t.Fatalf("UserCreate: %v", err)
	}
	if u.ID != 1 || u.CreatedAt.IsZero() {
		t.Errorf("UserCreate returned %+v; want ID=1 and a database-filled created_at", u)
	}
	if u.Bio.Valid {
		t.Errorf("bio should be NULL, got %+v", u.Bio)
	}

	got, err := q.UserGet(ctx, u.ID)
	if err != nil || got.Email != "ann@example.com" {
		t.Errorf("UserGet = %+v, %v", got, err)
	}
	byEmail, err := q.UserGetByEmail(ctx, "ann@example.com")
	if err != nil || byEmail.ID != u.ID {
		t.Errorf("UserGetByEmail = %+v, %v", byEmail, err)
	}

	u2 := seedUser(t, q, "bob@example.com")
	many, err := q.UserGetMany(ctx, []int{u.ID, u2.ID, 999})
	if err != nil || len(many) != 2 {
		t.Errorf("UserGetMany = %d rows, %v; want 2 (missing keys are not errors)", len(many), err)
	}
	if none, err := q.UserGetMany(ctx, nil); err != nil || none != nil {
		t.Errorf("UserGetMany(nil) = %v, %v; want nil, nil", none, err)
	}

	all, err := q.UserList(ctx)
	if err != nil || len(all) != 2 {
		t.Errorf("UserList = %d rows, %v", len(all), err)
	}
	if n, err := q.UserCount(ctx); err != nil || n != 2 {
		t.Errorf("UserCount = %d, %v", n, err)
	}

	upd, err := q.UserUpdate(ctx, u.Changeset().NameSet("Ann B.").BioSet(rt.Some("hi there")))
	if err != nil {
		t.Fatalf("UserUpdate: %v", err)
	}
	if upd.Name != "Ann B." || !upd.Bio.Valid || upd.Bio.V != "hi there" {
		t.Errorf("UserUpdate returned %+v", upd)
	}
	if upd.Email != u.Email || !upd.CreatedAt.Equal(u.CreatedAt) {
		t.Errorf("an update writes the marked columns only; email and created_at changed: %+v", upd)
	}

	if _, err := q.UserUpdate(ctx, User{ID: 999, Email: "x@x", Name: "x"}.Changeset().NameSet("y")); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("UserUpdate of a vanished row = %v; want rt.ErrNotFound", err)
	}
	if _, err := q.UserGet(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UserGet(999) = %v; want sql.ErrNoRows compatibility", err)
	}

	if err := q.UserDelete(ctx, u2.ID); err != nil {
		t.Errorf("UserDelete: %v", err)
	}
	if err := q.UserDelete(ctx, u2.ID); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("second UserDelete = %v; want rt.ErrNotFound", err)
	}

	_, err = q.UserCreate(ctx, UserNew().EmailSet("ann@example.com").NameSet("Imposter"))
	if inv := invalidOf(t, err, "unique", "email"); inv.Cause == nil {
		t.Errorf("a duplicate email is a unique field error wrapping the driver's error: %+v", inv)
	}
}

func TestOrderDefaultsMembershipAndFKs(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	o, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(19.99).FeeSet(0))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	if o.Status != OrderStatePending {
		t.Errorf("status = %q; want the database default %q for an unmarked column", o.Status, OrderStatePending)
	}
	if o.Tip != 0 || o.PlacedAt.Valid {
		t.Errorf("tip and placed_at take their defaults, got %+v", o)
	}
	if o.Total != 19.99 {
		t.Errorf("total = %v, want 19.99", o.Total)
	}

	placed := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	upd, err := q.OrderUpdate(ctx, o.Changeset().StatusSet(OrderStateShipped).PlacedAtSet(rt.Some(placed)))
	if err != nil {
		t.Fatalf("OrderUpdate: %v", err)
	}
	if upd.Status != OrderStateShipped || !upd.PlacedAt.Valid || !upd.PlacedAt.V.Equal(placed) || upd.Total != o.Total {
		t.Errorf("OrderUpdate returned %+v", upd)
	}

	_, err = q.OrderCreate(ctx, OrderNew().UserIDSet(999).TotalSet(1.00).FeeSet(0))
	if inv := invalidOf(t, err, "foreign_key"); inv.Cause == nil {
		t.Errorf("a dangling user_id is a foreign_key field error without a column, wrapping the driver's error: %+v", inv)
	}

	_, err = q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(1.00))
	if inv := invalidOf(t, err, "not_null", "fee"); inv.Cause == nil {
		t.Errorf("an unmarked NOT NULL column without a default is a not_null field error wrapping the driver's error: %+v", inv)
	}

	_, err = q.OrderUpdate(ctx, o.Changeset().StatusSet("teleported"))
	if inv := invalidOf(t, err, "OrderStateKnown", "status"); inv.Cause != nil {
		t.Errorf("a value outside the constants is a field error of the membership validator before any statement runs: %+v", inv)
	}
	if _, err := db.ExecContext(ctx, `UPDATE orders SET status = 'teleported' WHERE id = ?`, o.ID); err != nil {
		t.Errorf("the database carries no CHECK for a validated column: %v", err)
	}
}

func TestUserTagCompositePK(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	tag, err := q.UserTagCreate(ctx, UserTagNew().UserIDSet(u.ID).TagSet("vip"))
	if err != nil {
		t.Fatalf("UserTagCreate: %v", err)
	}
	got, err := q.UserTagGet(ctx, tag.UserID, tag.Tag)
	if err != nil || got != tag {
		t.Errorf("UserTagGet = %+v, %v", got, err)
	}
	_, err = q.UserTagCreate(ctx, UserTagNew().From(tag))
	if inv := invalidOf(t, err, "unique", "user_id", "tag"); inv.Cause == nil {
		t.Errorf("a duplicate composite key names both columns and wraps the driver's error: %+v", inv)
	}
	if err := q.UserTagDelete(ctx, tag.UserID, tag.Tag); err != nil {
		t.Errorf("UserTagDelete: %v", err)
	}
	if _, err := q.UserTagGet(ctx, tag.UserID, tag.Tag); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("UserTagGet after delete = %v; want rt.ErrNotFound", err)
	}
}

func TestTx(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)

	err := q.Tx(ctx, func(q *Queries) error {
		_, err := q.UserCreate(ctx, UserNew().EmailSet("tx@example.com").NameSet("Tx"))
		return err
	})
	if err != nil {
		t.Fatalf("Tx: %v", err)
	}
	if n, _ := q.UserCount(ctx); n != 1 {
		t.Errorf("after committed Tx, count = %d; want 1", n)
	}

	sentinel := errors.New("boom")
	err = q.Tx(ctx, func(q *Queries) error {
		if _, err := q.UserCreate(ctx, UserNew().EmailSet("gone@example.com").NameSet("Gone")); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Tx = %v; want the fn error back", err)
	}
	if n, _ := q.UserCount(ctx); n != 1 {
		t.Errorf("after rolled-back Tx, count = %d; want 1", n)
	}

	err = q.Tx(ctx, func(inner *Queries) error {
		return inner.Tx(ctx, func(*Queries) error { return nil })
	})
	if err == nil {
		t.Error("nested Tx unexpectedly succeeded")
	}
}

// TestJSONNull pins the Null[T] wire format: nullable columns marshal as the
// value or null, the JSON never shows a wrapper and never omits the key.
func TestJSONNull(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	doc, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), `"bio":null`) {
		t.Errorf("NULL bio marshals as %s; want \"bio\":null", doc)
	}

	upd, err := q.UserUpdate(ctx, u.Changeset().BioSet(rt.Some("hi")))
	if err != nil {
		t.Fatal(err)
	}
	doc, err = json.Marshal(upd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), `"bio":"hi"`) {
		t.Errorf("bio marshals as %s; want \"bio\":\"hi\"", doc)
	}

	var back User
	if err := json.Unmarshal(doc, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Bio.Valid || back.Bio.V != "hi" {
		t.Errorf("round-tripped bio = %+v", back.Bio)
	}
}

func TestStmtCache(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)
	seedUser(t, q, "ann@example.com")

	cache := rt.NewStmtCache(2)
	defer cache.Close()

	s1, err := cache.Prepare(ctx, db, userGetSQL)
	if err != nil {
		t.Fatal(err)
	}
	var u User
	if err := s1.QueryRowContext(ctx, 1).Scan(&u.ID, &u.Email, &u.Name, &u.Bio, &u.CreatedAt); err != nil {
		t.Fatalf("cached statement: %v", err)
	}
	again, err := cache.Prepare(ctx, db, userGetSQL)
	if err != nil || again != s1 {
		t.Errorf("second Prepare returned a different statement (%p vs %p, %v); the SQL text is the cache key", again, s1, err)
	}

	if _, err := cache.Prepare(ctx, db, userListSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Prepare(ctx, db, userDeleteSQL); err != nil {
		t.Fatal(err)
	}
	if n := cache.Len(); n != 2 {
		t.Errorf("cache holds %d statements; want 2 after eviction", n)
	}

	if err := cache.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if _, err := cache.Prepare(ctx, db, userGetSQL); err == nil {
		t.Error("Prepare on a closed cache unexpectedly succeeded")
	}
}

func TestOpenPragmas(t *testing.T) {
	db, err := rt.Open("sqlite3", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, %v; want 1", fk, err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, %v; want wal", mode, err)
	}
	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Errorf("busy_timeout = %d, %v; want 5000", timeout, err)
	}
}
