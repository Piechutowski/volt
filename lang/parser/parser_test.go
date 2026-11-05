package parser

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
)

func mustParse(t *testing.T, src string) *ast.File {
	t.Helper()
	f, diags := ParseFile("test.volt", src)
	if diag.HasErrors(diags) {
		t.Fatalf("ParseFile errors: %v", diags)
	}
	return f
}

func TestTableShape(t *testing.T) {
	f := mustParse(t, `
table core_users [model: "Person"] {
  id int [pk]
  name string
  indexes {
    (id, name) [unique]
  }
}
`)
	if len(f.Decls) != 1 {
		t.Fatalf("decls = %d, want 1", len(f.Decls))
	}
	tb := f.Decls[0].(*ast.Table)
	if tb.Name.Name() != "core_users" {
		t.Errorf("name = %q", tb.Name.Name())
	}
	if tb.Settings.Get("model") == nil {
		t.Error("model setting missing")
	}
	if len(tb.Body) != 3 {
		t.Fatalf("body items = %d, want 3", len(tb.Body))
	}
	col := tb.Body[0].(*ast.Column)
	if col.Settings.Get("pk") == nil {
		t.Error("setting 'pk' missing")
	}
	typ := tb.Body[1].(*ast.Column).Type
	if typ.String() != "string" {
		t.Errorf("type = %q, want string", typ.String())
	}
	ix := tb.Body[2].(*ast.IndexesBlock).Indexes[0]
	if !ix.Composite || len(ix.Key) != 2 {
		t.Errorf("index composite=%v keys=%d", ix.Composite, len(ix.Key))
	}
}

func TestSettingOneIdentifier(t *testing.T) {
	f := mustParse(t, "migration 0001__a [no_transaction] {\n}\n")
	if f.Decls[0].(*ast.Migration).Settings.Get("no_transaction") == nil {
		t.Error("setting no_transaction not read")
	}
	for _, src := range []string{
		"migration 0001__a [no transaction] {\n}\n",
		"ref: a.x > b.y [delete: set null]\n",
		"table t { x int [null unique] }\n",
		"ref: a.x > b.y [\n  delete: set_null\n  update: cascade\n]\n",
	} {
		_, diags := ParseFile("t", src)
		if len(diags) != 1 || !strings.Contains(diags[0].Msg, "expected ',' or ']' in settings list") {
			t.Errorf("%q: diagnostics %v, want one syntax error: a setting name or value is one identifier (§settings-lists)", src, diags)
		}
	}
}

func TestRefForms(t *testing.T) {
	f := mustParse(t, `
ref: a.x > b.y [delete: set_null]
table e { z int [unique, ref: f.w] }
`)
	ref := f.Decls[0].(*ast.Ref)
	if ref.Left.Table.Name() != "a" || ref.Right.Table.Name() != "b" || ref.Settings.Get("delete") == nil {
		t.Errorf("ref parsed wrong: %+v", ref)
	}
	if v, ok := ref.Settings.Get("delete").Value.(*ast.Ident); !ok || v.Name() != "set_null" {
		t.Errorf("delete: value = %#v, want the identifier set_null", ref.Settings.Get("delete").Value)
	}
	col := f.Decls[1].(*ast.Table).Body[0].(*ast.Column)
	ep, ok := col.Settings.Get("ref").Value.(*ast.RefEndpoint)
	if !ok || ep.Table.Name() != "f" || len(ep.Columns) != 1 || ep.Columns[0].Name() != "w" {
		t.Errorf("inline ref parsed wrong: %#v", col.Settings.Get("ref").Value)
	}
}

