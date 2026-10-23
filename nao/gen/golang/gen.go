// Package golang renders a checked package's naming plan (package naming)
// as the Go data files: the models with their changesets and column
// handles (spec §changesets), the static and dynamic query layers, the
// selects, the validators and the migrations. The output is
// gofmt-canonical by construction and imports only the standard library,
// the rt runtime package and the packages of declared types.
package golang

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
)

const rtImport = "github.com/Piechutowski/volt/nao/rt"

// Options configures generation.
type Options struct {
	// Package is the package name of the generated file.
	Package string
	// Source is the name of the schema input, recorded in the header.
	Source string
}

// Generate renders the models file for one checked schema file: the row
// structs and their changesets, every type the wire carries.
// The file must be free of check errors; Generate validates only what
// generation itself needs (name mapping, type mapping, collisions).
func Generate(f *ast.File, info *check.Info, opts Options) ([]byte, error) {
	return Models(naming.PlanBuild(f, info), opts)
}

// Models renders the models file (volt_models.go) for the planned package;
// a package-scope name the file would declare twice is an error, as it
// is for Dyn (spec §changesets).
func Models(pl *naming.Plan, opts Options) ([]byte, error) {
	if opts.Package == "" {
		return nil, fmt.Errorf("no package name")
	}
	if err := pl.Err(); err != nil {
		return nil, err
	}
	g := &generator{f: pl.File(), info: pl.Info(), opts: opts, imports: map[string]bool{}}
	g.run(pl.Tables())
	if err := pl.NameCollisionErr(); err != nil {
		return nil, err
	}
	return align.Finish(g.out.String()), nil
}

type generator struct {
	f       *ast.File
	info    *check.Info
	opts    Options
	imports map[string]bool

	out  strings.Builder
	body strings.Builder
}

func fieldCount(tables []*naming.Table) int {
	n := 0
	for _, t := range tables {
		n += len(t.Fields)
	}
	return n
}

func (g *generator) run(tables []*naming.Table) {
	g.body.Grow(fieldCount(tables)*700 + len(tables)*4000)
	g.typesEmit()
	for _, t := range tables {
		g.tableEmit(t)
		g.changesetEmit(t)
	}
	g.header()
	g.out.Grow(g.body.Len())
	g.out.WriteString(g.body.String())
}

// typesEmit asserts the Scanner and Valuer contract of every declared
// type but a kinded one, whose Go declaration spells its storage type
// and which scans through that type and binds by kind (spec §types).
func (g *generator) typesEmit() {
	var asserted []*check.TypeInfo
	for _, td := range g.info.Types {
		if !td.Kinded {
			asserted = append(asserted, td)
		}
	}
	if len(asserted) == 0 {
		return
	}
	g.imports["database/sql"] = true
	g.imports["database/sql/driver"] = true
	b := &g.body
	b.WriteString("var (\n")
	for _, td := range asserted {
		if td.Path != "" {
			g.imports[naming.ImportKey(td.Key[:strings.IndexByte(td.Key, '.')], td.Path)] = true
		}
		fmt.Fprintf(b, "\t_ sql.Scanner   = (*%s)(nil)\n", td.Key)
		fmt.Fprintf(b, "\t_ driver.Valuer = *new(%s)\n", td.Key)
	}
	b.WriteString(")\n\n")
}

