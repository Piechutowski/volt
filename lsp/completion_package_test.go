package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

const packageTypes = "package db\n\ntype Flag bool\n\ntable users {\n\tid     int [pk]\n\tactive bool\n}\n"

const packageSchema = "package db\n\ntable posts {\n\tid     int [pk]\n\tauthor int\n\tshown  Flag\n}\n"

func TestCompletionReadsThePackage(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod": "module pkgcomplete\n", "db/types.volt": packageTypes, "db/schema.volt": packageSchema,
	})
	for _, tc := range []struct{ from, to, want string }{
		{"\tauthor int\n", "\tauthor int [ref: users.", "active"},
		{"\tauthor int\n", "\tauthor int [ref: ", "users"},
		{"\tauthor int\n", "\tauthor F", "Flag"},
		{"\tshown  Flag\n", "\tshown  Flag [default: ", "true"},
		{"}\n", "}\n\nref: posts.author > users.", "active"},
	} {
		text := strings.Replace(packageSchema, tc.from, tc.to, 1)
		d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
		pos := offsetToLSP(text, strings.Index(text, tc.to)+len(tc.to))
		if items := d.Complete(pos); !hasLabel(items, tc.want) {
			t.Errorf("completion after %q = %v, want %s from the package's other file", tc.to, labels(items), tc.want)
		}
	}
}

func TestCompletionOffersBooleansOnlyAfterAColumn(t *testing.T) {
	text := sample + "\nref: orders.user_id > users.id [default: "
	d := doc(t, text)
	if items := d.Complete(offsetToLSP(text, len(text))); hasLabel(items, "true") {
		t.Errorf("a ref element's settings offer %v, want no boolean", labels(items))
	}
}

func TestVoltCompleteQualifiedTablesNoMatch(t *testing.T) {
	_, _, routes := navProject(t)
	text := strings.Replace(navRoutes, "resources db.posts", "resources db.zz", 1)
	d := NewDocument("file://"+routes, text)
	if items := d.Complete(offsetToLSP(text, strings.Index(text, "db.zz")+len("db.zz"))); len(items) != 0 {
		t.Errorf("completion after db.zz = %v, want none", labels(items))
	}
}
