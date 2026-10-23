// Changeset round trips (spec §changesets): Cast over JSON and GOB
// bodies, value semantics, partial updates, the no-op update, ErrNoRow,
// DEFAULT VALUES and the statement cache, all against a real SQLite.
package itest

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

func paramsOf(t *testing.T, doc string) UserParams {
	t.Helper()
	var p UserParams
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCastPartialUpdateSetsOnlyTheCastColumn(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")

	cs := u.Changeset().Cast(paramsOf(t, `{"bio": "hey"}`), UserEmail, UserName, UserBio, UserCreatedAt)
	if !cs.Valid() || !cs.Changed(UserBio) || cs.Changed(UserEmail) || cs.Changed(UserName) || cs.Changed(UserCreatedAt) {
		t.Fatalf("Cast marks the carried column only: %+v %+v", cs.Errors(), cs.Params().Keys())
	}
	upd, err := q.UserUpdate(ctx, cs)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Bio.V != "hey" || upd.Email != u.Email || upd.Name != u.Name || !upd.CreatedAt.Equal(u.CreatedAt) {
		t.Errorf("a PATCH body changes its columns alone: %+v", upd)
	}
}

func TestCastReportsUnknownKeysAndIgnoresDisallowedOnes(t *testing.T) {
	cs := UserNew().Cast(paramsOf(t, `{"emial": "x", "email": "a@b.c"}`), UserEmail)
	errs := cs.Errors()
	if len(errs) != 1 || errs[0].Check != "unknown_field" || errs[0].Columns[0] != "emial" || errs[0].Message != `unknown field "emial"` {
		t.Fatalf("a key that names no column is an unknown_field error: %+v", errs)
	}
	if !cs.Changed(UserEmail) {
		t.Error("the known key is still cast")
	}
	var inv *rt.Invalid
	if err := cs.Err(); !errors.As(err, &inv) || len(inv.Fields) != 1 || inv.Fields[0].Check != "unknown_field" {
		t.Errorf("Err = %v; want the *rt.Invalid of the unknown_field error", err)
	}

	outside := UserNew().Cast(paramsOf(t, `{"id": 7, "displayName": "Bo"}`), UserEmail)
	if !outside.Valid() || outside.Changed(UserID) || outside.Changed(UserName) || outside.Row().Name != "" {
		t.Errorf("a column outside allow is ignored, never an error: %+v %+v", outside.Errors(), outside.Row())
	}
}

func TestCastTypeMismatchIsAFieldError(t *testing.T) {
	cs := UserNew().EmailSet("keep@example.com").Cast(paramsOf(t, `{"created_at": 12, "email": null}`), UserEmail, UserCreatedAt)
	errs := cs.Errors()
	if len(errs) != 2 {
		t.Fatalf("want two cast errors, got %+v", errs)
	}
	byCol := map[string]rt.FieldError{}
	for _, e := range errs {
		byCol[e.Columns[0]] = e
	}
	if e := byCol["created_at"]; e.Check != "cast" || e.Message != "created_at: expected time.Time" {
		t.Errorf("created_at: %+v", e)
	}
	if e := byCol["email"]; e.Check != "cast" || e.Message != "email: null is not allowed (string)" {
		t.Errorf("email: %+v", e)
	}
	if cs.Changed(UserCreatedAt) || cs.Row().Email != "keep@example.com" {
		t.Errorf("a value that does not convert leaves the column as it was: %+v", cs.Row())
	}
}

func TestCastNullOnANullableColumn(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u, err := q.UserCreate(ctx, UserNew().EmailSet("ann@example.com").NameSet("Ann").BioSet(rt.Some("hi")))
	if err != nil {
		t.Fatal(err)
	}
	cs := u.Changeset().Cast(paramsOf(t, `{"bio": null}`), UserBio)
	if !cs.Valid() || !cs.Changed(UserBio) || cs.Row().Bio.Valid {
		t.Fatalf("null on a nullable column sets NULL: %+v %+v", cs.Errors(), cs.Row())
	}
	upd, err := q.UserUpdate(ctx, cs)
	if err != nil || upd.Bio.Valid {
		t.Errorf("stored bio = %+v, %v; want NULL", upd.Bio, err)
	}
}

func TestGobParamsRoundTripThroughCast(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	when := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(UserNew().EmailSet("gob@example.com").NameSet("Gob").CreatedAtSet(when).Params()); err != nil {
		t.Fatal(err)
	}
	var p UserParams
	if err := gob.NewDecoder(&buf).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Keys(), []string{"created_at", "displayName", "email"}) {
		t.Fatalf("keys = %v", p.Keys())
	}
	cs := UserNew().Cast(p, UserEmail, UserName, UserBio, UserCreatedAt)
	if !cs.Valid() || cs.Changed(UserBio) || !cs.Changed(UserName) || cs.Row().Email != "gob@example.com" || !cs.Row().CreatedAt.Equal(when) {
		t.Fatalf("GOB carries the row and the marks: %+v %+v", cs.Errors(), cs.Row())
	}
	u, err := q.UserCreate(ctx, cs)
	if err != nil || u.Name != "Gob" || !u.CreatedAt.Equal(when) {
		t.Errorf("UserCreate = %+v, %v", u, err)
	}

	viaJSON := paramsOf(t, `{"email": "j@example.com"}`)
	buf.Reset()
	if err := gob.NewEncoder(&buf).Encode(viaJSON); err != nil {
		t.Fatal(err)
	}
	var again UserParams
	if err := gob.NewDecoder(&buf).Decode(&again); err != nil {
		t.Fatal(err)
	}
	if cs := UserNew().Cast(again, UserEmail, UserName); !cs.Changed(UserEmail) || cs.Changed(UserName) || cs.Row().Email != "j@example.com" {
		t.Errorf("a JSON body re-encodes as GOB with the same marks: %+v", cs.Row())
	}
}

