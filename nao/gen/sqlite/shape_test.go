package sqlite

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestTableDDLOneToOneAndActions pins the shape rendering of a
// one-to-one, unique plus a ref:, of a composite key and an index
// column, and of delete: and update: beside a ref: (spec §migrations
// rules 4 and 5, §relationships-ref, §indexes): the same DDL as a schema
// Table of the same shape, which testdata/inline_one_to_one.volt pins as
// a golden. A unique or index column is a CREATE INDEX after the table,
// never an inline UNIQUE.
func TestTableDDLOneToOneAndActions(t *testing.T) {
	cases := []struct {
		name, body string
		want       string
		indexes    []string
	}{
		{
			name:    "a one-to-one is a unique column with a ref, and its uniqueness a unique index",
			body:    "    id      int [pk, increment]\n    user_id int [unique, ref: users.id]\n",
			want:    "CREATE TABLE profiles (\n  id INTEGER PRIMARY KEY,\n  user_id INTEGER NOT NULL,\n  FOREIGN KEY (user_id) REFERENCES users (id)\n)",
			indexes: []string{"CREATE UNIQUE INDEX idx_profiles_user_id ON profiles (user_id)"},
		},
		{
			name:    "pk columns form one composite key in declaration order, and index is a plain index",
			body:    "    user_id int    [pk, ref: users.id]\n    label   string [index]\n    slot    int    [pk]\n\n    Indexes {\n      (label, slot) [unique]\n    }\n",
			want:    "CREATE TABLE profiles (\n  user_id INTEGER NOT NULL,\n  label TEXT NOT NULL,\n  slot INTEGER NOT NULL,\n  PRIMARY KEY (user_id, slot),\n  FOREIGN KEY (user_id) REFERENCES users (id)\n)",
			indexes: []string{"CREATE INDEX idx_profiles_label ON profiles (label)", "CREATE UNIQUE INDEX idx_profiles_label_slot ON profiles (label, slot)"},
		},
		{
			name: "delete: and update: follow the reference",
			body: "    id      int [pk, increment]\n    user_id int [null, ref: users.id, delete: cascade, update: set_null]\n",
			want: "CREATE TABLE profiles (\n  id INTEGER PRIMARY KEY,\n  user_id INTEGER,\n  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE SET NULL\n)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := parsedBody(t, "create table profiles {\n"+tc.body+"  }")
			got, err := TableDDL("profiles", body)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("TableDDL:\n%s\nwant:\n%s", got, tc.want)
			}
			indexes, err := IndexesDDL("profiles", body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(indexes, "\n") != strings.Join(tc.indexes, "\n") {
				t.Errorf("IndexesDDL: %q, want %q", indexes, tc.indexes)
			}
		})
	}
}

// TestColumnDDLActions pins an added column's definition (spec
// §migrations): its referential actions follow the reference,
// and a unique or index setting is not part of it, since the add's
// CREATE INDEX carries it.
func TestColumnDDLActions(t *testing.T) {
	cases := []struct {
		column, want string
	}{
		{"user_id int [null, ref: users.id, delete: set_default]", "user_id INTEGER REFERENCES users (id) ON DELETE SET DEFAULT"},
		{"user_id int [null, ref: users.id, update: restrict]", "user_id INTEGER REFERENCES users (id) ON UPDATE RESTRICT"},
		{"twin_id int [null, unique, ref: users.id]", "twin_id INTEGER REFERENCES users (id)"},
		{"label string [index, default: \"\"]", "label TEXT NOT NULL DEFAULT ''"},
	}
	for _, tc := range cases {
		body := parsedBody(t, "create table profiles {\n    "+tc.column+"\n  }")
		got, err := ColumnDDL(body[0].(*ast.Column))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("ColumnDDL(%s) = %q; want %q", tc.column, got, tc.want)
		}
	}
}

// TestIndexesDDLRefusesOneNameTwice pins the generator's own refusal of
// two indexes of one name (spec §indexes), which the checker reports
// first on checked input: a column's index and an Indexes entry that
// derive one name.
func TestIndexesDDLRefusesOneNameTwice(t *testing.T) {
	body := parsedBody(t, "create table profiles {\n    email string [unique]\n    Indexes {\n      email [where: `email != ''`]\n    }\n  }")
	_, err := IndexesDDL("profiles", body)
	if err == nil || !strings.Contains(err.Error(), `duplicate index name "idx_profiles_email"`) {
		t.Errorf("IndexesDDL: want the duplicate name refused, got %v", err)
	}
}

func parsedBody(t *testing.T, op string) []ast.TableItem {
	t.Helper()
	f, diags := parser.ParseFile("t.volt", "package db\n\nMigration 0001__shape {\n  "+op+"\n}\n")
	if diag.HasErrors(diags) {
		t.Fatalf("test input must parse: %v", diags)
	}
	for _, d := range f.Decls {
		if m, ok := d.(*ast.Migration); ok {
			return m.Up[0].(*ast.CreateTable).Body
		}
	}
	t.Fatal("no migration parsed")
	return nil
}
