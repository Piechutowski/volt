// Package golang generates Go model structs from a checked DBML file: one
// struct per table with its changeset and column handles (spec
// §changesets), one string-typed enum per DBML enum, with db and json
// struct tags. The output is gofmt-canonical by construction and depends
// only on the standard library and the rt runtime package; nullable
// columns are rt.Null[T] values.
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
	// Source is the name of the DBML input, recorded in the header.
	Source string
}

// Generate renders the models file for one checked DBML file: the enums,
// the row structs and their changesets, every type the wire carries.
// The file must be free of check errors; Generate validates only what
// generation itself needs (name mapping, type mapping, collisions).
func Generate(f *ast.File, info *check.Info, opts Options) ([]byte, error) {
	p, perr := planBuild(f, info, nil)
	return modelsGenerate(f, info, p, perr, opts)
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

	enumTypes map[string]string
	out       strings.Builder
	body      strings.Builder
}

func (g *generator) run(p *plan, perr error) error {
	if err := g.enumTypesCollect(); err != nil {
		return err
	}

	if p != nil {
		g.body.Grow(p.fieldCount()*700 + len(p.tables)*1800)
	}
	for _, e := range g.info.Enums {
		if err := g.enumEmit(e.Decl); err != nil {
			return err
		}
	}
	g.typesEmit()
	usedNames := map[string]string{}
	for _, e := range g.info.Enums {
		usedNames[g.enumTypes[e.Key]] = "enum " + e.Decl.Name.String()
	}
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
// type (spec §types).
func (g *generator) typesEmit() {
	if len(g.info.Types) == 0 {
		return
	}
	g.imports["database/sql"] = true
	g.imports["database/sql/driver"] = true
	b := &g.body
	b.WriteString("var (\n")
	for _, td := range g.info.Types {
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
	tbl := t.ti.Decl.Name.String()
	settable := t.settable()

	fmt.Fprintf(b, "// %sColumn is a column handle of %s, the element of a Cast allow list\n// (spec §changesets).\n", m, m)
	fmt.Fprintf(b, "type %sColumn = rt.ColumnOf[%s]\n\n", m, m)
	fmt.Fprintf(b, "// %sParams is a %s write body as the wire carries it (spec §changesets).\n", m, tbl)
	fmt.Fprintf(b, "type %sParams = rt.Params[%s]\n\n", m, m)

	fmt.Fprintf(b, "// Typed column handles of %s: the Cast allow list of a changeset, and\n", m)
	fmt.Fprintf(b, "// the inert predicate, order and assignment builders of the dynamic\n")
	fmt.Fprintf(b, "// query layer. A predicate built here can only enter %s queries;\n", m)
	fmt.Fprintf(b, "// mixing models is a compile error.\n")
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

	fmt.Fprintf(b, "// %sNew starts a changeset for a %s row to create.\n", m, tbl)
	funcLine(b, fmt.Sprintf("func %sNew() %sChangeset", m, m), fmt.Sprintf("return %sChangeset{}", m))
	fmt.Fprintf(b, "// Changeset starts a changeset for an update of r.\n")
	funcLine(b, fmt.Sprintf("func (r %s) Changeset() %sChangeset", m, m), fmt.Sprintf("return %sChangeset{data: r, row: true}", m))

	fmt.Fprintf(b, "// From marks every data column of r but the auto-increment key.\n")
	fmt.Fprintf(b, "func (c %sChangeset) From(r %s) %sChangeset {\n", m, m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tc = c.%sSet(r.%s)\n", f.goField, f.goField)
	}
	b.WriteString("\treturn c\n}\n\n")

	for _, f := range settable {
		if f.nullable && !strings.HasPrefix(f.goType, "[]") {
			fmt.Fprintf(b, "// %sSet sets %s; an invalid rt.Null sets NULL.\n", f.goField, f.colName)
		} else {
			fmt.Fprintf(b, "// %sSet sets %s.\n", f.goField, f.colName)
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
			fmt.Fprintf(b, "\t\t\tif v, ok, err := rt.Field(p, %q, %t, func(r *%s) %s { return r.%s }); err != nil {\n", f.wire, f.nullable, m, f.goType, f.goField)
			fmt.Fprintf(b, "\t\t\t\tc = c.errorAdd(\"cast\", err.Error(), %q)\n", f.colName)
			fmt.Fprintf(b, "\t\t\t} else if ok {\n\t\t\t\tc = c.%sSet(v)\n\t\t\t}\n", f.goField)
		}
		b.WriteString("\t\t}\n\t}\n")
	}
	b.WriteString("\treturn c\n}\n\n")

	fmt.Fprintf(b, "// Changed reports whether col was set.\n")
	fmt.Fprintf(b, "func (c %sChangeset) Changed(col %sColumn) bool {\n\tswitch col.ColumnName() {\n", m, m)
	for i, f := range t.fields {
		fmt.Fprintf(b, "\tcase %q:\n\t\treturn c.set[%d]\n", f.colName, i)
	}
	b.WriteString("\t}\n\treturn false\n}\n\n")

	fmt.Fprintf(b, "// Row is the data with the changes applied.\n")
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

	fmt.Fprintf(b, "// Params is the changes of c as a write body.\n")
	fmt.Fprintf(b, "func (c %sChangeset) Params() %sParams {\n\tvar set []string\n", m, m)
	for _, f := range settable {
		fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tset = append(set, %q)\n\t}\n", t.index(f), f.wire)
	}
	b.WriteString("\treturn rt.ParamsOf(c.Row(), set)\n}\n\n")

	fmt.Fprintf(b, "// ErrorAdd records a field error with check custom on col.\n")
	fmt.Fprintf(b, "func (c %sChangeset) ErrorAdd(col %sColumn, msg string) %sChangeset {\n", m, m, m)
	b.WriteString("\treturn c.errorAdd(\"custom\", msg, col.ColumnName())\n}\n\n")
	fmt.Fprintf(b, "func (c %sChangeset) errorAdd(check, msg string, cols ...string) %sChangeset {\n", m, m)
	b.WriteString("\tc.errs = rt.FieldErrorsAppend(c.errs, rt.FieldError{Check: check, Columns: cols, Message: msg})\n\treturn c\n}\n\n")

	fmt.Fprintf(b, "// Errors lists every field error recorded so far: casts and custom\n")
	fmt.Fprintf(b, "// errors first, then the last Validate's findings.\n")
	fmt.Fprintf(b, "func (c %sChangeset) Errors() []rt.FieldError {\n", m)
	b.WriteString("\treturn rt.FieldErrorsAppend(rt.FieldErrorsAppend(nil, c.errs...), c.valid...)\n}\n\n")

	fmt.Fprintf(b, "// Valid reports whether no field error was recorded.\n")
	funcLine(b, fmt.Sprintf("func (c %sChangeset) Valid() bool", m), "return len(c.errs) == 0 && len(c.valid) == 0")

	fmt.Fprintf(b, "// Err is nil when c is valid and otherwise the *rt.Invalid carrying its\n// field errors.\n")
	fmt.Fprintf(b, "func (c %sChangeset) Err() error {\n\tif c.Valid() {\n\t\treturn nil\n\t}\n", m)
	fmt.Fprintf(b, "\treturn &rt.Invalid{Model: %q, Fields: c.Errors()}\n}\n\n", m)
}

func (g *generator) enumTypesCollect() error {
	g.enumTypes = map[string]string{}
	seen := map[string]string{}
	for _, e := range g.info.Enums {
		name, err := enumTypeName(e.Decl.Name.Schema(), e.Decl.Name.Base())
		if err != nil {
			return fmt.Errorf("enum %s: %w", e.Decl.Name.String(), err)
		}
		if prev, dup := seen[name]; dup {
			return fmt.Errorf("enums %s and %s both map to Go type %s", prev, e.Decl.Name.String(), name)
		}
		seen[name] = e.Decl.Name.String()
		g.enumTypes[e.Key] = name
	}
	return nil
}

func enumTypeName(schema, base string) (string, error) {
	n, err := structName(schema, base)
	if err != nil {
		return "", err
	}
	return "E" + n, nil
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
		if p, ok := d.(*ast.Project); ok && len(p.Notes) > 0 {
			return p.Notes[len(p.Notes)-1].Text.Tok.Val
		}
	}
	return ""
}

