package lang

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/rt"
)

type renderModel struct{}

var (
	rID    = rt.Column[renderModel, int]{Name: "id"}
	rSite  = rt.Column[renderModel, string]{Name: "site"}
	rDay   = rt.Column[renderModel, int]{Name: "day"}
	rHits  = rt.Column[renderModel, int]{Name: "hits"}
	rScore = rt.Column[renderModel, float64]{Name: "score"}
	rFlag  = rt.Column[renderModel, bool]{Name: "flag"}
	rNote  = rt.NullColumn[renderModel, string]{Column: rt.Column[renderModel, string]{Name: "note"}}
	rOrder = rt.Column[renderModel, int]{Name: "order"}
)

const renderColumns = `"id", "site", "day", "hits", "score", "flag", "note", "order"`

func typed(opts ...rt.Opt[renderModel]) []rt.Opt[renderModel] { return opts }

// TestSelectRenderMatchesTypedHandles is the differential proof of spec
// §predicates rule 6: every predicate form of the language, lowered by
// the checker and rendered by the generator, spells the same bytes as
// the typed handles rendered through the runtime, and binds the Go
// expressions the text names in placeholder order.
func TestSelectRenderMatchesTypedHandles(t *testing.T) {
	cases := []struct {
		name  string
		where string
		order string
		typed []rt.Opt[renderModel]
		args  []string
	}{
		{"eq param", "site = :site", "", typed(rSite.Eq("")), []string{"site"}},
		{"ne literal", "day != 3", "", typed(rDay.Ne(0)), []string{"3"}},
		{"lt", "day < :before", "", typed(rDay.Lt(0)), []string{"before"}},
		{"le", "day <= :before", "", typed(rDay.Le(0)), []string{"before"}},
		{"gt", "day > 0", "", typed(rDay.Gt(0)), []string{"0"}},
		{"ge", "day >= :since", "", typed(rDay.Ge(0)), []string{"since"}},
		{"like literal", "site like '%a%'", "", typed(rSite.Like("")), []string{`"%a%"`}},
		{"like param", "site like :pat", "", typed(rSite.Like("")), []string{"pat"}},
		{"not like", "not site like :pat", "", typed(rt.Not(rSite.Like(""))), []string{"pat"}},
		{"is null", "note is null", "", typed(rNote.IsNull()), nil},
		{"is not null", "note is not null", "", typed(rNote.IsNotNull()), nil},
		{"in values", "hits in (1, 2, 3)", "", typed(rHits.In(0, 0, 0)), []string{"1", "2", "3"}},
		{"in list param", "site in :sites", "", typed(rSite.InList(nil)), []string{"rt.JSONList(sites)"}},
		{"in list param int", "day in :days", "", typed(rDay.InList(nil)), []string{"rt.JSONList(days)"}},
		{
			"and chain",
			"site = :site and day = :day and hits > 0", "",
			typed(rt.And(rSite.Eq(""), rDay.Eq(0), rHits.Gt(0))),
			[]string{"site", "day", "0"},
		},
		{
			"or of ands",
			"site = :site and day = :day or hits > 0", "",
			typed(rt.Or(rt.And(rSite.Eq(""), rDay.Eq(0)), rHits.Gt(0))),
			[]string{"site", "day", "0"},
		},
		{
			"user parentheses",
			"site = :site and (day = :day or hits > 0)", "",
			typed(rt.And(rSite.Eq(""), rt.Or(rDay.Eq(0), rHits.Gt(0)))),
			[]string{"site", "day", "0"},
		},
		{
			"grouped chain",
			"(site = :site and day = :day) and hits > 0", "",
			typed(rt.And(rt.And(rSite.Eq(""), rDay.Eq(0)), rHits.Gt(0))),
			[]string{"site", "day", "0"},
		},
		{
			"redundant parentheses",
			"(site = :site) and ((day = :day))", "",
			typed(rt.And(rSite.Eq(""), rDay.Eq(0))),
			[]string{"site", "day"},
		},
		{"not", "not (day = 1 or day = 2)", "", typed(rt.Not(rt.Or(rDay.Eq(0), rDay.Eq(0)))), []string{"1", "2"}},
		{"not not", "not not note is null", "", typed(rt.Not(rt.Not(rNote.IsNull()))), nil},
		{
			"pred reference",
			"at or since", "",
			typed(rt.Or(rt.And(rSite.Eq(""), rDay.Eq(0)), rDay.Ge(0))),
			[]string{"site", "day", "from"},
		},
		{
			"pred reference in chain",
			"at and hits > :min", "",
			typed(rt.And(rt.And(rSite.Eq(""), rDay.Eq(0)), rHits.Gt(0))),
			[]string{"site", "day", "min"},
		},
		{"keyword column", `"order" = :n`, "", typed(rOrder.Eq(0)), []string{"n"}},
		{"literal left", "1 < day", "", typed(rDay.Gt(0)), []string{"1"}},
		{"literal left equal", "'x' = site", "", typed(rSite.Eq("")), []string{`"x"`}},
		{"param left", ":since <= day", "", typed(rDay.Ge(0)), []string{"since"}},
		{"param right", "day >= :since", "", typed(rDay.Ge(0)), []string{"since"}},
		{"float literal", "score = 1.5", "", typed(rScore.Eq(0)), []string{"1.5"}},
		{"fraction literal", "score < 0.5", "", typed(rScore.Lt(0)), []string{"0.5"}},
		{"negative float literal", "score < -2.5", "", typed(rScore.Lt(0)), []string{"-2.5"}},
		{"exponent literal", "score > 1e2", "", typed(rScore.Gt(0)), []string{"100.0"}},
		{"upper exponent literal", "score > 1E2", "", typed(rScore.Gt(0)), []string{"100.0"}},
		{"fraction exponent literal", "score > 1.5E3", "", typed(rScore.Gt(0)), []string{"1500.0"}},
		{"exponent literal against float64", "score >= 1e3", "", typed(rScore.Ge(0)), []string{"1000.0"}},
		{"integer literal against float64", "score >= 3", "", typed(rScore.Ge(0)), []string{"3.0"}},
		{"leading zero literal", "day = 007", "", typed(rDay.Eq(0)), []string{"7"}},
		{"negative leading zero literal", "day = -010", "", typed(rDay.Eq(0)), []string{"-10"}},
		{"negative octal-looking literal", "day = -08", "", typed(rDay.Eq(0)), []string{"-8"}},
		{"negative literal", "day > -1", "", typed(rDay.Gt(0)), []string{"-1"}},
		{"int64 bounds", "day in (9223372036854775807, -9223372036854775808)", "", typed(rDay.In(0, 0)), []string{"9223372036854775807", "-9223372036854775808"}},
		{"bool literal", "flag = true", "", typed(rFlag.Eq(false)), []string{"true"}},
		{"string escape", `site = 'it\'s "x"'`, "", typed(rSite.Eq("")), []string{`"it's \"x\""`}},
		{"two columns", "day < hits", "", typed(rDay.LtCol(rHits)), nil},
		{"two columns flipped operand", "hits != day", "", typed(rHits.NeCol(rDay)), nil},
		{"order asc", "", "(day asc)", typed(rt.OrderBy(rDay.Asc())), nil},
		{"order desc then asc", "", "(day desc, id asc)", typed(rt.OrderBy(rDay.Desc(), rID.Asc())), nil},
		{"order keyword column", "", `("order" desc)`, typed(rt.OrderBy(rOrder.Desc())), nil},
		{"where and order", "site = :site", "(id desc)", typed(rSite.Eq(""), rt.OrderBy(rID.Desc())), []string{"site"}},
	}

	var schema strings.Builder
	schema.WriteString("package db\n\nTable page_views {\n\tid int [pk, increment]\n\tsite string [not null]\n\tday int [not null]\n\thits int [not null]\n\tscore float64 [not null]\n\tflag bool [not null]\n\tnote string\n\t\"order\" int [not null]\n}\n\n")
	schema.WriteString("Pred at { site = :site and day = :day }\nPred since { day >= :from }\n\n")
	for i, tc := range cases {
		fmt.Fprintf(&schema, "Select c%d for page_views", i)
		if tc.where != "" {
			schema.WriteString(" where " + tc.where)
		}
		if tc.order != "" {
			schema.WriteString(" [order: " + tc.order + "]")
		}
		schema.WriteString("\n")
	}
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": schema.String(),
	})
	wantClean(t, diags)
	pkg := pr.Packages["db"]
	bySuffix := map[string]golang.SelectFn{}
	for _, fn := range pkg.SelectFns() {
		bySuffix[fn.MethodSuffix] = fn
	}
	if len(bySuffix) != len(cases) {
		t.Fatalf("%d selects lowered, want %d", len(bySuffix), len(cases))
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, ok := bySuffix[fmt.Sprintf("C%d", i)]
			if !ok {
				t.Fatalf("select c%d not lowered", i)
			}
			got, args, err := golang.SelectSQLFor(pkg.Merged(), pkg.Schema(), fn)
			if err != nil {
				t.Fatal(err)
			}
			want, _, err := rt.SelectRender(`"page_views"`, renderColumns, tc.typed)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("Volt %q renders\n  %s\nthe typed handles\n  %s", tc.where, got, want)
			}
			if len(args)+len(tc.args) > 0 && !reflect.DeepEqual(args, tc.args) {
				t.Errorf("Volt %q binds %q, want %q", tc.where, args, tc.args)
			}
		})
	}
}

