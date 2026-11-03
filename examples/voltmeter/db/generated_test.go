package db_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

// TestNotesBecomeComments proves every note reaches the generated files
// as the spec says (§notes; §notes-become-doc-comments;
// §mapping-to-sqlite-ddl): a table's note is its model's doc comment, a
// multi-line note keeps its lines, an escaped quote comes out as a
// quote, a column's note is its field's doc comment, a partial's column
// carries its note into every table that injects it, and each note is a
// comment above its table or column in volt_schema.sql.
func TestNotesBecomeComments(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "volt_models.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	types, fields := map[string]string{}, map[string]string{}
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, spec := range g.Specs {
			ts := spec.(*ast.TypeSpec)
			types[ts.Name.Name] = g.Doc.Text()
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fd := range st.Fields.List {
				for _, n := range fd.Names {
					fields[ts.Name.Name+"."+n.Name] = fd.Doc.Text()
				}
			}
		}
	}
	for name, want := range map[string]string{
		"User":       "A team member who reads and annotates runs.\n",
		"Tag":        "A label a run can carry, such as \"v0.3\" or an experiment.\n",
		"Run":        "One collection run: the sweep, the keystroke benchmark and the\nlatency samples of one commit on one machine. Its id is never reused,\nsince exported reports cite it.\n",
		"Commit":     "A commit of Volt's history whose lines were counted.\n",
		"MsGenerate": "The generate phase of the sweep at one size.\n",
	} {
		if types[name] != want {
			t.Errorf("doc comment of %s = %q, want %q", name, types[name], want)
		}
	}
	for name, want := range map[string]string{
		"User.Email":        "Sign-in name, one per member\n",
		"User.CreatedAt":    "When the row was stored, by the database clock\n",
		"Run.Commit":        "The measured commit, lower-case hex\n",
		"MsLoad.WallMs":     "Best wall time over the repetitions\n",
		"MsGenerate.WallMs": "Best wall time over the repetitions\n",
		"Budget.State":      "breached once a run went over\n",
		"User.Name":         "",
	} {
		if fields[name] != want {
			t.Errorf("doc comment of %s = %q, want %q", name, fields[name], want)
		}
	}

	ddl, err := os.ReadFile("volt_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(ddl), "\n")
	above := func(line string) string {
		t.Helper()
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), line) })
		if i < 1 {
			t.Fatalf("volt_schema.sql has no line %q", line)
		}
		return strings.TrimSpace(lines[i-1])
	}
	for line, want := range map[string]string{
		"CREATE TABLE users (":    "-- A team member who reads and annotates runs.",
		"CREATE TABLE tags (":     `-- A label a run can carry, such as "v0.3" or an experiment.`,
		"CREATE TABLE runs (":     "-- since exported reports cite it.",
		"email TEXT NOT NULL,":    "-- Sign-in name, one per member",
		`"commit" TEXT NOT NULL,`: "-- The measured commit, lower-case hex",
	} {
		if got := above(line); got != want {
			t.Errorf("the line above %q in volt_schema.sql = %q, want %q", line, got, want)
		}
	}
}

// TestPartialsInjectInSourceOrder proves a table's columns are its
// injected and direct columns concatenated in source order (spec
// §partial-injection-order) for the shapes Voltmeter has: users, runs,
// machines and breaches end with the stamped partial's created_at
// because they inject it last, and a phase table is the measured
// partial's columns in the partial's order, both in the generated
// model's fields and in the table the schema's DDL creates. Each table
// injects one partial, last or alone, so a generator appending every
// injection after the direct columns would pass too; lang/check's
// TestPartialInjectionKeepsSourceOrder proves the other shapes.
func TestPartialsInjectInSourceOrder(t *testing.T) {
	fieldsOf := func(v any) []string {
		var out []string
		for _, f := range reflect.VisibleFields(reflect.TypeOf(v)) {
			out = append(out, f.Tag.Get("db"))
		}
		return out
	}
	d := schemaDB(t)
	columnsOf := func(table string) []string {
		t.Helper()
		rows, err := d.Pool().QueryContext(context.Background(), `SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			out = append(out, n)
		}
		return out
	}
	for _, c := range []struct {
		table string
		model any
		want  []string
	}{
		{"users", db.User{}, []string{"id", "email", "name", "created_at"}},
		{"runs", db.Run{}, []string{"id", "commit", "dirty", "machine_id", "tag", "baseline_id", "note", "created_at"}},
		{"machines", db.Machine{}, []string{"id", "cpu", "cores", "memory_gb", "os", "arch", "go", "created_at"}},
		{"ms_vet", db.MsVet{}, []string{"run_id", "tables", "wall_ms", "alloc_mb", "heap_mb", "allocs", "output_mb", "source_kb"}},
	} {
		if got := fieldsOf(c.model); !slices.Equal(got, c.want) {
			t.Errorf("fields of the %s model = %v, want %v", c.table, got, c.want)
		}
		if got := columnsOf(c.table); !slices.Equal(got, c.want) {
			t.Errorf("columns volt_schema.sql gives %s = %v, want %v", c.table, got, c.want)
		}
	}
}

// TestStructTagsNameEveryColumn proves every model field carries the
// default tag pair, db and json both the column's name, that a JSON
// document carries every column with NULL as null and nothing omitted,
// and that a struct derivative copies each kept field's tag unchanged
// (spec §struct-tags).
func TestStructTagsNameEveryColumn(t *testing.T) {
	for _, model := range []any{db.User{}, db.Run{}, db.Machine{}, db.Breach{}, db.MsLoad{}} {
		typ := reflect.TypeOf(model)
		for _, f := range reflect.VisibleFields(typ) {
			col := f.Tag.Get("db")
			if col == "" || f.Tag != reflect.StructTag(`db:"`+col+`" json:"`+col+`"`) {
				t.Errorf("%s.%s is tagged %q, want the default db and json pair", typ.Name(), f.Name, f.Tag)
			}
		}
	}
	b, err := json.Marshal(db.Run{})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, f := range reflect.VisibleFields(reflect.TypeOf(db.Run{})) {
		if _, ok := doc[f.Tag.Get("db")]; !ok {
			t.Errorf("a zero run's JSON %s omits %s", b, f.Tag.Get("db"))
		}
	}
	for _, col := range []string{"tag", "baseline_id", "note"} {
		if string(doc[col]) != "null" {
			t.Errorf("a zero run's %s renders %s, want null", col, doc[col])
		}
	}
	model := reflect.TypeOf(db.MsLoad{})
	for _, f := range reflect.VisibleFields(reflect.TypeOf(db.MsLoadLeanRow{})) {
		src, ok := model.FieldByName(f.Name)
		if !ok || src.Tag != f.Tag || src.Type != f.Type {
			t.Errorf("MsLoadLeanRow.%s %s %q does not copy the model's field", f.Name, f.Type, f.Tag)
		}
	}
}