func TestRefOneSpelling(t *testing.T) {
	for _, src := range []string{
		"ref: a.x < b.y",
		"ref: a.x - b.y",
		"ref: a.x <> b.y",
		"ref fk: a.x > b.y",
		"ref { a.x > b.y }",
		"ref: s.a.x > b.y",
		"table e { z int [ref: > f.w] }",
	} {
		if _, diags := ParseFile("t", src); !diag.HasErrors(diags) {
			t.Errorf("%q must be a syntax error: a relationship is Ref: fk > target, an inline ref is ref: table.column (§relationships-ref)", src)
		}
	}
}

func TestCompositeEndpoint(t *testing.T) {
	f := mustParse(t, "ref: a.(x, y) > b.(p, q)")
	r := f.Decls[0].(*ast.Ref)
	if r.Left.Table.Name() != "a" || len(r.Left.Columns) != 2 {
		t.Errorf("left endpoint = %q cols=%d", r.Left.Table.Name(), len(r.Left.Columns))
	}
}

func TestRecordsRows(t *testing.T) {
	f := mustParse(t, `
table t {
  a int
  records (a, b, c) {
    1, "x", true
    null, null, null
    -5, "v", `+"`now()`"+`
  }
}
`)
	rec := f.Decls[0].(*ast.Table).Body[1].(*ast.Records)
	if len(rec.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rec.Rows))
	}
	for i, want := range []int{3, 3, 3} {
		if got := len(rec.Rows[i].Values); got != want {
			t.Errorf("row %d values = %d, want %d", i, got, want)
		}
	}
	if _, ok := rec.Rows[2].Values[0].(*ast.NegNumber); !ok {
		t.Error("negative number not parsed")
	}
	if _, ok := rec.Rows[2].Values[2].(*ast.FuncExpr); !ok {
		t.Error("expression literal not parsed")
	}
}

func TestRecordsDottedValue(t *testing.T) {
	src := "table t {\n  a string\n  records (a) {\n    status.active\n    \"active\"\n  }\n}\n"
	f, diags := ParseFile("t", src)
	if len(diags) != 1 || !strings.Contains(diags[0].Msg, "a dotted name is not a record value") {
		t.Fatalf("want one dotted-name error, got %v", diags)
	}
	rec := f.Decls[0].(*ast.Table).Body[1].(*ast.Records)
	if last := rec.Rows[len(rec.Rows)-1]; len(last.Values) != 1 {
		t.Errorf("the row after the error was not parsed")
	}
}

func TestRecordsEmptyField(t *testing.T) {
	for _, row := range []string{"1, , 3", ", 2, 3", "1, 2,"} {
		src := "table t {\n  a int\n  records (a, b, c) {\n    " + row + "\n    4, 5, 6\n  }\n}\n"
		f, diags := ParseFile("t", src)
		if len(diags) != 1 {
			t.Errorf("%q: %d diagnostics, want one empty-field error: %v", row, len(diags), diags)
			continue
		}
		rec := f.Decls[0].(*ast.Table).Body[1].(*ast.Records)
		if last := rec.Rows[len(rec.Rows)-1]; len(last.Values) != 3 {
			t.Errorf("%q: the row after the error was not parsed", row)
		}
	}
}

func TestKeywordsAreContextual(t *testing.T) {
	mustParse(t, `
table table {
  note string
  indexes int
  ref string
}
`)
}

func TestErrorRecovery(t *testing.T) {
	f, diags := ParseFile("t", `
table a {
  id int [
  name string
}
table b {
  ] broken
  x int
}
type Ok string
`)
	if !diag.HasErrors(diags) {
		t.Fatal("want errors")
	}
	if len(diags) < 2 {
		t.Errorf("want at least 2 diagnostics from independent sites, got %d: %v", len(diags), diags)
	}
	var types int
	for _, d := range f.Decls {
		if _, ok := d.(*ast.TypeDecl); ok {
			types++
		}
	}
	if types != 1 {
		t.Errorf("recovery lost the trailing Type: decls=%d", len(f.Decls))
	}
}