// changesetEmit writes one table's changeset surface after its row
// struct (spec §changesets): the column and params aliases, the column
// handles, the changeset type and its methods.
func (g *generator) changesetEmit(t *naming.Table) {
	if len(t.Fields) == 0 {
		return
	}
	g.imports[rtImport] = true
	b := &g.body
	m := t.Model
	tbl := t.Info.Decl.Name.Name()
	settable := t.Settable()

	fmt.Fprintf(b, "// %sColumn is a column handle of %s, the element of a Cast allow list\n// (spec §changesets).\n", m, m)
	fmt.Fprintf(b, "type %sColumn = rt.ColumnOf[%s]\n\n", m, m)
	fmt.Fprintf(b, "// %sParams is a %s write body as the wire carries it (spec §changesets).\n", m, tbl)
	fmt.Fprintf(b, "type %sParams = rt.Params[%s]\n\n", m, m)

	fmt.Fprintf(b, "// Typed column handles of %s (spec §changesets): the Cast allow list of\n", m)
	fmt.Fprintf(b, "// a changeset, and the inert predicate, order and assignment builders\n")
	fmt.Fprintf(b, "// of the dynamic query layer. A predicate built here can only enter %s\n", m)
	fmt.Fprintf(b, "// queries; mixing models is a compile error.\n")
	b.WriteString("var (\n")
	var handles align.Block
	for _, f := range t.Fields {
		if f.Nullable {
			handles.Row(m+f.GoField, fmt.Sprintf("= rt.NullColumn[%s, %s]{Column: rt.Column[%s, %s]{Name: %q}}",
				m, f.BaseType, m, f.BaseType, f.ColName))
			continue
		}
		handles.Row(m+f.GoField, fmt.Sprintf("= rt.Column[%s, %s]{Name: %q}", m, f.BaseType, f.ColName))
	}
	handles.WriteTo(b, "\t")
	b.WriteString(")\n\n")

	fmt.Fprintf(b, "// %sChangeset carries a %s row, the changes proposed to it and the\n", m, tbl)
	fmt.Fprintf(b, "// errors found so far (spec §changesets). It is a value: every method\n")
	fmt.Fprintf(b, "// returns a new changeset and leaves its receiver as it was.\n")
	fmt.Fprintf(b, "type %sChangeset struct {\n", m)
	var fields align.Block
	fields.Row("data", m)
	fields.Row("vals", m)
	fields.Row("row", "bool")
	fields.Row("set", fmt.Sprintf("[%d]bool", len(t.Fields)))
	fields.Row("errs", "[]rt.FieldError")
	fields.Row("valid", "[]rt.FieldError")
	fields.WriteTo(b, "\t")
	b.WriteString("}\n\n")

	fmt.Fprintf(b, "// %sNew starts a changeset for a %s row to create, the schema's literal\n// defaults in place (spec §changesets).\n", m, tbl)
	newBody := fmt.Sprintf("return %sChangeset{}", m)
	if seed := defaultsGo(t); seed != "" {
		newBody = fmt.Sprintf("return %sChangeset{data: %s%s}", m, m, seed)
	}
	funcLine(b, fmt.Sprintf("func %sNew() %sChangeset", m, m), newBody)
	fmt.Fprintf(b, "// Changeset starts a changeset for an update of r, nothing set (spec\n// §changesets).\n")
	funcLine(b, fmt.Sprintf("func (r %s) Changeset() %sChangeset", m, m), fmt.Sprintf("return %sChangeset{data: r, row: true}", m))

	fmt.Fprintf(b, "// From marks every data column of r but the auto-increment key (spec\n// §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) From(r %s) %sChangeset {\n", m, m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tc = c.%sSet(r.%s)\n", f.GoField, f.GoField)
	}
	b.WriteString("\treturn c\n}\n\n")

	for _, f := range settable {
		switch {
		case f.Nullable:
			fmt.Fprintf(b, "// %sSet sets %s and marks it; an invalid rt.Null sets NULL (spec\n// §changesets).\n", f.GoField, f.ColName)
		case f.PlainBytes():
			fmt.Fprintf(b, "// %sSet sets %s and marks it; nil is empty bytes (spec §columns).\n", f.GoField, f.ColName)
		default:
			fmt.Fprintf(b, "// %sSet sets %s and marks it (spec §changesets).\n", f.GoField, f.ColName)
		}
		fmt.Fprintf(b, "func (c %sChangeset) %sSet(v %s) %sChangeset {\n", m, f.GoField, f.GoType, m)
		fmt.Fprintf(b, "\tc.vals.%s, c.set[%d] = v, true\n\treturn c\n}\n\n", f.GoField, t.Index(f))
	}

	fmt.Fprintf(b, "// Cast sets every column of allow that p carries, recording a cast\n")
	fmt.Fprintf(b, "// error for a value that does not convert and an unknown_field error\n")
	fmt.Fprintf(b, "// for a key that names no column (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Cast(p %sParams, allow ...%sColumn) %sChangeset {\n", m, m, m, m)
	wires := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		wires[i] = strconv.Quote(f.Wire)
	}
	fmt.Fprintf(b, "\tc.errs = rt.FieldErrorsAppend(c.errs, rt.UnknownFields(p, %s)...)\n", strings.Join(wires, ", "))
	if len(settable) > 0 {
		b.WriteString("\tfor _, voltCol := range allow {\n\t\tswitch voltCol.ColumnName() {\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\t\tcase %q:\n", f.ColName)
			fmt.Fprintf(b, "\t\t\tif voltV, voltOK, voltErr := rt.Field(p, %q, %q, %t, func(voltR *%s) %s { return voltR.%s }); voltErr != nil {\n", f.Wire, f.Col.Type.String(), f.Nullable, m, f.GoType, f.GoField)
			fmt.Fprintf(b, "\t\t\t\tc = c.errorAdd(\"cast\", voltErr.Error(), %q)\n", f.ColName)
			fmt.Fprintf(b, "\t\t\t} else if voltOK {\n\t\t\t\tc = c.%sSet(voltV)\n\t\t\t}\n", f.GoField)
		}
		b.WriteString("\t\t}\n\t}\n")
	}
	b.WriteString("\treturn c\n}\n\n")

	fmt.Fprintf(b, "// Changed reports whether col is marked (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Changed(col %sColumn) bool {\n\tswitch col.ColumnName() {\n", m, m)
	for i, f := range t.Fields {
		fmt.Fprintf(b, "\tcase %q:\n\t\treturn c.set[%d]\n", f.ColName, i)
	}
	b.WriteString("\t}\n\treturn false\n}\n\n")

	fmt.Fprintf(b, "// Row is the data with the marked values applied (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Row() %s {\n", m, m)
	if len(settable) == 0 {
		b.WriteString("\treturn c.data\n}\n\n")
	} else {
		b.WriteString("\tvoltR := c.data\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tvoltR.%s = c.vals.%s\n\t}\n", t.Index(f), f.GoField, f.GoField)
		}
		b.WriteString("\treturn voltR\n}\n\n")
	}

	fmt.Fprintf(b, "// Params is the marked columns of c as a write body, each under its\n// wire name (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Params() %sParams {\n\tvar voltMarks []rt.Mark\n", m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tvoltMarks = append(voltMarks, rt.Mark{Wire: %q, Value: c.vals.%s})\n\t}\n", t.Index(f), f.Wire, f.GoField)
	}
	b.WriteString("\treturn rt.ParamsOf(c.Row(), voltMarks...)\n}\n\n")

	fmt.Fprintf(b, "// ErrorAdd records a field error with check custom on col (spec\n// §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) ErrorAdd(col %sColumn, msg string) %sChangeset {\n", m, m, m)
	b.WriteString("\treturn c.errorAdd(\"custom\", msg, col.ColumnName())\n}\n\n")
	fmt.Fprintf(b, "func (c %sChangeset) errorAdd(check, msg string, cols ...string) %sChangeset {\n", m, m)
	b.WriteString("\tc.errs = rt.FieldErrorsAppend(c.errs, rt.FieldError{Check: check, Columns: cols, Message: msg})\n\treturn c\n}\n\n")

	fmt.Fprintf(b, "// Errors lists every field error recorded so far (spec §changesets): casts\n")
	fmt.Fprintf(b, "// and custom errors first, then the last Validate's findings.\n")
	fmt.Fprintf(b, "func (c %sChangeset) Errors() []rt.FieldError {\n", m)
	b.WriteString("\treturn rt.FieldErrorsAppend(rt.FieldErrorsAppend(nil, c.errs...), c.valid...)\n}\n\n")

	fmt.Fprintf(b, "// Valid reports whether no field error was recorded (spec §changesets).\n")
	funcLine(b, fmt.Sprintf("func (c %sChangeset) Valid() bool", m), "return len(c.errs) == 0 && len(c.valid) == 0")

	fmt.Fprintf(b, "// Err is nil when c is valid and otherwise the *rt.Invalid carrying its\n// field errors (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Err() error {\n\tif c.Valid() {\n\t\treturn nil\n\t}\n", m)
	fmt.Fprintf(b, "\treturn &rt.Invalid{Model: %q, Fields: c.Errors()}\n}\n\n", m)
}

