package rt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGenConstructorsMatchTypedHandles(t *testing.T) {
	userName := Column[m, string]{Name: "name"}
	parent := Column[m, int64]{Name: "parent_id"}
	cases := []struct {
		name  string
		typed Pred[m]
		gen   func() (GenPred, error)
	}{
		{"eq", email.Eq("a"), func() (GenPred, error) { return PredCmp("email", "=", "a") }},
		{"ne", email.Ne("a"), func() (GenPred, error) { return PredCmp("email", "!=", "a") }},
		{"lt", id.Lt(4), func() (GenPred, error) { return PredCmp("id", "<", int64(4)) }},
		{"le", id.Le(4), func() (GenPred, error) { return PredCmp("id", "<=", int64(4)) }},
		{"gt", id.Gt(4), func() (GenPred, error) { return PredCmp("id", ">", int64(4)) }},
		{"ge", id.Ge(4), func() (GenPred, error) { return PredCmp("id", ">=", int64(4)) }},
		{"eqcol", email.EqCol(userName), func() (GenPred, error) { return PredColCmp("email", "=", "name") }},
		{"necol", email.NeCol(userName), func() (GenPred, error) { return PredColCmp("email", "!=", "name") }},
		{"ltcol", id.LtCol(parent), func() (GenPred, error) { return PredColCmp("id", "<", "parent_id") }},
		{"in", id.In(1, 2), func() (GenPred, error) { return PredIn("id", int64(1), int64(2)), nil }},
		{"in empty", id.In(), func() (GenPred, error) { return PredIn("id"), nil }},
		{"inlist", id.InList([]int64{1, 2}), func() (GenPred, error) { return PredInList("id", "[1,2]"), nil }},
		{"like", email.Like("%a"), func() (GenPred, error) { return PredLike("email", "%a"), nil }},
		{"isnull", bio.IsNull(), func() (GenPred, error) { return PredNull("bio"), nil }},
		{"isnotnull", bio.IsNotNull(), func() (GenPred, error) { return PredNotNull("bio"), nil }},
		{
			"nested",
			And(Or(id.Lt(1), id.Gt(9)), Not(email.Eq("a"))),
			func() (GenPred, error) {
				lt, _ := PredCmp("id", "<", int64(1))
				gt, _ := PredCmp("id", ">", int64(9))
				eq, _ := PredCmp("email", "=", "a")
				return And(Or(lt, gt), Not(eq)), nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := tc.gen()
			if err != nil {
				t.Fatal(err)
			}
			typedSQL, typedArgs, err := SelectRender(`"users"`, `"id"`, []Opt[m]{tc.typed})
			if err != nil {
				t.Fatal(err)
			}
			genSQL, genArgs, err := SelectRender(`"users"`, `"id"`, []Opt[Gen]{g})
			if err != nil {
				t.Fatal(err)
			}
			if typedSQL != genSQL {
				t.Errorf("typed handle renders %q, the generator's view %q", typedSQL, genSQL)
			}
			if len(typedArgs)+len(genArgs) > 0 && !reflect.DeepEqual(typedArgs, genArgs) {
				t.Errorf("typed handle binds %#v, the generator's view %#v", typedArgs, genArgs)
			}
		})
	}
}

func TestGenOrderTermMatchesTypedHandles(t *testing.T) {
	typed := OrderRender([]Order[m]{email.Asc(), id.Desc()})
	gen := OrderRender([]GenOrder{OrderTerm("email", false), OrderTerm("id", true)})
	if want := `"email" ASC, "id" DESC`; typed != want || gen != want {
		t.Errorf("typed %q, generator's view %q, want %q", typed, gen, want)
	}
}

func TestGenCmpRefusesOtherSpellings(t *testing.T) {
	for _, op := range []string{"<>", "==", "=<", "LIKE", "IS", "", " ="} {
		if _, err := PredCmp("email", op, "a"); err == nil {
			t.Errorf("PredCmp accepted %q", op)
		}
		if _, err := PredColCmp("email", op, "name"); err == nil {
			t.Errorf("PredColCmp accepted %q", op)
		}
	}
	for _, op := range []string{"=", "!=", "<", "<=", ">", ">="} {
		if _, err := PredCmp("email", op, "a"); err != nil {
			t.Errorf("PredCmp(%q): %v", op, err)
		}
		if _, err := PredColCmp("email", op, "name"); err != nil {
			t.Errorf("PredColCmp(%q): %v", op, err)
		}
	}
}

func TestRendererSpellsOneInequality(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "<>") {
				t.Errorf("%s: %s spells <>; the renderer's inequality is != (spec §predicates)", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
	}
}