func TestChangesetValueSemantics(t *testing.T) {
	base := UserNew().Cast(paramsOf(t, `{"nope": 1}`), UserEmail)
	a := base.ErrorAdd(UserEmail, "a")
	b := base.ErrorAdd(UserEmail, "b")
	if len(base.Errors()) != 1 || a.Errors()[1].Message != "a" || b.Errors()[1].Message != "b" || a.Errors()[1].Check != "custom" {
		t.Errorf("copies of one changeset never share errors: base %+v a %+v b %+v", base.Errors(), a.Errors(), b.Errors())
	}
	marked := base.EmailSet("x@example.com")
	if base.Changed(UserEmail) || !marked.Changed(UserEmail) {
		t.Error("a setter marks the copy it returns, not its receiver")
	}
	u := User{ID: 3, Email: "e@example.com", Name: "N", CreatedAt: time.Now()}
	from := UserNew().From(u)
	if from.Changed(UserID) || !from.Changed(UserEmail) || !from.Changed(UserBio) || from.Row().Name != "N" {
		t.Errorf("From marks every data column but the auto-increment key: %v", from.Params().Keys())
	}
	if keys := from.Params().Keys(); !reflect.DeepEqual(keys, []string{"bio", "created_at", "displayName", "email"}) {
		t.Errorf("Params keys = %v", keys)
	}
	if same := u.Changeset().EmailSet(u.Email); !same.Changed(UserEmail) {
		t.Error("a column set to the value it holds is still a change")
	}
}