func (g *generator) header() {
	fmt.Fprintf(&g.out, "// Code generated by volt gen from %s. DO NOT EDIT.\n\n", g.opts.Source)

	fmt.Fprintf(&g.out, "// Package %s contains data models generated from %s.\n", g.opts.Package, g.opts.Source)
	if note := g.projectNote(); note != "" {
		g.out.WriteString("//\n")
		commentWrite(&g.out, note)
	}
	fmt.Fprintf(&g.out, "package %s\n\n", g.opts.Package)

	if len(g.imports) > 0 {
		g.out.WriteString("import (\n")
		for _, p := range importKeysSort(g.imports) {
			fmt.Fprintf(&g.out, "\t%s\n", importDecl(p))
		}
		g.out.WriteString(")\n\n")
	}
}

func (g *generator) projectNote() string {
	for _, d := range g.f.Decls {
		if p, ok := d.(*ast.Project); ok {
			return naming.SettingNote(p.Settings)
		}
	}
	return ""
}

func (g *generator) tableEmit(t *naming.Table) {
	tbl := t.Info.Decl.Name.Name()
	fmt.Fprintf(&g.body, "// %s is a row of the %s table.\n", t.Model, tbl)
	if note := naming.SettingNote(t.Info.Decl.Settings); note != "" {
		g.body.WriteString("//\n")
		commentWrite(&g.body, note)
	}
	fmt.Fprintf(&g.body, "type %s struct {\n", t.Model)
	var rows align.Block
	for _, f := range t.Fields {
		if f.Imp != "" {
			g.imports[f.Imp] = true
		}
		if f.Nullable {
			g.imports[rtImport] = true
		}
		if note := naming.SettingNote(f.Col.Settings); note != "" {
			commentLines(&rows, note)
		}
		rows.Row(f.GoField, f.GoType, "`"+f.Tag+"`")
	}
	rows.WriteTo(&g.body, "\t")
	g.body.WriteString("}\n\n")
}