func TestSelectInfoCarriesRendererFragments(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": "package db\n\nTable page_views {\n\tid int [pk, increment]\n\tsite string [not null]\n\tday int [not null]\n}\n\n" +
			"Select rows for page_views where site = :site and day in :days [order: (day desc, id asc)]\n",
	})
	wantClean(t, diags)
	si := pr.Packages["db"].Selects[0]
	if want := `("site" = ? AND "day" IN (SELECT value FROM json_each(?)))`; si.WhereSQL != want {
		t.Errorf("WhereSQL = %q, want %q", si.WhereSQL, want)
	}
	if want := []string{"site", "rt.JSONList(days)"}; !reflect.DeepEqual(si.Binds, want) {
		t.Errorf("Binds = %q, want %q", si.Binds, want)
	}
	if want := `"day" DESC, "id" ASC`; si.OrderSQL != want {
		t.Errorf("OrderSQL = %q, want %q", si.OrderSQL, want)
	}
	wantParams := []SelectParam{{GoName: "site", GoType: "string"}, {GoName: "days", GoType: "[]int"}}
	if !reflect.DeepEqual(si.Params, wantParams) {
		t.Errorf("Params = %+v, want %+v", si.Params, wantParams)
	}
}

func TestSelectLiteralOutsideStorageRefused(t *testing.T) {
	cases := []struct{ name, where, want string }{
		{"int overflow", "hits > 9223372036854775808", "literal 9223372036854775808 does not fit int"},
		{"int underflow", "hits < -9223372036854775809", "literal -9223372036854775809 does not fit int"},
		{"float literal against int", "hits = 1.5", "literal 1.5 does not fit int"},
		{"exponent literal against int", "hits = 1e2", "literal 1e2 does not fit int"},
		{"float overflow", "score > 1e400", "literal 1e400 does not fit float64"},
		{"in item overflow", "hits in (1, 9223372036854775808)", "literal 9223372036854775808 does not fit int"},
		{"declared type storage", "cents = 9223372036854775808", "literal 9223372036854775808 does not fit int, the storage type of column \"cents\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := project(t, map[string]string{
				"go.mod": modFile,
				"db/schema.volt": "package db\n\nType money int\n\nTable page_views {\n\tid int [pk, increment]\n\thits int [not null]\n\tscore float64 [not null]\n\tcents money [not null]\n}\n\n" +
					"Select c for page_views where " + tc.where + "\n",
			})
			wantError(t, diags, tc.want)
		})
	}
}
