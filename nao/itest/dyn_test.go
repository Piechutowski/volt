// Integration tests for the v2 dynamic query layer:
// predicates, options and typed assignments run against a float64 SQLite
// through a float64 driver, sharing the fixture with the CRUD tests.
package itest

import (
	"context"
	"errors"
	"github.com/Piechutowski/volt/nao/run"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

func dynSeed(t *testing.T, db run.DB) (ann, bea, bob User) {
	t.Helper()
	ctx := context.Background()
	create := func(email, name string, bio rt.Null[string]) User {
		u, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet(email).NameSet(name).BioSet(bio)))
		if err != nil {
			t.Fatalf("UserCreate(%s): %v", email, err)
		}
		return u
	}
	ann = create("ann@example.com", "Ann", rt.Some("gardener"))
	bea = create("bea@example.com", "Bo", rt.Null[string]{})
	bob = create("bob@example.com", "Bo", rt.Null[string]{})
	return ann, bea, bob
}

func emails(us []User) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.Email
	}
	return out
}

func TestUserQueryPredicates(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ann, bea, bob := dynSeed(t, db)

	cases := []struct {
		name string
		opts []rt.Opt[User]
		want []string
	}{
		{"eq", []rt.Opt[User]{UserEmail.Eq("ann@example.com")}, []string{"ann@example.com"}},
		{"like", []rt.Opt[User]{UserEmail.Like("b%")}, []string{"bea@example.com", "bob@example.com"}},
		{"isnull", []rt.Opt[User]{UserBio.IsNull()}, []string{"bea@example.com", "bob@example.com"}},
		{"isnotnull", []rt.Opt[User]{UserBio.IsNotNull()}, []string{"ann@example.com"}},
		{"in", []rt.Opt[User]{UserID.In(ann.ID, bob.ID)}, []string{"ann@example.com", "bob@example.com"}},
		{"in empty", []rt.Opt[User]{UserID.In()}, nil},
		{"notin", []rt.Opt[User]{UserID.NotIn(bea.ID)}, []string{"ann@example.com", "bob@example.com"}},
		{"gt", []rt.Opt[User]{UserID.Gt(ann.ID)}, []string{"bea@example.com", "bob@example.com"}},
		{
			"two opts join with and",
			[]rt.Opt[User]{UserName.Eq("Bo"), UserBio.IsNull()},
			[]string{"bea@example.com", "bob@example.com"},
		},
		{
			"or with not",
			[]rt.Opt[User]{rt.Or(UserEmail.Eq("ann@example.com"), rt.Not(UserName.Eq("Bo")))},
			[]string{"ann@example.com"},
		},
		{"eqcol no match", []rt.Opt[User]{UserEmail.EqCol(UserName)}, nil},
		{"raw", []rt.Opt[User]{rt.Raw[User]("length(\"name\") = ?", 2)}, []string{"bea@example.com", "bob@example.com"}},
		{"empty pred means all", []rt.Opt[User]{rt.Pred[User]{}}, []string{"ann@example.com", "bea@example.com", "bob@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := run.Run(ctx, db, UserQuery(tc.opts...))
			if err != nil {
				t.Fatalf("UserQuery: %v", err)
			}
			if gotEmails := emails(got); !equalStrings(gotEmails, tc.want) {
				t.Errorf("UserQuery = %v, want %v", gotEmails, tc.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUserQueryShaping(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)

	got, err := run.Run(ctx, db, UserQuery(UserOrderBy(UserEmail.Desc()), UserLimit(2)))
	if err != nil {
		t.Fatalf("UserQuery: %v", err)
	}
	if want := []string{"bob@example.com", "bea@example.com"}; !equalStrings(emails(got), want) {
		t.Errorf("desc+limit = %v, want %v", emails(got), want)
	}

	got, err = run.Run(ctx, db, UserQuery(UserOrderBy(UserEmail.Asc()), UserLimit(2), UserOffset(2)))
	if err != nil {
		t.Fatalf("UserQuery: %v", err)
	}
	if want := []string{"bob@example.com"}; !equalStrings(emails(got), want) {
		t.Errorf("offset page = %v, want %v", emails(got), want)
	}
}

// TestUserQueryKeyset walks the whole table one row at a time via After
// , over a deliberately non-unique first order term with a mixed-
// direction unique tiebreak — the case OFFSET pagination gets wrong when
// rows shift between pages.
func TestUserQueryKeyset(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)

	var walked []string
	var opts []rt.Opt[User]
	base := []rt.Opt[User]{UserOrderBy(UserName.Asc(), UserID.Desc()), UserLimit(1)}
	opts = base
	for i := 0; i < 10; i++ {
		page, err := run.Run(ctx, db, UserQuery(opts...))
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		if len(page) == 0 {
			break
		}
		last := page[len(page)-1]
		walked = append(walked, last.Email)
		opts = append(append([]rt.Opt[User]{}, base...), UserAfter(last.Name, last.ID))
	}
	if want := []string{"ann@example.com", "bob@example.com", "bea@example.com"}; !equalStrings(walked, want) {
		t.Errorf("keyset walk = %v, want %v", walked, want)
	}

	if _, err := run.Run(ctx, db, UserQuery(UserAfter("x"))); err == nil || !strings.Contains(err.Error(), "OrderBy") {
		t.Errorf("After without OrderBy: err = %v", err)
	}
}

func TestUserCountExists(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)

	if n, err := run.Run(ctx, db, UserCount()); err != nil || n != 3 {
		t.Errorf("bare UserCount = %d, %v; want 3", n, err)
	}
	if n, err := run.Run(ctx, db, UserCount(UserName.Eq("Bo"))); err != nil || n != 2 {
		t.Errorf("filtered UserCount = %d, %v; want 2", n, err)
	}
	if ok, err := run.Run(ctx, db, UserExists(UserBio.IsNotNull())); err != nil || !ok {
		t.Errorf("UserExists(bio set) = %v, %v; want true", ok, err)
	}
	if ok, err := run.Run(ctx, db, UserExists(UserEmail.Eq("nobody@example.com"))); err != nil || ok {
		t.Errorf("UserExists(missing) = %v, %v; want false", ok, err)
	}
}

func TestUserUpdateWhere(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ann, _, _ := dynSeed(t, db)

	n, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserBio.Set("beekeeper")), UserID.Eq(ann.ID)))
	if err != nil || n != 1 {
		t.Fatalf("UserUpdateWhere = %d, %v; want 1 row", n, err)
	}
	got, err := run.Run(ctx, db, UserGet(ann.ID))
	if err != nil || got.Bio.Or("") != "beekeeper" || got.Email != ann.Email {
		t.Errorf("after update: %+v, %v", got, err)
	}

	if n, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserBio.SetNull()), UserID.Eq(ann.ID))); err != nil || n != 1 {
		t.Fatalf("SetNull = %d, %v", n, err)
	}
	if got, _ := run.Run(ctx, db, UserGet(ann.ID)); got.Bio.Valid {
		t.Errorf("bio still set after SetNull: %+v", got.Bio)
	}

	if n, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserName.Set("x")), UserID.Eq(99))); err != nil || n != 0 {
		t.Errorf("miss = %d, %v; want 0, nil", n, err)
	}

	if _, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserName.Set("x")))); err == nil {
		t.Error("UserUpdateWhere without predicate: want error")
	}
	if n, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserName.Set("Everyone")), rt.Raw[User]("1 = 1"))); err != nil || n != 3 {
		t.Errorf("explicit all-rows update = %d, %v; want 3", n, err)
	}
}

