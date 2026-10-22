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
Table core_users [note: 'people'] {
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
	if tb.Settings.Get("note") == nil {
		t.Error("note setting missing")
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
	f := mustParse(t, "Migration 0001__a [No_Transaction] {\n}\n")
	if f.Decls[0].(*ast.Migration).Settings.Get("no_transaction") == nil {
		t.Error("setting 'No_Transaction' not canonicalized to no_transaction")
	}
	for _, src := range []string{
		"Migration 0001__a [no transaction] {\n}\n",
		"Ref: a.x > b.y [delete: set null]\n",
		"Table t { x int [null unique] }\n",
		"Ref: a.x > b.y [\n  delete: set_null\n  update: cascade\n]\n",
	} {
		_, diags := ParseFile("t", src)
		if len(diags) != 1 || !strings.Contains(diags[0].Msg, "expected ',' or ']' in settings list") {
			t.Errorf("%q: diagnostics %v, want one syntax error: a setting name or value is one identifier (§settings-lists)", src, diags)
		}
	}
}

func TestRefForms(t *testing.T) {
	f := mustParse(t, `
Ref: a.x > b.y [delete: set_null]
Table e { z int [unique, ref: f.w] }
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
		"Ref: a.x < b.y",
		"Ref: a.x - b.y",
		"Ref: a.x <> b.y",
		"Ref fk: a.x > b.y",
		"Ref { a.x > b.y }",
		"Ref: s.a.x > b.y",
		"Table e { z int [ref: > f.w] }",
	} {
		if _, diags := ParseFile("t", src); !diag.HasErrors(diags) {
			t.Errorf("%q must be a syntax error: a relationship is Ref: fk > target, an inline ref is ref: table.column (§relationships-ref)", src)
		}
	}
}

func TestCompositeEndpoint(t *testing.T) {
	f := mustParse(t, "Ref: a.(x, y) > b.(p, q)")
	r := f.Decls[0].(*ast.Ref)
	if r.Left.Table.Name() != "a" || len(r.Left.Columns) != 2 {
		t.Errorf("left endpoint = %q cols=%d", r.Left.Table.Name(), len(r.Left.Columns))
	}
}

func TestRecordsRows(t *testing.T) {
	f := mustParse(t, `
Table t {
  a int
  records (a, b, c) {
    1, 'x', true
    null, null, null
    -5, 'v', `+"`now()`"+`
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
	src := "Table t {\n  a string\n  records (a) {\n    status.active\n    'active'\n  }\n}\n"
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
		src := "Table t {\n  a int\n  records (a, b, c) {\n    " + row + "\n    4, 5, 6\n  }\n}\n"
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
Table table {
  note string
  indexes int
  ref string
}
`)
}

func TestErrorRecovery(t *testing.T) {
	f, diags := ParseFile("t", `
Table a {
  id int [
  name string
}
Table b {
  ] broken
  x int
}
Type Ok string
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
Table posts {
  id int [pk increment]
}
Table users {
  id int [pk]
}
TablePartial stamps {
  created int [null unique]
}
Migration 0001__a {
  create table posts {
    id int [pk increment]
  }
  rebuild table users {
    id int [pk increment]
  }
  drop table users {
    id int [pk increment]
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
	f, diags := ParseFile("t", "Table ok { id int }\nTable broken {")
	if !diag.HasErrors(diags) {
		t.Fatal("want errors")
	}
	if len(f.Decls) == 0 {
		t.Error("want partial AST with the first table")
	}
}
