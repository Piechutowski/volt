// Optional parameters and exists terms on a real SQLite (spec
// §predicates): a term that reads an absent optional parameter is taken
// out, true under and and false under or, a present one filters as written, an optional list present and empty
// matches no row, and exists tests a relation without repeating rows.
package itest

import (
	"context"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

type viewSeed struct {
	site      string
	day, hits int
}

func viewsSeed(t *testing.T, db run.DB, seeds []viewSeed) []PageView {
	t.Helper()
	var out []PageView
	for _, s := range seeds {
		v, err := run.Run(context.Background(), db, PageViewCreate(PageViewNew().SiteSet(s.site).DaySet(s.day).HitsSet(s.hits)))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func viewIDs(rows []PageView) []int {
	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func orderIDs(rows []Order) []int {
	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func idsSame(a, b []int) bool {
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

func TestOptionalAbsentAndPresent(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	v := viewsSeed(t, db, []viewSeed{{"blog", 1, 5}, {"blog", 2, 50}, {"docs", 3, 80}})
	for _, tc := range []struct {
		name string
		q    rt.Query[PageView]
		want []int
	}{
		{"both absent", PageViewPicked(rt.Null[string]{}, rt.Null[int]{}), []int{v[0].ID, v[1].ID, v[2].ID}},
		{"site present", PageViewPicked(rt.Some("blog"), rt.Null[int]{}), []int{v[0].ID, v[1].ID}},
		{"min present", PageViewPicked(rt.Null[string]{}, rt.Some(40)), []int{v[1].ID, v[2].ID}},
		{"both present", PageViewPicked(rt.Some("blog"), rt.Some(40)), []int{v[1].ID}},
		{"a present empty string is a value", PageViewPicked(rt.Some(""), rt.Null[int]{}), []int{}},
		{"list absent", PageViewInSites(rt.Null[[]string]{}), []int{v[0].ID, v[1].ID, v[2].ID}},
		{"list present", PageViewInSites(rt.Some([]string{"docs"})), []int{v[2].ID}},
		{"list present and empty", PageViewInSites(rt.Some([]string{})), []int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := run.Run(ctx, db, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := viewIDs(rows); !idsSame(got, tc.want) {
				t.Errorf("rows %v, want %v", got, tc.want)
			}
		})
	}
}

// The term of an absent optional parameter is taken out even where the
// column is NULL, and a present time binds as its stored form (spec
// §go-types).
func TestOptionalTimeOverNullColumn(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	early := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var ids []int
	for _, at := range []rt.Null[time.Time]{rt.Some(early), {}, rt.Some(early.Add(48 * time.Hour))} {
		o, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(1).FeeSet(1).PlacedAtSet(at)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, o.ID)
	}
	all, err := run.Run(ctx, db, OrderPlacedBy(rt.Null[time.Time]{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("an absent bound keeps %d orders, want all 3, the one never placed included", len(all))
	}
	before, err := run.Run(ctx, db, OrderPlacedBy(rt.Some(early.Add(24*time.Hour))))
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].ID != ids[0] {
		t.Errorf("a present bound keeps %+v, want the order placed first alone", before)
	}
}

// Absent is what binds NULL: a present float64 NaN, which SQLite binds as
// NULL, reads as absent and keeps every row, where the same NaN compared
// by a term with no guard matches none (spec §predicates).
func TestOptionalNaNReadsAbsent(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	for _, total := range []float64{5, 9} {
		if _, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(total).FeeSet(1))); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		min  rt.Null[float64]
		want int
	}{
		{"absent", rt.Null[float64]{}, 2},
		{"present", rt.Some(6.0), 1},
		{"present NaN", rt.Some(math.NaN()), 2},
	} {
		rows, err := run.Run(ctx, db, OrderCosting(tc.min))
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != tc.want {
			t.Errorf("%s: %d orders, want %d", tc.name, len(rows), tc.want)
		}
	}
	rows, err := run.Run(ctx, db, OrderQuery(OrderTotal.Ge(math.NaN())))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("total >= NaN with no guard keeps %d orders, want none", len(rows))
	}
}

// A keyset over (day desc, id desc) whose first page passes its cursor
// absent and every later page the last row's values walks every row once.
func TestOptionalKeysetFirstPage(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	v := viewsSeed(t, db, []viewSeed{{"blog", 3, 1}, {"blog", 1, 1}, {"blog", 3, 1}, {"docs", 2, 1}, {"docs", 1, 1}})
	want := []int{v[2].ID, v[0].ID, v[3].ID, v[4].ID, v[1].ID}
	var got []int
	day, id := rt.Null[int]{}, rt.Null[int]{}
	for pages := 0; ; pages++ {
		if pages > len(v) {
			t.Fatalf("the keyset does not end; read %v", got)
		}
		rows, err := run.Run(ctx, db, PageViewKeyset(day, id, 2))
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		got = append(got, viewIDs(rows)...)
		last := rows[len(rows)-1]
		day, id = rt.Some(last.Day), rt.Some(last.ID)
	}
	if !idsSame(got, want) {
		t.Errorf("pages read %v, want %v", got, want)
	}
}

// A keyset over a nullable column pages soundly once the where keeps its
// NULLs out; without that, a page that ends on a NULL hands the next page
// an absent time, its terms vanish and the id's alone stays, and the next
// page reads again rows an earlier page read (spec §predicates).
func TestOptionalKeysetNullableColumn(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	early := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	late := early.Add(48 * time.Hour)
	var o []Order
	for _, at := range []rt.Null[time.Time]{rt.Some(early), {}, rt.Some(late), rt.Some(late), {}} {
		row, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(1).FeeSet(1).PlacedAtSet(at)))
		if err != nil {
			t.Fatal(err)
		}
		o = append(o, row)
	}
	want := []int{o[3].ID, o[2].ID, o[0].ID}
	var got []int
	at, id := rt.Null[time.Time]{}, rt.Null[int]{}
	for pages := 0; ; pages++ {
		if pages > len(o) {
			t.Fatalf("the keyset does not end; read %v", got)
		}
		rows, err := run.Run(ctx, db, OrderPlacedKeyset(at, id, 2))
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		got = append(got, orderIDs(rows)...)
		last := rows[len(rows)-1]
		at, id = last.PlacedAt, rt.Some(last.ID)
	}
	if !idsSame(got, want) {
		t.Errorf("pages read %v, want the placed orders %v", got, want)
	}

	first, err := run.Run(ctx, db, OrderPlacedKeysetLoose(rt.Null[time.Time]{}, rt.Null[int]{}, 4))
	if err != nil {
		t.Fatal(err)
	}
	last := first[len(first)-1]
	if last.PlacedAt.Valid {
		t.Fatalf("the first page ends on %+v, want an order never placed", last)
	}
	again, err := run.Run(ctx, db, OrderPlacedKeysetLoose(last.PlacedAt, rt.Some(last.ID), 4))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(orderIDs(again), func(id int) bool { return slices.Contains(orderIDs(first), id) && id != last.ID }) {
		t.Errorf("the page after a NULL cursor is %v, want it to read again a row of the first page %v", orderIDs(again), orderIDs(first))
	}
}

// Filtering through a join of a reverse relation repeats each row once
// per matching row, which a grouped count reads; exists keeps the row
// once, so the count is the user's orders (spec §predicates).
func TestExistsReverseRelationKeepsCounts(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("bob@blog.test").NameSet("Bob")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("cy@blog.test").NameSet("Cy"))); err != nil {
		t.Fatal(err)
	}
	for _, w := range []rt.AnyEffect{
		UserTagCreate(UserTagNew().UserIDSet(ann.ID).TagSet("go")),
		UserTagCreate(UserTagNew().UserIDSet(ann.ID).TagSet("sql")),
		UserTagCreate(UserTagNew().UserIDSet(bob.ID).TagSet("go")),
		OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(1).FeeSet(1)),
		OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(2).FeeSet(1)),
		OrderCreate(OrderNew().UserIDSet(ann.ID).TotalSet(3).FeeSet(1)),
		OrderCreate(OrderNew().UserIDSet(bob.ID).TotalSet(4).FeeSet(1)),
	} {
		if _, err := run.Run(ctx, db, rt.Tx(w)); err != nil {
			t.Fatal(err)
		}
	}
	tags := []string{"go", "sql"}
	joined, err := run.Run(ctx, db, UserJoinedCounts(tags))
	if err != nil {
		t.Fatal(err)
	}
	if len(joined) != 2 || joined[0].ID != ann.ID || joined[0].N != 6 || joined[1].N != 1 {
		t.Fatalf("through a join Ann's 3 orders count %+v, want 6, twice over her two matching tags, the inflation exists avoids", joined)
	}
	counted, err := run.Run(ctx, db, UserOrderCounts(tags))
	if err != nil {
		t.Fatal(err)
	}
	if len(counted) != 2 || counted[0].ID != ann.ID || counted[0].N != 3 || counted[1].ID != bob.ID || counted[1].N != 1 {
		t.Errorf("through exists the counts are %+v, want Ann 3 and Bob 1, Cy untagged and left out", counted)
	}
	none, err := run.Run(ctx, db, UserOrderCounts(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("exists over an empty list keeps %+v, want no user", none)
	}
}