func TestUserDeleteWhere(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)

	if _, err := run.Run(ctx, db, UserDeleteWhere()); err == nil {
		t.Error("UserDeleteWhere without predicate: want error")
	}
	n, err := run.Run(ctx, db, UserDeleteWhere(UserName.Eq("Bo")))
	if err != nil || n != 2 {
		t.Fatalf("UserDeleteWhere = %d, %v; want 2", n, err)
	}
	if left, err := run.Run(ctx, db, UserCount()); err != nil || left != 1 {
		t.Errorf("rows left = %d, %v; want 1", left, err)
	}
}

// TestOrderDynamic exercises the handle of a declared type and a second model's
// surface: a Pred[Order] is a different type from a Pred[User], so
// cross-model mixups never reach the database (a compile error).
func TestOrderDynamic(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ann, _, _ := dynSeed(t, db)

	placed, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(9.99).FeeSet(0).PlacedAtSet(rt.Some(time.Now().UTC()))))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}
	unplaced, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(15.00).FeeSet(0).PlacedAtSet(rt.Null[time.Time]{})))
	if err != nil {
		t.Fatalf("OrderCreate: %v", err)
	}

	if n, err := run.Run(ctx, db, OrderCount(OrderStatus.Eq(OrderStatePending))); err != nil || n != 2 {
		t.Errorf("pending count = %d, %v; want 2", n, err)
	}
	if n, err := run.Run(ctx, db, OrderUpdateWhere(OrderSet(OrderStatus.Set(OrderStateShipped)), OrderID.Eq(placed.ID))); err != nil || n != 1 {
		t.Fatalf("ship = %d, %v", n, err)
	}
	shipped, err := run.Run(ctx, db, OrderQuery(OrderStatus.Eq(OrderStateShipped)))
	if err != nil || len(shipped) != 1 || shipped[0].ID != placed.ID {
		t.Errorf("shipped = %+v, %v; want just order %d", shipped, err, placed.ID)
	}
	if never, err := run.Run(ctx, db, OrderQuery(OrderPlacedAt.IsNull())); err != nil || len(never) != 1 || never[0].ID != unplaced.ID {
		t.Errorf("unplaced = %+v, %v; want just order %d", never, err, unplaced.ID)
	}
}

