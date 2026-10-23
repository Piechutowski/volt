// Package golang generates Go model structs from a checked schema file: one
// struct per table with its changeset and column handles (spec
// §changesets), with db and json struct tags. The output is
// gofmt-canonical by construction and depends only on the standard
// library, the rt runtime package and the packages of declared types;
// nullable columns are rt.Null[T] values.
package golang

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
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
	return PlanBuild(f, info).Models(opts)
}

func modelsGenerate(f *ast.File, info *check.Info, p *plan, perr error, opts Options) ([]byte, error) {
	if opts.Package == "" {
		return nil, fmt.Errorf("no package name")
	}
	g := &generator{f: f, info: info, opts: opts, imports: map[string]bool{}}
	if err := g.run(p, perr); err != nil {
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

func (g *generator) run(p *plan, perr error) error {
	if p != nil {
		g.body.Grow(p.fieldCount()*700 + len(p.tables)*4000)
	}
	g.typesEmit()
	usedNames := map[string]string{}
	for i, ti := range g.info.Tables {
		if err := g.tableEmit(ti, usedNames); err != nil {
			return err
		}
		if p != nil {
			g.changesetEmit(p.tables[i])
		}
	}
	if perr != nil {
		return perr
	}

	g.header()
	g.out.Grow(g.body.Len())
	g.out.WriteString(g.body.String())
	return nil
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
			g.imports[importKeyOf(td.Key[:strings.IndexByte(td.Key, '.')], td.Path)] = true
		}
		fmt.Fprintf(b, "\t_ sql.Scanner   = (*%s)(nil)\n", td.Key)
		fmt.Fprintf(b, "\t_ driver.Valuer = *new(%s)\n", td.Key)
	}
	b.WriteString(")\n\n")
}

