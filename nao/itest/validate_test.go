// Validate round trips (spec §changesets, §validators): required, enum
// membership and the hand-written validators run in Go over the changeset,
// every write validates before it touches the database, and the database
// carries no twin of any rule.
package itest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
)

func TestValidatorsRunTopDownAndOnlyInGo(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()

	if good := PageViewNew().SiteSet("alpha").DaySet(1).HitsSet(3).Validate(); !good.Valid() || good.Err() != nil {
		t.Fatalf("valid row rejected: %v", good.Errors())
	}
	if _, err := q.PageViewCreate(ctx, PageViewNew().SiteSet("alpha").DaySet(1)); err != nil {
		t.Fatalf("valid row rejected by the database: %v", err)
	}

	bad := PageViewNew().SiteSet("").DaySet(0).HitsSet(-1).Validate()
	errs := bad.Errors()
	if len(errs) != 2 || errs[0].Check != "CountsPositive" || errs[1].Check != "SiteShape" {
		t.Fatalf("every validator runs, in declaration order: %+v", errs)
	}
	if errs[0].Message == "" || len(errs[0].Columns) != 2 || errs[0].Columns[0] != "hits" || errs[0].Columns[1] != "day" {
		t.Errorf("a failed validator carries its message and the columns it reads: %+v", errs[0])
	}
	var inv *rt.Invalid
	if err := bad.Err(); !errors.As(err, &inv) || inv.Model != "PageView" || inv.StatusCode() != 422 || len(inv.Details()) != 2 {
		t.Errorf("Err is the *rt.Invalid of the field errors: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO page_views (site, day, hits) VALUES ('', 0, -1)`); err != nil {
		t.Errorf("the database carries no CHECK for a validator: %v", err)
	}
}

func TestValidatorIsGoOnly(t *testing.T) {
	db, _ := newDB(t)
	ctx := context.Background()

	bad := UserNew().EmailSet("not-an-address").NameSet("Ann").Validate()
	errs := bad.Errors()
	if len(errs) != 1 || errs[0].Check != "EmailValid" || errs[0].Columns[0] != "email" || !strings.Contains(errs[0].Message, "not an email address") {
		t.Errorf("a validator failure carries its name, column and message, got %+v", errs)
	}
	if !UserNew().EmailSet("ann@example.com").NameSet("Ann").Validate().Valid() {
		t.Error("Validate rejected a valid email")
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (email, name) VALUES ('not-an-address', 'Ann')`); err != nil {
		t.Errorf("the database carries no CHECK for a validator: %v", err)
	}
}

func TestWritesValidateBeforeTouchingTheDatabase(t *testing.T) {
	_, q := newDB(t)
	ctx := context.Background()

	_, err := q.UserCreate(ctx, UserNew().EmailSet("nope").NameSet("N"))
	inv := invalidOf(t, err, "EmailValid", "email")
	if inv.StatusCode() != 422 || inv.Cause != nil || inv.Details()[0].Columns[0] != "email" {
		t.Errorf("a failed validator is a 422 with no driver error beneath it: %+v", inv)
	}
	if n, _ := q.UserCount(ctx); n != 0 {
		t.Errorf("an invalid changeset must write nothing, count = %d", n)
	}

	u := seedUser(t, q, "ann@example.com")
	if _, err := q.UserUpdate(ctx, u.Changeset().EmailSet("nope")); !errors.As(err, &inv) || inv.Fields[0].Check != "EmailValid" {
		t.Errorf("Update validates too: %v", err)
	}
	if again, _ := q.UserGet(ctx, u.ID); again.Email != u.Email {
		t.Errorf("a refused update must leave the row as it was: %+v", again)
	}

	twice := UserNew().EmailSet("nope").NameSet("N").Validate().Validate()
	if len(twice.Errors()) != 1 {
		t.Errorf("Validate is idempotent, got %+v", twice.Errors())
	}
	fixed := twice.EmailSet("ann@example.com").Validate()
	if !fixed.Valid() {
		t.Errorf("a later Validate replaces the earlier findings: %+v", fixed.Errors())
	}
}

func TestInvalidCarriesStatus(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()
	if _, err := q.UserCreate(ctx, UserNew().EmailSet("one@example.com").NameSet("One")); err != nil {
		t.Fatal(err)
	}
	_, err := q.UserCreate(ctx, UserNew().EmailSet("one@example.com").NameSet("Two"))
	inv := invalidOf(t, err, "unique", "email")
	if inv.StatusCode() != 409 || inv.Model != "User" || !errors.Is(err, inv.Cause) || inv.Cause == nil {
		t.Fatalf("duplicate = %v (%+v)", err, inv)
	}
	if !strings.Contains(err.Error(), "User: email: already exists") {
		t.Errorf("message = %q", err.Error())
	}
	if d := inv.Details(); len(d) != 1 || d[0].Check != "unique" || d[0].Columns[0] != "email" {
		t.Errorf("details = %+v", d)
	}

	_, raw := db.ExecContext(ctx, `INSERT INTO users (email) VALUES ('two@example.com')`)
	if inv := invalidOf(t, rt.Refused("User", raw), "not_null", "name"); inv.StatusCode() != 422 {
		t.Fatalf("not-null violation = %v (%+v)", raw, inv)
	}
}

func TestRequiredIsAFieldErrorOnly(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()
	u := seedUser(t, q, "r@example.com")

	unmarked := OrderNew().UserIDSet(u.ID).FeeSet(0).Validate()
	if errs := unmarked.Errors(); len(errs) != 1 || errs[0].Check != "required" || errs[0].Columns[0] != "total" || errs[0].Message != "total is required" {
		t.Fatalf("an unmarked required column on a create: %+v", errs)
	}
	zero := OrderNew().UserIDSet(u.ID).TotalSet(0).FeeSet(0).Validate()
	if errs := zero.Errors(); len(errs) != 1 || errs[0].Check != "required" {
		t.Fatalf("the explicit empty value is refused too: %+v", errs)
	}
	_, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).FeeSet(0))
	if inv := invalidOf(t, err, "required", "total"); inv.StatusCode() != 422 {
		t.Errorf("Create refuses it with a 422: %+v", inv)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO orders (user_id, status, total, fee) VALUES (?, 'pending', 0, 0)`, u.ID); err != nil {
		t.Fatalf("the database carries no CHECK for required: %v", err)
	}
	if _, err := q.OrderCreate(ctx, OrderNew().UserIDSet(u.ID).TotalSet(1.00).FeeSet(0)); err != nil {
		t.Fatalf("non-empty total rejected: %v", err)
	}
}
