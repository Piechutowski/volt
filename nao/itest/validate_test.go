// Validation round trips (spec §V12): the generated Validate methods
// agree with the DDL's CHECK constraints on typed checks, Go-reference
// checks run in Go only, and the documented asymmetry — SQLite cannot
// call Go — is provable at the database.
package itest

import (
	"context"
	"errors"
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