func commentWrite(b *strings.Builder, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString("//\n")
			continue
		}
		b.WriteString("// " + line + "\n")
	}
}

func commentLines(rows *align.Block, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			rows.Line("//")
			continue
		}
		rows.Line("// " + line)
	}
}

func defaultsGo(t *naming.Table) string {
	var parts []string
	for _, f := range t.Fields {
		if expr, ok := defaultGo(f); ok {
			parts = append(parts, f.GoField+": "+expr)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// defaultGo renders the literal default a <Model>New seeds for one column
// (spec §changesets): a Go constant of the column's type, wrapped
// in rt.Some when nullable, converted to a kinded declared type there so
// the wrapper takes the column's type. A zero literal on a non-nullable
// column is the zero value and needs no seed; on a nullable column it is
// a value, not NULL, and is seeded. A declared type that is not kinded
// gets no seed: its default is the database's.
func defaultGo(f *naming.Field) (string, bool) {
	s := f.Col.Settings.Get("default")
	if s == nil || (f.Declared && !f.Kinded) || f.Increment {
		return "", false
	}
	var expr string
	var zero bool
	switch v := s.Value.(type) {
	case *ast.BasicLit:
		if v.Tok.Kind == token.STRING {
			expr, zero = strconv.Quote(v.Tok.Val), v.Tok.Val == ""
		} else {
			expr, zero = numberGo(v.Tok.Val, f.Storage)
		}
	case *ast.NegNumber:
		expr, zero = numberGo("-"+v.Num.Tok.Val, f.Storage)
	case *ast.Ident:
		switch {
		case v.Name() == "true":
			expr = "true"
		case v.Name() == "false":
			expr, zero = "false", true
		default:
			return "", false
		}
	default:
		return "", false
	}
	if expr == "" || (zero && !f.Nullable) {
		return "", false
	}
	if f.Nullable {
		if f.Kinded {
			expr = f.BaseType + "(" + expr + ")"
		}
		expr = "rt.Some(" + expr + ")"
	}
	return expr, true
}

func numberGo(lit, storage string) (expr string, zero bool) {
	switch storage {
	case "int":
		n, err := strconv.ParseInt(lit, 10, 64)
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(n, 10), n == 0
	case "float64":
		x, err := strconv.ParseFloat(lit, 64)
		if err != nil {
			return "", false
		}
		expr = strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(expr, ".eE") {
			expr += ".0"
		}
		return expr, x == 0
	}
	return "", false
}
