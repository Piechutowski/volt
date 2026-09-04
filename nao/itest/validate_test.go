// Validation round trips (spec §V12): the generated Validate methods
// agree with the DDL's CHECK constraints on typed checks, Go-reference
// checks run in Go only, and the documented asymmetry — SQLite cannot
// call Go — is provable at the database.
package itest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
)

func TestValidateTypedCheckMirrorsDDL(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()

	good := PageView{Site: "alpha", Day: 1, Hits: 3}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid row rejected by the Go tier: %v", err)
	}
	if _, err := q.PageViewCreate(ctx, PageViewCreateParams{Site: "alpha", Day: 1}); err != nil {
		t.Fatalf("valid row rejected by the SQL tier: %v", err)
	}

	bad := PageView{Site: "alpha", Day: 0, Hits: -1}
	err := bad.Validate()
	if err == nil {
		t.Fatal("Go tier accepted a row the CHECK rejects")
	}
	var ce rt.CheckError
	if !errors.As(err, &ce) || ce.Check != "counts_positive" {
		t.Errorf("expected the named check to fail, got %v", err)
	}

	// The SQL tier is the same rendering: the raw INSERT must violate
	// the CHECK constraint.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO page_views (site, day, hits) VALUES ('alpha', 0, -1)`); err == nil {
		t.Error("SQL tier accepted a row the Go tier rejects — the tiers disagree")
	}

	// like rides both tiers too: site must have at least one character.
	empty := PageView{Site: "", Day: 1}
	if empty.Validate() == nil {
		t.Error("Go tier accepted site = '' against site like '%_'")
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO page_views (site, day, hits) VALUES ('', 1, 0)`); err == nil {
		t.Error("SQL tier accepted site = '' against site like '%_'")
	}
}

func TestValidateGoRefCheckIsGoOnly(t *testing.T) {
	db, _ := newDB(t)
	ctx := context.Background()

	bad := User{Email: "not-an-address", Name: "Ann"}
	err := bad.Validate()
	if err == nil {
		t.Fatal("Validate accepted an invalid email")
	}
	var ce rt.CheckError
	if !errors.As(err, &ce) || ce.Cause == nil {
		t.Errorf("a Go-reference failure carries its cause, got %v", err)
	}

	if (User{Email: "ann@example.com", Name: "Ann"}).Validate() != nil {
		t.Error("Validate rejected a valid email")
	}

	// The documented asymmetry (§V12.5): SQLite cannot call Go, so the
	// database accepts what only the validator can refuse.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (email, name) VALUES ('not-an-address', 'Ann')`); err != nil {
		t.Errorf("the DDL must carry no CHECK for a Go-reference check: %v", err)
	}
}

// Params structs validate the checks they can see (§V12.6): a create
// params struct carries email, so both users checks run before any
// insert; page_views' counts_positive reads hits, a defaulted column
// absent from CreateParams, so only the like check runs there.
func TestParamsValidate(t *testing.T) {
	bad := UserCreateParams{Email: "nope", Name: "N"}
	err := bad.Validate()
	var ve rt.ValidationError
	if !errors.As(err, &ve) || len(ve) != 1 || ve[0].Check != "EmailValid(email)" {
		t.Fatalf("UserCreateParams.Validate = %v, want the Go-reference failure", err)
	}
	if sc, ok := err.(interface{ StatusCode() int }); !ok || sc.StatusCode() != 422 {
		t.Errorf("a validation failure is a 422, got %v", err)
	}
	if (UserCreateParams{Email: "ok@example.com"}).Validate() != nil {
		t.Error("valid params rejected")
	}
	if (UserUpdateParams{Email: "still@example.com"}).Validate() != nil {
		t.Error("valid update params rejected")
	}

	pv := PageViewCreateParams{Site: "", Day: 1}
	if err := pv.Validate(); err == nil {
		t.Error("site like '%_' should fail on an empty site")
	}
	// hits is defaulted and absent from CreateParams: counts_positive is
	// the row's and the DDL's business, so Day alone does not fail here.
	if err := (PageViewCreateParams{Site: "a", Day: 0}).Validate(); err != nil {
		t.Errorf("a check the params cannot see must not run: %v", err)
	}
}

// TestConstraintErrorsCarryStatus: the database's refusals reach the
// caller as rt.ConstraintError with a status (§V12.7).
func TestConstraintErrorsCarryStatus(t *testing.T) {
	_, q := newDB(t)
	ctx := context.Background()
	if _, err := q.UserCreate(ctx, UserCreateParams{Email: "one@example.com", Name: "One"}); err != nil {
		t.Fatal(err)
	}
	_, err := q.UserCreate(ctx, UserCreateParams{Email: "one@example.com", Name: "Two"})
	var ce rt.ConstraintError
	if !errors.As(err, &ce) || ce.Kind != "unique" || ce.Detail != "users.email" || ce.StatusCode() != 409 {
		t.Fatalf("duplicate = %v (%+v)", err, ce)
	}
	if !strings.Contains(err.Error(), "users.email already exists") {
		t.Errorf("message = %q", err.Error())
	}
	_, err = q.PageViewCreate(ctx, PageViewCreateParams{Site: "", Day: 1})
	if !errors.As(err, &ce) || ce.Kind != "check" || ce.StatusCode() != 422 {
		t.Fatalf("check violation = %v (%+v)", err, ce)
	}
}

// TestRequiredBothTiers (§V12.8): a required column refuses its empty
// value in Go — naming the column — and in the DDL, under the same
// constraint name.
func TestRequiredBothTiers(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()
	u, err := q.UserCreate(ctx, UserCreateParams{Email: "r@example.com", Name: "R"})
	if err != nil {
		t.Fatal(err)
	}
	empty := OrderCreateParams{UserID: u.ID, Total: ""}
	var ve rt.ValidationError
	if err := empty.Validate(); !errors.As(err, &ve) || ve[0].Check != "total_required" || ve[0].Columns[0] != "total" {
		t.Fatalf("Go tier: %v", err)
	}
	if d := ve.Details(); d[0].Columns[0] != "total" {
		t.Errorf("details = %+v", d)
	}
	var ce rt.ConstraintError
	if _, err := db.ExecContext(ctx, `INSERT INTO orders (user_id, status, total) VALUES (?, 'pending', '')`, u.ID); !errors.As(rt.Constraint(err), &ce) || ce.Detail != "total_required" {
		t.Fatalf("SQL tier: %v", err)
	}
	if _, err := q.OrderCreate(ctx, OrderCreateParams{UserID: u.ID, Total: "1.00"}); err != nil {
		t.Fatalf("non-empty total rejected: %v", err)
	}
}