// changesetEmit writes one table's changeset surface after its row
// struct (spec §changesets): the column and params aliases, the column
// handles, the changeset type and its methods.
func (g *generator) changesetEmit(t *tableModel) {
	if len(t.fields) == 0 {
		return
	}
	g.imports[rtImport] = true
	b := &g.body
	m := t.model
	tbl := t.ti.Decl.Name.Name()
	settable := t.settable()

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
	for _, f := range t.fields {
		if f.nullable {
			handles.Row(m+f.goField, fmt.Sprintf("= rt.NullColumn[%s, %s]{Column: rt.Column[%s, %s]{Name: %q}}",
				m, f.baseType, m, f.baseType, f.colName))
			continue
		}
		handles.Row(m+f.goField, fmt.Sprintf("= rt.Column[%s, %s]{Name: %q}", m, f.baseType, f.colName))
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
	fields.Row("set", fmt.Sprintf("[%d]bool", len(t.fields)))
	fields.Row("errs", "[]rt.FieldError")
	fields.Row("valid", "[]rt.FieldError")
	fields.WriteTo(b, "\t")
	b.WriteString("}\n\n")

	fmt.Fprintf(b, "// %sNew starts a changeset for a %s row to create, the schema's literal\n// defaults in place (spec §changesets).\n", m, tbl)
	newBody := fmt.Sprintf("return %sChangeset{}", m)
	if seed := t.defaultsGo(); seed != "" {
		newBody = fmt.Sprintf("return %sChangeset{data: %s%s}", m, m, seed)
	}
	funcLine(b, fmt.Sprintf("func %sNew() %sChangeset", m, m), newBody)
	fmt.Fprintf(b, "// Changeset starts a changeset for an update of r, nothing set (spec\n// §changesets).\n")
	funcLine(b, fmt.Sprintf("func (r %s) Changeset() %sChangeset", m, m), fmt.Sprintf("return %sChangeset{data: r, row: true}", m))

	fmt.Fprintf(b, "// From marks every data column of r but the auto-increment key (spec\n// §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) From(r %s) %sChangeset {\n", m, m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tc = c.%sSet(r.%s)\n", f.goField, f.goField)
	}
	b.WriteString("\treturn c\n}\n\n")

	for _, f := range settable {
		switch {
		case f.nullable:
			fmt.Fprintf(b, "// %sSet sets %s and marks it; an invalid rt.Null sets NULL (spec\n// §changesets).\n", f.goField, f.colName)
		case f.plainBytes():
			fmt.Fprintf(b, "// %sSet sets %s and marks it; nil is empty bytes (spec §columns).\n", f.goField, f.colName)
		default:
			fmt.Fprintf(b, "// %sSet sets %s and marks it (spec §changesets).\n", f.goField, f.colName)
		}
		fmt.Fprintf(b, "func (c %sChangeset) %sSet(v %s) %sChangeset {\n", m, f.goField, f.goType, m)
		fmt.Fprintf(b, "\tc.vals.%s, c.set[%d] = v, true\n\treturn c\n}\n\n", f.goField, t.index(f))
	}

	fmt.Fprintf(b, "// Cast sets every column of allow that p carries, recording a cast\n")
	fmt.Fprintf(b, "// error for a value that does not convert and an unknown_field error\n")
	fmt.Fprintf(b, "// for a key that names no column (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Cast(p %sParams, allow ...%sColumn) %sChangeset {\n", m, m, m, m)
	wires := make([]string, len(t.fields))
	for i, f := range t.fields {
		wires[i] = strconv.Quote(f.wire)
	}
	fmt.Fprintf(b, "\tc.errs = rt.FieldErrorsAppend(c.errs, rt.UnknownFields(p, %s)...)\n", strings.Join(wires, ", "))
	if len(settable) > 0 {
		b.WriteString("\tfor _, col := range allow {\n\t\tswitch col.ColumnName() {\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\t\tcase %q:\n", f.colName)
			fmt.Fprintf(b, "\t\t\tif v, ok, err := rt.Field(p, %q, %q, %t, func(r *%s) %s { return r.%s }); err != nil {\n", f.wire, f.col.Type.String(), f.nullable, m, f.goType, f.goField)
			fmt.Fprintf(b, "\t\t\t\tc = c.errorAdd(\"cast\", err.Error(), %q)\n", f.colName)
			fmt.Fprintf(b, "\t\t\t} else if ok {\n\t\t\t\tc = c.%sSet(v)\n\t\t\t}\n", f.goField)
		}
		b.WriteString("\t\t}\n\t}\n")
	}
	b.WriteString("\treturn c\n}\n\n")

	fmt.Fprintf(b, "// Changed reports whether col is marked (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Changed(col %sColumn) bool {\n\tswitch col.ColumnName() {\n", m, m)
	for i, f := range t.fields {
		fmt.Fprintf(b, "\tcase %q:\n\t\treturn c.set[%d]\n", f.colName, i)
	}
	b.WriteString("\t}\n\treturn false\n}\n\n")

	fmt.Fprintf(b, "// Row is the data with the marked values applied (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Row() %s {\n", m, m)
	if len(settable) == 0 {
		b.WriteString("\treturn c.data\n}\n\n")
	} else {
		b.WriteString("\tr := c.data\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tr.%s = c.vals.%s\n\t}\n", t.index(f), f.goField, f.goField)
		}
		b.WriteString("\treturn r\n}\n\n")
	}

	fmt.Fprintf(b, "// Params is the marked columns of c as a write body, each under its\n// wire name (spec §changesets).\n")
	fmt.Fprintf(b, "func (c %sChangeset) Params() %sParams {\n\tvar marks []rt.Mark\n", m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tmarks = append(marks, rt.Mark{Wire: %q, Value: c.vals.%s})\n\t}\n", t.index(f), f.wire, f.goField)
	}
	b.WriteString("\treturn rt.ParamsOf(c.Row(), marks...)\n}\n\n")

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
	fmt.Fprintf(&g.out, "// Code generated by nao gen from %s; DO NOT EDIT.\n\n", g.opts.Source)

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
			return settingNote(p.Settings)
		}
	}
	return ""
}

