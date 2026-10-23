package naming

import (
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestSelectRowNameMatchesRowType(t *testing.T) {
	f, diags := parser.ParseFile("rows.volt", "Table users {\n  id int [pk]\n  email string\n  name string [null]\n}\n")
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
		{TableKey: "users", MethodSuffix: "Active"},
		{TableKey: "users", MethodSuffix: "Active", Cols: []string{"id", "email"}},
		{TableKey: "users", MethodSuffix: "Active", Cols: []string{"id", "email"}, SharedType: "UserContact"},
		{TableKey: "users", MethodSuffix: "Public", Excluded: []string{"email"}},
		{TableKey: "users", MethodSuffix: "Public", Excluded: []string{"email"}, SharedType: "UserPublic"},
		{TableKey: "nope", MethodSuffix: "Active"},
	} {
		want, _, wantErr := pl.SelectRowType(fn)
		got, gotErr := pl.SelectRowName(fn)
		if got != want || (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%+v: SelectRowName %q, %v; SelectRowType %q, %v", fn, got, gotErr, want, wantErr)
		}
	}
}
