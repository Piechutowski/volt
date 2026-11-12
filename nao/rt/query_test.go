package rt

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type m struct{}

var (
	id    = Column[m, int64]{Name: "id"}
	email = Column[m, string]{Name: "email"}
	bio   = NullColumn[m, string]{Column[m, string]{Name: "bio"}}
)

func selectRender(t *testing.T, opts ...Opt[m]) (string, []any) {
	t.Helper()
	query, args, err := SelectRender(`"users"`, `"id", "email", "bio"`, opts)
	if err != nil {
		t.Fatalf("SelectRender: %v", err)
	}
	return query, args
}

func argsCheck(t *testing.T, got []any, want ...any) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v, want %#v", got, want)
	}
}

func TestSelectRenderBare(t *testing.T) {
	query, args := selectRender(t)
	if want := `SELECT "id", "email", "bio" FROM "users"`; query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args)
}

func TestSelectRenderOperators(t *testing.T) {
	cases := []struct {
		name string
		pred Pred[m]
		sql  string
		args []any
	}{
		{"eq", email.Eq("a"), `"email" = ?`, []any{"a"}},
		{"ne", email.Ne("a"), `"email" != ?`, []any{"a"}},
		{"gt", id.Gt(4), `"id" > ?`, []any{int64(4)}},
		{"ge", id.Ge(4), `"id" >= ?`, []any{int64(4)}},
		{"lt", id.Lt(4), `"id" < ?`, []any{int64(4)}},
		{"le", id.Le(4), `"id" <= ?`, []any{int64(4)}},
		{"like", email.Like("%@x%"), `"email" LIKE ?`, []any{"%@x%"}},
		{"notlike", email.NotLike("%@x%"), `"email" NOT LIKE ?`, []any{"%@x%"}},
		{"in", id.In(1, 2, 3), `"id" IN (?, ?, ?)`, []any{int64(1), int64(2), int64(3)}},
		{"in empty", id.In(), `1 = 0`, nil},
		{"notin", id.NotIn(1), `"id" NOT IN (?)`, []any{int64(1)}},
		{"notin empty", id.NotIn(), `1 = 1`, nil},
		{"inlist", id.InList([]int64{1, 2}), `"id" IN (SELECT value FROM json_each(?))`, []any{JSONList([]int64{1, 2})}},
		{"inlist empty", id.InList(nil), `"id" IN (SELECT value FROM json_each(?))`, []any{JSONList[int64](nil)}},
		{"isnull", bio.IsNull(), `"bio" IS NULL`, nil},
		{"isnotnull", bio.IsNotNull(), `"bio" IS NOT NULL`, nil},
		{"eqcol", email.EqCol(Column[m, string]{Name: "name"}), `"email" = "name"`, nil},
		{"necol", email.NeCol(Column[m, string]{Name: "name"}), `"email" != "name"`, nil},
		{"ltcol", id.LtCol(Column[m, int64]{Name: "parent_id"}), `"id" < "parent_id"`, nil},
		{"raw", Raw[m]("length(email) > ?", 3), `(length(email) > ?)`, []any{3}},
		{"and", And(email.Eq("a"), id.Gt(1)), `("email" = ? AND "id" > ?)`, []any{"a", int64(1)}},
		{"or", Or(email.Eq("a"), bio.IsNull()), `("email" = ? OR "bio" IS NULL)`, []any{"a"}},
		{"not", Not(email.Eq("a")), `NOT ("email" = ?)`, []any{"a"}},
		{"not not", Not(Not(email.Eq("a"))), `NOT (NOT ("email" = ?))`, []any{"a"}},
		{"not group", Not(Or(email.Eq("a"), bio.IsNull())), `NOT ("email" = ? OR "bio" IS NULL)`, []any{"a"}},
		{"not raw", Not(Raw[m]("length(email) > ?", 3)), `NOT (length(email) > ?)`, []any{3}},
		{
			"nested",
			And(Or(id.Lt(1), id.Gt(9)), Not(And(email.Eq("a"), bio.IsNotNull()))),
			`(("id" < ? OR "id" > ?) AND NOT ("email" = ? AND "bio" IS NOT NULL))`,
			[]any{int64(1), int64(9), "a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, args := selectRender(t, tc.pred)
			want := `SELECT "id", "email", "bio" FROM "users" WHERE ` + tc.sql
			if query != want {
				t.Errorf("query = %q, want %q", query, want)
			}
			argsCheck(t, args, tc.args...)
		})
	}
}

