package sqlite

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestTableDDLOneToOneAndActions pins the shape rendering of an inline
// - ref and of delete: and update: beside a ref: (spec §migrations
// rules 4 and 5): the same DDL as a schema Table of the same shape,
// which testdata/inline_one_to_one.volt pins as a golden.
func TestTableDDLOneToOneAndActions(t *testing.T) {
	cases := []struct {
		name, body string
		want       string
		indexes    []string
	}{
		{
			name: "a plain one-to-one adds UNIQUE",
			body: "    id      int [pk, increment]\n    user_id int [ref: - users.id]\n",
			want: "CREATE TABLE profiles (\n  id INTEGER PRIMARY KEY,\n  user_id INTEGER NOT NULL,\n  FOREIGN KEY (user_id) REFERENCES users (id),\n  UNIQUE (user_id)\n)",
		},
		{
			name:    "a unique index of the body stands in for UNIQUE",
			body:    "    id      int [pk, increment]\n    user_id int [ref: - users.id]\n\n    indexes {\n      user_id [unique]\n    }\n",
			want:    "CREATE TABLE profiles (\n  id INTEGER PRIMARY KEY,\n  user_id INTEGER NOT NULL,\n  FOREIGN KEY (user_id) REFERENCES users (id)\n)",
			indexes: []string{"CREATE UNIQUE INDEX idx_profiles_user_id ON profiles (user_id)"},
		},
		{
			name: "a unique column takes no second UNIQUE",
			body: "    id      int [pk, increment]\n    user_id int [unique, ref: - users.id]\n",
			want: "CREATE TABLE profiles (\n  id INTEGER PRIMARY KEY,\n  user_id INTEGER NOT NULL UNIQUE,\n  FOREIGN KEY (user_id) REFERENCES users (id)\n)",
		},
		{
			name: "delete: and update: follow the reference",
			body: "    id      int [pk, increment]\n    user_id int [null, ref: > users.id, delete: cascade, update: set null]\n",
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

func TestColumnDDLActions(t *testing.T) {
	cases := []struct {
		column, want string
		unique       bool
	}{
		{"user_id int [null, ref: > users.id, delete: set default]", "user_id INTEGER REFERENCES users (id) ON DELETE SET DEFAULT", false},
		{"user_id int [null, ref: > users.id, update: no action]", "user_id INTEGER REFERENCES users (id) ON UPDATE NO ACTION", false},
		{"twin_id int [null, ref: - users.id]", "twin_id INTEGER REFERENCES users (id)", true},
	}
	for _, tc := range cases {
		body := parsedBody(t, "create table profiles {\n    "+tc.column+"\n  }")
		got, unique, err := ColumnDDL(body[0].(*ast.Column))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want || unique != tc.unique {
			t.Errorf("ColumnDDL(%s) = %q, unique %v; want %q, %v", tc.column, got, unique, tc.want, tc.unique)
		}
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
