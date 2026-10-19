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

	good := PageView{Site: "alpha", Day: 1, Hits: 3}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid row rejected: %v", err)
	}
	if _, err := q.PageViewCreate(ctx, PageViewCreateParams{Site: "alpha", Day: 1}); err != nil {
		t.Fatalf("valid row rejected by the database: %v", err)
	}

	bad := PageView{Site: "", Day: 0, Hits: -1}
	err := bad.Validate()
	var ve rt.ValidationError
	if !errors.As(err, &ve) || len(ve) != 2 || ve[0].Check != "CountsPositive" || ve[1].Check != "SiteShape" {
		t.Fatalf("every validator runs, in declaration order: %v", err)
	}
	if ve[0].Cause == nil || len(ve[0].Columns) != 2 || ve[0].Columns[0] != "hits" || ve[0].Columns[1] != "day" {
		t.Errorf("a failed validator carries its cause and columns: %+v", ve[0])
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO page_views (site, day, hits) VALUES ('', 0, -1)`); err != nil {
		t.Errorf("the database carries no CHECK for a validator: %v", err)
	}
}

func TestValidatorIsGoOnly(t *testing.T) {
	db, _ := newDB(t)
	ctx := context.Background()

	bad := User{Email: "not-an-address", Name: "Ann"}
	err := bad.Validate()
	if err == nil {
		t.Fatal("Validate accepted an invalid email")
	}
	var ce rt.CheckError
	if !errors.As(err, &ce) || ce.Cause == nil || ce.Check != "EmailValid" {
		t.Errorf("a validator failure carries its cause and name, got %v", err)
	}

	if (User{Email: "ann@example.com", Name: "Ann"}).Validate() != nil {
		t.Error("Validate rejected a valid email")
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (email, name) VALUES ('not-an-address', 'Ann')`); err != nil {
		t.Errorf("the database carries no CHECK for a validator: %v", err)
	}
}

func TestParamsValidate(t *testing.T) {
	bad := UserCreateParams{Email: "nope", Name: "N"}
	err := bad.Validate()
	var ve rt.ValidationError
	if !errors.As(err, &ve) || len(ve) != 1 || ve[0].Check != "EmailValid" {
		t.Fatalf("UserCreateParams.Validate = %v, want the validator failure", err)
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
		t.Error("SiteShape should fail on an empty site")
	}
	if err := (PageViewCreateParams{Site: "a", Day: 0}).Validate(); err != nil {
		t.Errorf("a validator reading a column the params do not carry must not run: %v", err)
	}
}

func TestConstraintErrorsCarryStatus(t *testing.T) {
	db, q := newDB(t)
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
	_, err = db.ExecContext(ctx, `INSERT INTO users (email) VALUES ('two@example.com')`)
	if !errors.As(rt.Constraint(err), &ce) || ce.Kind != "not null" || ce.StatusCode() != 422 {
		t.Fatalf("not-null violation = %v (%+v)", err, ce)
	}
}

func TestRequiredIsGoOnly(t *testing.T) {
	db, q := newDB(t)
	ctx := context.Background()
	u, err := q.UserCreate(ctx, UserCreateParams{Email: "r@example.com", Name: "R"})
	if err != nil {
		t.Fatal(err)
	}
	empty := OrderCreateParams{UserID: u.ID, Total: 0}
	var ve rt.ValidationError
	if err := empty.Validate(); !errors.As(err, &ve) || ve[0].Check != "total_required" || ve[0].Columns[0] != "total" {
		t.Fatalf("required: %v", err)
	}
	if d := ve.Details(); d[0].Columns[0] != "total" {
		t.Errorf("details = %+v", d)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO orders (user_id, status, total, fee) VALUES (?, 'pending', 0, 0)`, u.ID); err != nil {
		t.Fatalf("the database carries no CHECK for required: %v", err)
	}
	if _, err := q.OrderCreate(ctx, OrderCreateParams{UserID: u.ID, Total: 1.00}); err != nil {
		t.Fatalf("non-empty total rejected: %v", err)
	}
}
