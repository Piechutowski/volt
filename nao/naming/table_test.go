package naming

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestCreateCastLeavesOutTheAssignedKeyAlone proves a generated create
// casts every field but the database-assigned key (spec §changesets,
// §columns): a plain or autoincrement int key is left out, while a key
// of a declared type stored as int, a shared key that is also a
// foreign key, by a ref element or an inline ref:, and every member of
// a composite key are cast, and only the left-out key is Assigned, the
// mark From skips.
func TestCreateCastLeavesOutTheAssignedKeyAlone(t *testing.T) {
	src := "type Level int\n\ntable posts {\n  id    int    [pk]\n  title string\n}\n\ntable pings {\n  id int [pk, autoincrement]\n}\n\ntable tiers {\n  id    Level  [pk]\n  label string\n}\n\ntable post_tags {\n  post_id int    [pk]\n  tag     string [pk]\n}\n\ntable post_stats {\n  post_id int [pk]\n  views   int\n}\n\ntable post_covers {\n  post_id int    [pk, ref: posts.id]\n  url     string\n}\n\nref: post_stats.post_id > posts.id\n"
	f, diags := parser.ParseFile("keys.volt", src)
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
	for _, c := range []struct {
		table, want, assigned string
	}{
		{"posts", "title", "id"},
		{"pings", "", "id"},
		{"tiers", "id label", ""},
		{"post_tags", "post_id tag", ""},
		{"post_stats", "post_id views", ""},
		{"post_covers", "post_id url", ""},
	} {
		tb, err := pl.Table(c.table)
		if err != nil {
			t.Fatal(err)
		}
		var got, assigned []string
		for _, fd := range tb.CreateCast() {
			got = append(got, fd.ColName)
		}
		for _, fd := range tb.Fields {
			if fd.Assigned {
				assigned = append(assigned, fd.ColName)
			}
		}
		if strings.Join(got, " ") != c.want {
			t.Errorf("%s: a create casts %v, want %q", c.table, got, c.want)
		}
		if strings.Join(assigned, " ") != c.assigned {
			t.Errorf("%s: the assigned key is %v, want %q", c.table, assigned, c.assigned)
		}
	}
}

// TestPlanMemoFollowsTheAssignedKey proves the plan memo keys a table's
// model on its database-assigned key as well as its checked table
// (spec §columns): a ref element added beside the same table
// declarations turns the rowid alias into a foreign key while the check
// memo answers the table unchanged, and the model is rebuilt with it.
func TestPlanMemoFollowsTheAssignedKey(t *testing.T) {
	src := "table users {\n  id int [pk]\n}\n\ntable profiles {\n  user_id int    [pk]\n  bio     string\n}\n\nref: profiles.user_id > users.id\n"
	full, diags := parser.ParseFile("keys.volt", src)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	bare := &ast.File{Name: full.Name, Decls: full.Decls[:2:2], EOF: full.EOF}
	memo := &check.Memo{}
	plans := &PlanMemo{}
	var marks []bool
	var models []*Table
	for _, f := range []*ast.File{bare, full} {
		info, semDiags := check.FileMemo(f, memo)
		if len(semDiags) > 0 {
			t.Fatal(semDiags)
		}
		pl := PlanBuildMemo(f, info, plans)
		tb, err := pl.Table("profiles")
		if err != nil {
			t.Fatal(err)
		}
		marks = append(marks, tb.Field("user_id").Assigned)
		models = append(models, tb)
	}
	if memo.Hits != 2 {
		t.Fatalf("the check memo answered %d tables, want both: the tables did not change", memo.Hits)
	}
	if !marks[0] || marks[1] || models[0] == models[1] {
		t.Errorf("profiles.user_id assigned without and with the ref element: %v, want [true false] from a rebuilt model", marks)
	}
}