func (g *generator) enumEmit(e *ast.Enum) error {
	typeName := g.enumTypes[canonEnumKey(e)]

	fmt.Fprintf(&g.body, "// %s is the %s enum.\n", typeName, e.Name.String())
	if note := enumNote(e); note != "" {
		g.body.WriteString("//\n")
		commentWrite(&g.body, note)
	}
	fmt.Fprintf(&g.body, "type %s string\n\n", typeName)

	g.body.WriteString("const (\n")
	var specs align.Block
	seen := map[string]string{}
	for _, v := range e.Values {
		constName, err := goName(v.Name.Name())
		if err != nil {
			return fmt.Errorf("enum %s value %q: %w", e.Name.String(), v.Name.Name(), err)
		}
		constName = typeName + constName
		if prev, dup := seen[constName]; dup {
			return fmt.Errorf("enum %s: values %q and %q both map to Go constant %s", e.Name.String(), prev, v.Name.Name(), constName)
		}
		seen[constName] = v.Name.Name()
		if note := settingNote(v.Settings); note != "" {
			commentLines(&specs, note)
		}
		specs.Row(constName, typeName, "= "+strconv.Quote(v.Name.Name()))
	}
	specs.WriteTo(&g.body, "\t")
	g.body.WriteString(")\n\n")
	return nil
}