// An exists term whose every optional parameter is absent is taken out,
// true for every row, those the relation reaches nothing from included.
func TestExistsOptionalVanishesWhole(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("bob@blog.test").NameSet("Bob")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Run(ctx, db, UserTagCreate(UserTagNew().UserIDSet(ann.ID).TagSet("go"))); err != nil {
		t.Fatal(err)
	}
	all, err := run.Run(ctx, db, UserTagged(rt.Null[string]{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != ann.ID || all[1].ID != bob.ID {
		t.Errorf("an absent tag keeps %+v, want Ann and the untagged Bob", all)
	}
	gophers, err := run.Run(ctx, db, UserTagged(rt.Some("go")))
	if err != nil {
		t.Fatal(err)
	}
	if len(gophers) != 1 || gophers[0].ID != ann.ID {
		t.Errorf("tag go keeps %+v, want Ann alone", gophers)
	}
	nobody, err := run.Run(ctx, db, UserTagged(rt.Some("rust")))
	if err != nil {
		t.Fatal(err)
	}
	if len(nobody) != 0 {
		t.Errorf("an unused tag keeps %+v, want nobody", nobody)
	}
}

// A term that reads an absent optional parameter is taken out of the
// predicate, so an or keeps the operand whose parameter is present and an
// exists term the inner term whose parameter is present, and either is
// taken out whole only when every parameter it reads is absent (spec
// §predicates).
func TestOptionalAbsentTakesTheTermOut(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann := seedUser(t, db, "ann@blog.test")
	bob := seedUser(t, db, "bob@blog.test")
	cy := seedUser(t, db, "cy@blog.test")
	var o []Order
	for _, row := range []struct {
		user  int
		total float64
		state OrderState
	}{{ann.ID, 5, OrderStatePending}, {bob.ID, 50, OrderStateShipped}, {bob.ID, 1, OrderStatePending}} {
		made, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(row.user).TotalSet(row.total).FeeSet(1).StatusSet(row.state)))
		if err != nil {
			t.Fatal(err)
		}
		o = append(o, made)
	}
	for _, tc := range []struct {
		name  string
		buyer rt.Null[int]
		over  rt.Null[float64]
		want  []int
	}{
		{"both absent", rt.Null[int]{}, rt.Null[float64]{}, []int{o[0].ID, o[1].ID, o[2].ID}},
		{"buyer alone", rt.Some(ann.ID), rt.Null[float64]{}, []int{o[0].ID}},
		{"over alone", rt.Null[int]{}, rt.Some(10.0), []int{o[1].ID}},
		{"both", rt.Some(ann.ID), rt.Some(10.0), []int{o[0].ID, o[1].ID}},
	} {
		rows, err := run.Run(ctx, db, OrderEither(tc.buyer, tc.over))
		if err != nil || !idsSame(orderIDs(rows), tc.want) {
			t.Errorf("either, %s: %v, %v; want %v", tc.name, orderIDs(rows), err, tc.want)
		}
	}
	for _, tc := range []struct {
		name  string
		state rt.Null[OrderState]
		above rt.Null[float64]
		want  []int
	}{
		{"both absent", rt.Null[OrderState]{}, rt.Null[float64]{}, []int{ann.ID, bob.ID, cy.ID}},
		{"state alone", rt.Some(OrderStateShipped), rt.Null[float64]{}, []int{bob.ID}},
		{"above alone", rt.Null[OrderState]{}, rt.Some(2.0), []int{ann.ID, bob.ID}},
		{"both, on one order", rt.Some(OrderStatePending), rt.Some(2.0), []int{ann.ID}},
	} {
		rows, err := run.Run(ctx, db, UserSpenders(tc.state, tc.above))
		got := make([]int, len(rows))
		for i, u := range rows {
			got[i] = u.ID
		}
		if err != nil || !idsSame(got, tc.want) {
			t.Errorf("spenders, %s: %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
}

// exists over a forward relation tests the one row the foreign key
// refers to.
func TestExistsForwardRelation(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	ann, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("ann@blog.test").NameSet("Ann")))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := run.Run(ctx, db, UserCreate(UserNew().EmailSet("bob@blog.test").NameSet("Bob")))
	if err != nil {
		t.Fatal(err)
	}
	var anns []int
	for _, u := range []User{ann, bob, ann} {
		o, err := run.Run(ctx, db, OrderCreate(OrderNew().UserIDSet(u.ID).TotalSet(1).FeeSet(1)))
		if err != nil {
			t.Fatal(err)
		}
		if u.ID == ann.ID {
			anns = append(anns, o.ID)
		}
	}
	rows, err := run.Run(ctx, db, OrderBoughtBy("Ann"))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, len(rows))
	for i, r := range rows {
		got[i] = r.ID
	}
	if !idsSame(got, anns) {
		t.Errorf("orders bought by Ann: %v, want %v", got, anns)
	}
}