func TestUpdateWithNothingMarkedIsANoOp(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")
	if _, err := db.ExecContext(ctx, `UPDATE users SET name = 'Renamed' WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	got, err := q.UserUpdate(ctx, u.Changeset())
	if err != nil || got.Name != u.Name {
		t.Errorf("nothing marked returns Row() without touching the database: %+v, %v", got, err)
	}
	if _, err := q.UserUpdate(ctx, UserNew().EmailSet("x@example.com")); !errors.Is(err, rt.ErrNoRow) {
		t.Errorf("Update on UserNew() = %v; want rt.ErrNoRow", err)
	}
}

func TestMembershipIsAFieldError(t *testing.T) {
	errs := OrderNew().StatusSet("teleported").Validate().Errors()
	if len(errs) != 2 || errs[0].Check != "required" || errs[1].Check != "OrderStateKnown" || errs[1].Columns[0] != "status" || errs[1].Message != `"teleported" is not a known order status` {
		t.Errorf("required first, then the membership validator: %+v", errs)
	}
	if got := OrderNew().Row().Status; got != OrderStatePending {
		t.Errorf("OrderNew seeds the literal default of a declared type whose Go declaration spells its storage: status %q, want pending", got)
	}
	if errs := OrderNew().TotalSet(1).Validate().Errors(); len(errs) != 0 {
		t.Errorf("an unmarked column of a declared type on a create holds its default and passes: %+v", errs)
	}
	if errs := OrderNew().TotalSet(1).StatusSet("").Validate().Errors(); len(errs) != 1 || errs[0].Check != "OrderStateKnown" {
		t.Errorf("a column blanked on a create is checked: %+v", errs)
	}
	if errs := (Order{Status: "teleported", Total: 1}).Changeset().Validate().Errors(); len(errs) != 1 || errs[0].Check != "OrderStateKnown" {
		t.Errorf("a row's column is always checked: %+v", errs)
	}
	if errs := (Order{Status: OrderStateShipped, Total: 1}).Changeset().Validate().Errors(); len(errs) != 0 {
		t.Errorf("a member value passes: %+v", errs)
	}
}

func TestCreateDefaults(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)

	_, err := q.LinkClickCreate(ctx, LinkClickNew())
	if inv := invalidOf(t, err, "not_null", "site"); inv.Cause == nil {
		t.Errorf("nothing marked inserts DEFAULT VALUES, which the database refuses on its first NOT NULL column: %+v", inv)
	}
	lc, err := q.LinkClickCreate(ctx, LinkClickNew().SiteSet("alpha").DaySet(1))
	if err != nil || lc.Target != "" || lc.ID == 0 {
		t.Errorf("an unmarked column takes its default: %+v, %v", lc, err)
	}

	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	pinned, err := q.UserCreate(ctx, UserNew().EmailSet("a@example.com").NameSet("A").CreatedAtSet(when))
	if err != nil || !pinned.CreatedAt.Equal(when) {
		t.Errorf("a marked created_at is stored as given: %+v, %v", pinned, err)
	}
	stamped, err := q.UserCreate(ctx, UserNew().EmailSet("b@example.com").NameSet("B"))
	if err != nil || stamped.CreatedAt.IsZero() || stamped.CreatedAt.Equal(when) {
		t.Errorf("an unmarked created_at takes CURRENT_TIMESTAMP: %+v, %v", stamped, err)
	}
}

func TestChangesetWritesUseTheStatementCache(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	cache := rt.NewStmtCache(8)
	defer cache.Close()
	cq := q.WithCache(cache)

	if _, err := cq.UserCreate(ctx, UserNew().EmailSet("a@example.com").NameSet("A")); err != nil {
		t.Fatal(err)
	}
	u, err := cq.UserCreate(ctx, UserNew().EmailSet("b@example.com").NameSet("B"))
	if err != nil {
		t.Fatal(err)
	}
	if n := cache.Len(); n != 1 {
		t.Errorf("two creates of one shape prepare one statement, got %d", n)
	}
	if _, err := cq.UserCreate(ctx, UserNew().EmailSet("c@example.com").NameSet("C").BioSet(rt.Some("x"))); err != nil {
		t.Fatal(err)
	}
	if _, err := cq.UserUpdate(ctx, u.Changeset().NameSet("B2")); err != nil {
		t.Fatal(err)
	}
	if _, err := cq.UserUpdate(ctx, u.Changeset().NameSet("B3")); err != nil {
		t.Fatal(err)
	}
	if n := cache.Len(); n != 3 {
		t.Errorf("a new mark shape prepares a new statement, got %d", n)
	}
}