func canonEnumKey(e *ast.Enum) string {
	if e.Name.Schema() == "" {
		return "public." + e.Name.Base()
	}
	return e.Name.String()
}

func (g *generator) tableEmit(ti *check.TableInfo, usedNames map[string]string) error {
	name, err := modelName(ti.Decl)
	if err != nil {
		return fmt.Errorf("table %s: %w", ti.Decl.Name.String(), err)
	}
	if prev, dup := usedNames[name]; dup {
		return fmt.Errorf("table %s and %s both map to Go type %s", ti.Decl.Name.String(), prev, name)
	}
	usedNames[name] = "table " + ti.Decl.Name.String()

	fmt.Fprintf(&g.body, "// %s is a row of the %s table.\n", name, ti.Decl.Name.String())
	if note := tableNote(ti.Decl); note != "" {
		g.body.WriteString("//\n")
		commentWrite(&g.body, note)
	}
	fmt.Fprintf(&g.body, "type %s struct {\n", name)

	pkCols := compositePKColumns(ti)
	fields := map[string]string{}
	var rows align.Block
	for _, cd := range ti.Columns {
		if err := g.fieldEmit(cd, pkCols, fields, &rows); err != nil {
			return fmt.Errorf("table %s: %w", ti.Decl.Name.String(), err)
		}
	}
	rows.WriteTo(&g.body, "\t")
	g.body.WriteString("}\n\n")
	return nil
}

func compositePKColumns(ti *check.TableInfo) map[string]bool {
	out := map[string]bool{}
	for _, ix := range ti.Indexes {
		if ix.Settings.Get("pk") == nil {
			continue
		}
		for _, atom := range ix.Key {
			if id, ok := atom.(*ast.Ident); ok {
				out[id.Name()] = true
			}
		}
	}
	return out
}

func (g *generator) fieldEmit(cd *check.ColumnDef, pkCols map[string]bool, fields map[string]string, rows *align.Block) error {
	col := cd.Col
	fieldName, err := goName(col.Name.Name())
	if err != nil {
		return fmt.Errorf("column %q: %w", col.Name.Name(), err)
	}
	if prev, dup := fields[fieldName]; dup {
		return fmt.Errorf("columns %q and %q both map to Go field %s", prev, col.Name.Name(), fieldName)
	}
	fields[fieldName] = col.Name.Name()

	typ := typeOf(cd.Type, g.enumTypes)
	if typ.imp != "" {
		g.imports[typ.imp] = true
	}

	nullable := isNullable(col) && !pkCols[col.Name.Name()]
	goTypeName := typ.name
	if nullable && !typ.nilable {
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

func isNullable(col *ast.Column) bool {
	s := col.Settings
	if s != nil {
		if s.Get("not null") != nil || s.Get("pk") != nil || s.Get("primary key") != nil || s.Get("increment") != nil {
			return false
		}
	}
	for _, f := range col.LegacyFlags {
		if strings.EqualFold(f.Name(), "pk") {
			return false
		}
	}
	return true
}

func tableNote(t *ast.Table) string {
	for _, item := range t.Body {
		if n, ok := item.(*ast.Note); ok {
			return n.Text.Tok.Val
		}
	}
	return settingNote(t.Settings)
}

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

func enumNote(e *ast.Enum) string {
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
