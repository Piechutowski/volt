package naming

import (
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestSelectRowNameMatchesRowType(t *testing.T) {
	pl := usersPlan(t)
	for _, tc := range []struct {
		fn   SelectFn
		name string
	}{
		{SelectFn{TableKey: "users", MethodSuffix: "Active"}, "User"},
		{SelectFn{TableKey: "users", MethodSuffix: "Active", Cols: []string{"id", "email"}}, "User"},
		{SelectFn{TableKey: "users", MethodSuffix: "Active", Cols: []string{"id", "email"}, SharedType: "UserContact"}, "UserContact"},
		{SelectFn{TableKey: "users", MethodSuffix: "Public", Excluded: []string{"email"}}, "UserPublic"},
		{SelectFn{TableKey: "users", MethodSuffix: "Public", Excluded: []string{"email"}, SharedType: "UserPublic"}, "UserPublic"},
		{SelectFn{TableKey: "nope", MethodSuffix: "Active"}, ""},
	} {
		want, _, wantErr := pl.SelectRowType(tc.fn)
		got, gotErr := pl.SelectRowName(tc.fn)
		if got != want || (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%+v: SelectRowName %q, %v; SelectRowType %q, %v", tc.fn, got, gotErr, want, wantErr)
		}
		if got != tc.name {
			t.Errorf("%+v: SelectRowName %q, want %q", tc.fn, got, tc.name)
		}
	}
}

// TestPlanSlicesAreCopies proves a Plan stays immutable when a caller
// writes into the slices it hands out (spec §mapping-to-go).
func TestPlanSlicesAreCopies(t *testing.T) {
	f, diags := parser.ParseFile("two.volt", "Table users {\n  id int [pk]\n}\n\nTable posts {\n  id int [pk]\n}\n")
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
	ts := pl.Tables()
	if len(ts) != 2 {
		t.Fatalf("%d tables planned", len(ts))
	}
	ts[0], ts[1] = ts[1], ts[0]
	if got := pl.Tables()[0].Model; got != "User" {
		t.Errorf("a write into Tables reached the plan: first table %s", got)
	}
	names := pl.CRUDMethodNames()
	first := names[0]
	names[0] = "Changed"
	if got := pl.CRUDMethodNames()[0]; got != first {
		t.Errorf("a write into CRUDMethodNames reached the plan: first name %s", got)
	}
}

func usersPlan(t *testing.T) *Plan {
	t.Helper()
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
	return pl
}
