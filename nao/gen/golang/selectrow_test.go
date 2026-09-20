package golang

import (
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestSelectRowNameMatchesRowType proves the name a select's row type
// carries is the one SelectRowType names, for every shape a select
// takes, without the fields being built (D105).
func TestSelectRowNameMatchesRowType(t *testing.T) {
	f, diags := parser.ParseFile("rows.dbml", "Table users {\n  id int [pk]\n  email text [not null]\n  name text\n}\n")
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	info, semDiags := check.File(f)
	if len(semDiags) > 0 {
		t.Fatal(semDiags)
	}
	pl := PlanBuild(f, info)
	if pl.Err() != nil {
		t.Fatal(pl.Err())
	}
	for _, fn := range []SelectFn{
		{TableKey: "public.users", MethodSuffix: "Active"},
		{TableKey: "public.users", MethodSuffix: "Active", Cols: []string{"id", "email"}},
		{TableKey: "public.users", MethodSuffix: "Active", Cols: []string{"id", "email"}, SharedType: "UserContact"},
		{TableKey: "public.users", MethodSuffix: "Public", Excluded: []string{"email"}},
		{TableKey: "public.users", MethodSuffix: "Public", Excluded: []string{"email"}, SharedType: "UserPublic"},
		{TableKey: "public.nope", MethodSuffix: "Active"},
	} {
		want, _, wantErr := pl.SelectRowType(fn)
		got, gotErr := pl.SelectRowName(fn)
		if got != want || (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%+v: SelectRowName %q, %v; SelectRowType %q, %v", fn, got, gotErr, want, wantErr)
		}
	}
}