func TestSelectRenderEmptyPreds(t *testing.T) {
	var zero Pred[m]
	cases := []struct {
		name string
		opts []Opt[m]
	}{
		{"zero value", []Opt[m]{zero}},
		{"and of nothing", []Opt[m]{And[m]()}},
		{"and of zeros", []Opt[m]{And(zero, zero)}},
		{"not of zero", []Opt[m]{Not(zero)}},
		{"raw empty", []Opt[m]{Raw[m]("")}},
		{"nil opt", []Opt[m]{nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, args := selectRender(t, tc.opts...)
			if want := `SELECT "id", "email", "bio" FROM "users"`; query != want {
				t.Errorf("query = %q, want %q", query, want)
			}
			argsCheck(t, args)
		})
	}
	query, _ := selectRender(t, And(zero, email.Eq("a"), zero))
	if want := `SELECT "id", "email", "bio" FROM "users" WHERE "email" = ?`; query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
}

func TestSelectRenderMultiplePredsJoinWithAnd(t *testing.T) {
	query, args := selectRender(t, email.Like("%x%"), bio.IsNull())
	want := `SELECT "id", "email", "bio" FROM "users" WHERE "email" LIKE ? AND "bio" IS NULL`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, "%x%")
}

func TestSelectRenderShaping(t *testing.T) {
	query, args := selectRender(t,
		Distinct[m](),
		email.Like("%@x"),
		OrderBy(email.Asc(), id.Desc()),
		Limit[m](10),
		Offset[m](20),
	)
	want := `SELECT DISTINCT "id", "email", "bio" FROM "users" WHERE "email" LIKE ? ORDER BY "email" ASC, "id" DESC LIMIT ? OFFSET ?`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, "%@x", 10, 20)
}

