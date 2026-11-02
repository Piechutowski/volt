package naming

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestCreateCastLeavesOutTheRowidAliasAlone proves a generated create
// casts every field but the rowid alias key (spec §changesets,
// §columns): a plain or autoincrement int key is left out, while a key
// of a declared type stored as int, which Volt treats as an ordinary
// key, and every member of a composite key are cast.
func TestCreateCastLeavesOutTheRowidAliasAlone(t *testing.T) {
	src := "type Level int\n\ntable posts {\n  id    int    [pk]\n  title string\n}\n\ntable pings {\n  id int [pk, autoincrement]\n}\n\ntable tiers {\n  id    Level  [pk]\n  label string\n}\n\ntable post_tags {\n  post_id int    [pk]\n  tag     string [pk]\n}\n"
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
		table, want string
	}{
		{"posts", "title"},
		{"pings", ""},
		{"tiers", "id label"},
		{"post_tags", "post_id tag"},
	} {
		tb, err := pl.Table(c.table)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, fd := range tb.CreateCast() {
			got = append(got, fd.ColName)
		}
		if strings.Join(got, " ") != c.want {
			t.Errorf("%s: a create casts %v, want %q", c.table, got, c.want)
		}
	}
}