func TestBodyIncompleteAfterDroppedItem(t *testing.T) {
	f, _ := ParseFile("t", `
table posts {
  id int [pk autoincrement]
}
table users {
  id int [pk]
}
table_partial stamps {
  created int [null unique]
}
migration 0001__a {
  create table posts {
    id int [pk autoincrement]
  }
  rebuild table users {
    id int [pk autoincrement]
  }
  drop table users {
    id int [pk autoincrement]
  }
  create table users {
    id int [pk]
  }
}
`)
	var got []bool
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Table:
			got = append(got, d.Incomplete)
		case *ast.TablePartial:
			got = append(got, d.Incomplete)
		case *ast.Migration:
			for _, op := range d.Up {
				switch op := op.(type) {
				case *ast.CreateTable:
					got = append(got, op.Incomplete)
				case *ast.RebuildTable:
					got = append(got, op.Incomplete)
				case *ast.DropTable:
					got = append(got, op.Incomplete)
				}
			}
		}
	}
	want := []bool{true, false, true, true, true, true, false}
	if len(got) != len(want) {
		t.Fatalf("bodies = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("body %d: Incomplete = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestPartialAST(t *testing.T) {
	f, diags := ParseFile("t", "table ok { id int }\ntable broken {")
	if !diag.HasErrors(diags) {
		t.Fatal("want errors")
	}
	if len(f.Decls) == 0 {
		t.Error("want partial AST with the first table")
	}
}

func TestWrongCaseKeywordIsReadAsTheKeyword(t *testing.T) {
	src := "TABLE t {\n  id int [PK, Default: 1]\n  flag bool [default: TRUE]\n  INDEXES {\n    id\n  }\n}\n\nref: a.x > t.id [Delete: CASCADE]\n"
	f, diags := ParseFile("t.volt", src)
	want := []string{
		"write table, not TABLE",
		"write pk, not PK",
		"write default, not Default",
		"write true, not TRUE",
		"write indexes, not INDEXES",
		"write delete, not Delete",
		"write cascade, not CASCADE",
	}
	if len(diags) != len(want) {
		t.Fatalf("diagnostics %v, want one per misspelled keyword: %v", diags, want)
	}
	for i, d := range diags {
		if !strings.Contains(d.Msg, want[i]) || !strings.HasSuffix(d.Msg, "(§keywords)") {
			t.Errorf("diagnostic %d is %q, want %q citing §keywords", i, d.Msg, want[i])
		}
	}
	tb, ok := f.Decls[0].(*ast.Table)
	if !ok {
		t.Fatalf("TABLE did not read as a Table: %T", f.Decls[0])
	}
	id := tb.Body[0].(*ast.Column)
	if id.Settings.Get("pk") == nil || id.Settings.Get("default") == nil {
		t.Errorf("PK and Default were not read as pk and default: %+v", id.Settings)
	}
	if v := tb.Body[1].(*ast.Column).Settings.Get("default").Value.(*ast.Ident); v.Name() != "true" {
		t.Errorf("TRUE read as %q, want true", v.Name())
	}
	if _, ok := tb.Body[2].(*ast.IndexesBlock); !ok {
		t.Errorf("INDEXES did not read as an Indexes block: %T", tb.Body[2])
	}
	if a := f.Decls[1].(*ast.Ref).Settings.Get("delete"); a == nil || a.Value.(*ast.Ident).Name() != "cascade" {
		t.Errorf("Delete: CASCADE was not read as delete: cascade")
	}
}

func TestWrongCaseKeywordOutsideKeywordPositionIsAName(t *testing.T) {
	f := mustParse(t, "table t {\n  indexes int\n  records string\n  TRUE bool\n}\n")
	tb := f.Decls[0].(*ast.Table)
	for i, want := range []string{"indexes", "records", "TRUE"} {
		if c, ok := tb.Body[i].(*ast.Column); !ok || c.Name.Name() != want {
			t.Errorf("item %d is %T, want the column %s", i, tb.Body[i], want)
		}
	}
}