// A select's limit: (spec §selects-over-groups) renders after ORDER BY
// over its value slot, and LimitCheck refuses a limit below 1, which
// SQLite would read as none when negative.
func TestSelectRenderLimitVal(t *testing.T) {
	query, args, err := SelectRender(`"users"`, `"id"`, []Opt[Gen]{OrderBy(OrderTerm("id", true)), LimitVal("limit")})
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT "id" FROM "users" ORDER BY "id" DESC LIMIT ?`; query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, "limit")
	for _, n := range []int{1, 50} {
		if err := LimitCheck(n); err != nil {
			t.Errorf("LimitCheck(%d) = %v, want nil", n, err)
		}
	}
	for _, n := range []int{0, -1} {
		if err := LimitCheck(n); !errors.Is(err, ErrLimit) || !strings.Contains(err.Error(), fmt.Sprint(n)) {
			t.Errorf("LimitCheck(%d) = %v, want ErrLimit naming %d", n, err, n)
		}
	}
}

// The dynamic layer's Limit below 1 is a rendering that fails with
// ErrLimit (spec §generated-query-surface), as a select's limit: parameter
// below 1 is, so LIMIT -1 never reaches SQLite as no limit at all.
func TestSelectRenderLimitBelowOne(t *testing.T) {
	for _, n := range []int{0, -1} {
		query, args, err := SelectRender(`"users"`, `"id"`, []Opt[m]{OrderBy(id.Asc()), Limit[m](n)})
		if !errors.Is(err, ErrLimit) || query != "" || args != nil {
			t.Errorf("Limit(%d) = %q, %v, %v; want ErrLimit and no statement", n, query, args, err)
		}
	}
	query, args := selectRender(t, Limit[m](1))
	if want := `SELECT "id", "email", "bio" FROM "users" LIMIT ?`; query != want {
		t.Errorf("Limit(1) = %q, want %q", query, want)
	}
	argsCheck(t, args, 1)
}

func TestSelectRenderOffsetWithoutLimit(t *testing.T) {
	query, args := selectRender(t, Offset[m](5))
	if want := `SELECT "id", "email", "bio" FROM "users" LIMIT -1 OFFSET ?`; query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, 5)
}

func TestSelectRenderAfter(t *testing.T) {
	query, args := selectRender(t, OrderBy(id.Asc()), After[m](int64(7)))
	want := `SELECT "id", "email", "bio" FROM "users" WHERE "id" > ? ORDER BY "id" ASC`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, int64(7))

	query, args = selectRender(t,
		bio.IsNotNull(),
		OrderBy(email.Desc(), id.Asc()),
		After[m]("bob@x", int64(3)),
		Limit[m](2),
	)
	want = `SELECT "id", "email", "bio" FROM "users" WHERE "bio" IS NOT NULL AND ` +
		`("email" < ? OR ("email" = ? AND "id" > ?)) ORDER BY "email" DESC, "id" ASC LIMIT ?`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	argsCheck(t, args, "bob@x", "bob@x", int64(3), 2)
}

func TestSelectRenderAfterIsAPredicate(t *testing.T) {
	name := Column[m, string]{Name: "name"}
	cases := []struct {
		label string
		opts  []Opt[m]
		preds []Pred[m]
	}{
		{
			"one term",
			[]Opt[m]{OrderBy(id.Asc()), After[m](int64(7))},
			[]Pred[m]{id.Gt(7)},
		},
		{
			"two terms",
			[]Opt[m]{email.Like("%x"), OrderBy(email.Desc(), id.Asc()), After[m]("bob@x", int64(3))},
			[]Pred[m]{email.Like("%x"), Or(email.Lt("bob@x"), And(email.Eq("bob@x"), id.Gt(3)))},
		},
		{
			"three terms",
			[]Opt[m]{OrderBy(name.Asc(), email.Desc(), id.Asc()), After[m]("n", "e", int64(1))},
			[]Pred[m]{Or(name.Gt("n"), And(name.Eq("n"), email.Lt("e")), And(name.Eq("n"), email.Eq("e"), id.Gt(1)))},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, gotArgs := selectRender(t, tc.opts...)
			want, wantArgs, err := WhereRender(tc.preds)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, " WHERE "+want+" ORDER BY ") {
				t.Errorf("After renders\n  %s\nthe equivalent predicate\n  %s", got, want)
			}
			argsCheck(t, gotArgs, wantArgs...)
		})
	}
}

func TestSelectRenderAfterErrors(t *testing.T) {
	if _, _, err := SelectRender(`"users"`, `*`, []Opt[m]{After[m](1)}); err == nil || !strings.Contains(err.Error(), "OrderBy") {
		t.Errorf("After without OrderBy: err = %v, want OrderBy complaint", err)
	}
	if _, _, err := SelectRender(`"users"`, `*`, []Opt[m]{OrderBy(id.Asc(), email.Asc()), After[m](1)}); err == nil || !strings.Contains(err.Error(), "key values") {
		t.Errorf("After arity mismatch: err = %v, want key-count complaint", err)
	}
}

func TestCountExistsRender(t *testing.T) {
	query, args, err := CountRender(`"users"`, []Pred[m]{email.Eq("a")})
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT count(*) FROM "users" WHERE "email" = ?`; query != want {
		t.Errorf("count = %q, want %q", query, want)
	}
	argsCheck(t, args, "a")

	query, _, err = CountRender[m](`"users"`, nil)
	if err != nil || query != `SELECT count(*) FROM "users"` {
		t.Errorf("bare count = %q, %v", query, err)
	}

	query, args, err = ExistsRender(`"users"`, []Pred[m]{id.Gt(int64(0))})
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT EXISTS (SELECT 1 FROM "users" WHERE "id" > ?)`; query != want {
		t.Errorf("exists = %q, want %q", query, want)
	}
	argsCheck(t, args, int64(0))
}

func TestDeleteUpdateRender(t *testing.T) {
	query, args, err := DeleteRender(`"users"`, []Pred[m]{id.Eq(1)})
	if err != nil {
		t.Fatal(err)
	}
	if want := `DELETE FROM "users" WHERE "id" = ?`; query != want {
		t.Errorf("delete = %q, want %q", query, want)
	}
	argsCheck(t, args, int64(1))

	query, args, err = UpdateRender(`"users"`,
		[]Assign[m]{email.Set("new@x"), bio.SetNull()},
		[]Pred[m]{id.Eq(1)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := `UPDATE "users" SET "email" = ?, "bio" = ? WHERE "id" = ?`; query != want {
		t.Errorf("update = %q, want %q", query, want)
	}
	argsCheck(t, args, "new@x", nil, int64(1))
}

func TestWholeTableGuards(t *testing.T) {
	var zero Pred[m]
	if _, _, err := DeleteRender[m](`"users"`, nil); err == nil {
		t.Error("DeleteRender(nil preds): want error")
	}
	if _, _, err := DeleteRender(`"users"`, []Pred[m]{zero, And(zero)}); err == nil {
		t.Error("DeleteRender(empty preds): want error")
	}
	if _, _, err := UpdateRender(`"users"`, []Assign[m]{email.Set("x")}, nil); err == nil {
		t.Error("UpdateRender(no preds): want error")
	}
	if _, _, err := UpdateRender(`"users"`, nil, []Pred[m]{id.Eq(1)}); err == nil {
		t.Error("UpdateRender(no assignments): want error")
	}
	if _, _, err := DeleteRender(`"users"`, []Pred[m]{Raw[m]("1 = 1")}); err != nil {
		t.Errorf("DeleteRender(Raw true): %v", err)
	}
}

func TestRenderDeterminism(t *testing.T) {
	build := func() []Opt[m] {
		return []Opt[m]{
			Or(email.Like("%a%"), And(id.Gt(1), bio.IsNull())),
			OrderBy(id.Desc()),
			Limit[m](3),
		}
	}
	q1, a1, err1 := SelectRender(`"users"`, `*`, build())
	q2, a2, err2 := SelectRender(`"users"`, `*`, build())
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if q1 != q2 {
		t.Errorf("same tree rendered differently:\n%q\n%q", q1, q2)
	}
	if !reflect.DeepEqual(a1, a2) {
		t.Errorf("same tree bound differently: %#v vs %#v", a1, a2)
	}
	q3, _, _ := SelectRender(`"users"`, `*`, []Opt[m]{
		Or(email.Like("%zzz%"), And(id.Gt(99), bio.IsNull())),
		OrderBy(id.Desc()),
		Limit[m](8),
	})
	if q1 != q3 {
		t.Errorf("same shape, different values changed the SQL text:\n%q\n%q", q1, q3)
	}
}

func TestIdentQuote(t *testing.T) {
	weird := Column[m, string]{Name: `evil"col`}
	query, _ := selectRender(t, weird.Eq("x"))
	if !strings.Contains(query, `"evil""col" = ?`) {
		t.Errorf("quoting failed: %q", query)
	}
}