// TestDynStmtCache proves the statement cache wiring end to end: same option shape,
// different values -> one prepared statement; a new shape -> a second.

// TestDynInTx runs dynamic verbs inside Queries.Tx: the transactional
// copy drops the statement cache by construction (its statements
// belong to the outer handle) and a rollback undoes the writes.

// TestUserUpdateWhereRunsNoValidator pins the bulk-path bypass of
// §changesets: UpdateWhere takes assignments, never a row, so a value
// EmailValid refuses in Validate is written and read back unchanged.
func TestUserUpdateWhereRunsNoValidator(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	ann, _, _ := dynSeed(t, db)

	const bad = "not-an-address"
	if UserNew().EmailSet(bad).Validate().Valid() {
		t.Fatalf("EmailValid accepted %q; the test needs a value Validate refuses", bad)
	}
	n, err := run.Run(ctx, db, UserUpdateWhere(UserSet(UserEmail.Set(bad)), UserID.Eq(ann.ID)))
	if err != nil || n != 1 {
		t.Fatalf("UserUpdateWhere = %d, %v; want 1 row and no validator", n, err)
	}
	got, err := run.Run(ctx, db, UserGet(ann.ID))
	if err != nil || got.Email != bad {
		t.Errorf("after the bulk write: %+v, %v; want email %q", got, err, bad)
	}
}

func TestDynStmtCache(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)
	before := db.Statements()

	for _, name := range []string{"Ann", "Bo", "Nobody"} {
		if _, err := run.Run(ctx, db, UserQuery(UserName.Eq(name), UserLimit(5))); err != nil {
			t.Fatalf("cached query (%s): %v", name, err)
		}
	}
	if n := db.Statements() - before; n != 1 {
		t.Errorf("cache grew by %d statements after 3 same-shape queries; want 1, values must ride as parameters", n)
	}
	if _, err := run.Run(ctx, db, UserQuery(UserName.Eq("Ann"), UserBio.IsNull(), UserLimit(5))); err != nil {
		t.Fatal(err)
	}
	if n := db.Statements() - before; n != 2 {
		t.Errorf("cache grew by %d statements after a second shape; want 2", n)
	}
}

func TestDynInTx(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	dynSeed(t, db)

	_, err := run.Run(ctx, db, rt.Tx(
		UserDeleteWhere(UserName.Eq("Bo")),
		UserCreate(UserNew().EmailSet("ann@example.com").NameSet("Dup")),
	))
	var inv *rt.Invalid
	if !errors.As(err, &inv) {
		t.Fatalf("Tx err = %v, want the refused create", err)
	}
	if n, err := run.Run(ctx, db, UserCount()); err != nil || n != 3 {
		t.Errorf("after rollback count = %d, %v; want 3", n, err)
	}

	out, err := run.Run(ctx, db, rt.Tx(UserDeleteWhere(UserName.Eq("Bo"))))
	if err != nil || len(out) != 1 || out[0] != int64(2) {
		t.Fatalf("committing tx = %v, %v; want two rows deleted", out, err)
	}
	if n, err := run.Run(ctx, db, UserCount()); err != nil || n != 1 {
		t.Errorf("after commit count = %d, %v; want 1", n, err)
	}
}