func (g *generator) tableEmit(ti *check.TableInfo, usedNames map[string]string) error {
	name, err := modelName(ti.Decl)
	if err != nil {
		return fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
	}
	if prev, dup := usedNames[name]; dup {
		return fmt.Errorf("table %s and %s both map to Go type %s", ti.Decl.Name.Name(), prev, name)
	}
	usedNames[name] = "table " + ti.Decl.Name.Name()

	fmt.Fprintf(&g.body, "// %s is a row of the %s table.\n", name, ti.Decl.Name.Name())
	if note := tableNote(ti.Decl); note != "" {
		g.body.WriteString("//\n")
		commentWrite(&g.body, note)
	}
	fmt.Fprintf(&g.body, "type %s struct {\n", name)

	fields := map[string]string{}
	var rows align.Block
	for _, cd := range ti.Columns {
		if err := g.fieldEmit(cd, fields, &rows); err != nil {
			return fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
		}
	}
	rows.WriteTo(&g.body, "\t")
	g.body.WriteString("}\n\n")
	return nil
}

func (g *generator) fieldEmit(cd *check.ColumnDef, fields map[string]string, rows *align.Block) error {
	col := cd.Col
	fieldName, err := goName(col.Name.Name())
	if err != nil {
		return fmt.Errorf("column %q: %w", col.Name.Name(), err)
	}
	if prev, dup := fields[fieldName]; dup {
		return fmt.Errorf("columns %q and %q both map to Go field %s", prev, col.Name.Name(), fieldName)
	}
	fields[fieldName] = col.Name.Name()

	typ := typeOf(cd.Type)
	if typ.imp != "" {
		g.imports[typ.imp] = true
	}

	goTypeName := typ.name
	if cd.Nullable {
		goTypeName = "rt.Null[" + goTypeName + "]"
		g.imports[rtImport] = true
	}

	if note := settingNote(col.Settings); note != "" {
		commentLines(rows, note)
	}
	rows.Row(fieldName, goTypeName, "`"+fieldTag(col)+"`")
	return nil
}

func fieldTag(col *ast.Column) string {
	name := col.Name.Name()
	jsonPair := `json:"` + name + `"`
	var extra []string
	if col.Settings != nil {
		for _, s := range col.Settings.Settings {
			if s.Name != "tag" {
				continue
			}
			lit, ok := s.Value.(*ast.BasicLit)
			if !ok || lit.Tok.Kind != token.STRING {
				continue
			}
			pair := lit.Tok.Val
			if key, _, ok := strings.Cut(pair, ":"); ok && key == "json" {
				jsonPair = pair
				continue
			}
			extra = append(extra, pair)
		}
	}
	parts := append([]string{`db:"` + name + `"`, jsonPair}, extra...)
	return strings.Join(parts, " ")
}

func tableNote(t *ast.Table) string { return settingNote(t.Settings) }

func settingNote(sl *ast.SettingList) string {
	s := sl.Get("note")
	if s == nil {
		return ""
	}
	if lit, ok := s.Value.(*ast.BasicLit); ok {
		return lit.Tok.Val
	}
	return ""
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

func (t *tableModel) defaultsGo() string {
	var parts []string
	for _, f := range t.fields {
		if expr, ok := f.defaultGo(); ok {
			parts = append(parts, f.goField+": "+expr)
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
func (f *fieldPlan) defaultGo() (string, bool) {
	s := f.col.Settings.Get("default")
	if s == nil || (f.declared && !f.kinded) || f.increment {
		return "", false
	}
	var expr string
	var zero bool
	switch v := s.Value.(type) {
	case *ast.BasicLit:
		if v.Tok.Kind == token.STRING {
			expr, zero = strconv.Quote(v.Tok.Val), v.Tok.Val == ""
		} else {
			expr, zero = numberGo(v.Tok.Val, f.storage)
		}
	case *ast.NegNumber:
		expr, zero = numberGo("-"+v.Num.Tok.Val, f.storage)
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
	if expr == "" || (zero && !f.nullable) {
		return "", false
	}
	if f.nullable {
		if f.kinded {
			expr = f.baseType + "(" + expr + ")"
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