func TestConstructorsCopyTheirSlices(t *testing.T) {
	render := func(opts []Opt[m]) string {
		sql, args, err := SelectRender[m](`"t"`, "*", opts)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(sql, args)
	}
	args := []any{1}
	terms := []Order[m]{Column[m, int]{Name: "a"}.Asc()}
	key := []any{10}
	cols := []string{"a"}
	opts := []Opt[m]{Raw[m]("a = ?", args...), OrderBy(terms...), After[m](key...), GroupBy[m](cols...)}
	want := render(opts)
	args[0], terms[0], key[0], cols[0] = 2, Column[m, int]{Name: "b"}.Desc(), 99, "b"
	if got := render(opts); got != want {
		t.Errorf("options changed with the caller's slices:\n got %s\nwant %s", got, want)
	}

	guards := []any{"p"}
	on := [][2]string{{"a", "b"}}
	gen := []GenPred{PredOptional(guards, PredNull("a")), PredExists("t", "x", on, GenPred{})}
	where := func() string {
		sql, args, _ := WhereRender(gen)
		return fmt.Sprint(sql, args)
	}
	wantGen := where()
	guards[0], on[0] = "q", [2]string{"c", "d"}
	if got := where(); got != wantGen {
		t.Errorf("generator trees changed with the caller's slices:\n got %s\nwant %s", got, wantGen)
	}

	steps := []AnyEffect{Pure(1)}
	tx, all := Tx(steps...), All(steps...)
	wantTx, wantAll := Describe(tx), Describe(all)
	steps[0] = Pure(2)
	if Describe(tx) != wantTx || Describe(all) != wantAll {
		t.Errorf("effects changed with the caller's slice: %s, %s", Describe(tx), Describe(all))
	}
}
