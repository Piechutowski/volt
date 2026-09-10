// Package golang generates Go model structs from a checked DBML file — one
// struct per table, one string-typed enum per DBML enum, the params
// structs its CRUD takes (D77), with db and json struct tags.
//
// DBML notes become Go doc comments at the corresponding level: the Project
// note becomes the package comment, a table note the struct's doc comment,
// a column note the field's doc comment, and enum / enum-value notes the
// doc comments of the generated type and constants. Documentation is
// written once, in the schema.
//
// The output is gofmt-canonical by construction (D75): aligned blocks
// are laid out by gen/align exactly as gofmt's tabwriter would, and the
// golden tests prove gofmt is the identity on every generated file.
// Generated code depends only on the standard library and the rt runtime
// package (D03). Nullable columns (no "not null", not part of a primary
// key) are rt.Null[T] values (D13), except types where nil already
// expresses NULL ([]byte, json.RawMessage).
package golang

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
)

// rtImport is the runtime support package generated code may depend on
// (the only non-stdlib import, decision D03).
const rtImport = "github.com/Piechutowski/volt/nao/rt"

// Options configures generation.
type Options struct {
	// Package is the package name of the generated file.
	Package string
	// Source is the name of the DBML input, recorded in the header.
	Source string
}

// Generate renders the models file for one checked DBML file: the enums,
// the row structs and the params structs — every type the wire carries
// (D77). The file must be free of check errors; Generate validates only
// what generation itself needs (name mapping, type mapping, collisions).
func Generate(f *ast.File, info *check.Info, opts Options) ([]byte, error) {
	p, perr := planBuild(f, info)
	return modelsGenerate(f, info, p, perr, opts)
}

// modelsGenerate is Generate over an already built plan (D74), whose
// tables carry the params structs' field plans; perr is the plan's
// build error, reported after the models' own checks have had their say.
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

	enumTypes map[string]string // canonical enum key -> Go type name
	out       strings.Builder
	body      strings.Builder
}

func (g *generator) run(p *plan, perr error) error {
	if err := g.enumTypesCollect(); err != nil {
		return err
	}

	// Body first: emitting fields discovers which imports are needed.
	for _, e := range g.info.Enums {
		if err := g.enumEmit(e.Decl); err != nil {
			return err
		}
	}
	usedNames := map[string]string{} // Go type name -> DBML origin, across enums and structs
	for _, e := range g.info.Enums {
		usedNames[g.enumTypes[e.Key]] = "enum " + e.Decl.Name.String()
	}
	for _, ti := range g.info.Tables {
		if err := g.tableEmit(ti, usedNames); err != nil {
			return err
		}
	}
	// The params structs follow the models (D77): they are the types a
	// client shares with the server, so they live in the models file.
	// Their fields are the plan's, the same the queries bind.
	if perr != nil {
		return perr
	}
	for _, t := range p.tables {
		g.paramsEmit(t)
	}

	g.header()
	g.out.WriteString(g.body.String())
	return nil
}

// paramsEmit renders the CreateParams and UpdateParams structs of one
// table, when its CRUD takes them (CRUD-4, CRUD-5).
func (g *generator) paramsEmit(t *tableModel) {
	if len(t.fields) == 0 {
		return // a columnless table has no queryable shape
	}
	b := &g.body
	if fields := t.createFields(); len(fields) > 0 {
		fmt.Fprintf(b, "// %sCreateParams are the caller-supplied columns of %sCreate. The\n", t.model, t.model)
		fmt.Fprintf(b, "// auto-increment key and defaulted columns are the database's job (D16).\n")
		fmt.Fprintf(b, "type %sCreateParams struct {\n", t.model)
		paramFieldsWrite(b, fields)
		b.WriteString("}\n\n")
	}
	if len(t.pk) > 0 && len(t.nonPK()) > 0 {
		fmt.Fprintf(b, "// %sUpdateParams are the data columns of %sUpdate: every column\n// outside the primary key.\n", t.model, t.model)
		fmt.Fprintf(b, "type %sUpdateParams struct {\n", t.model)
		paramFieldsWrite(b, t.nonPK())
		b.WriteString("}\n\n")
	}
}

// paramFieldsWrite renders the fields of a params struct, aligned as
// gofmt would, each column's note as its doc comment.
func paramFieldsWrite(b *strings.Builder, fields []*fieldPlan) {
	var rows align.Block
	for _, f := range fields {
		if note := settingNote(f.col.Settings); note != "" {
			commentLines(&rows, note)
		}
		rows.Row(f.goField, f.goType, "`"+f.tag+"`")
	}
	rows.WriteTo(b, "\t")
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

// enumTypeName derives the Go type of an enum: structName with an "E"
// prefix (D11, edited with D29). The prefix keeps enum types out of the
// flat handle namespace: 'Table orders { status order_status }' — the
// idiomatic enum pattern — mints the handle OrderStatus and the enum
// type EOrderStatus instead of colliding.
func enumTypeName(schema, base string) (string, error) {
	n, err := structName(schema, base)
	if err != nil {
		return "", err
	}
	return "E" + n, nil
}

/* ===== header: generated-code marker, package comment, imports ===== */

func (g *generator) header() {
	// The marker matches the official convention (golang.org/s/generatedcode):
	// ^// Code generated .* DO NOT EDIT\.$
	fmt.Fprintf(&g.out, "// Code generated by nao gen from %s; DO NOT EDIT.\n\n", g.opts.Source)

	// Project note (spec §6.11) becomes the package comment.
	fmt.Fprintf(&g.out, "// Package %s contains data models generated from %s.\n", g.opts.Package, g.opts.Source)
	if note := g.projectNote(); note != "" {
		g.out.WriteString("//\n")
		commentWrite(&g.out, note)
	}
	fmt.Fprintf(&g.out, "package %s\n\n", g.opts.Package)

	if len(g.imports) > 0 {
		paths := make([]string, 0, len(g.imports))
		for p := range g.imports {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		g.out.WriteString("import (\n")
		for _, p := range paths {
			fmt.Fprintf(&g.out, "\t%q\n", p)
		}
		g.out.WriteString(")\n\n")
	}
}

func (g *generator) projectNote() string {
	for _, d := range g.f.Decls {
		if p, ok := d.(*ast.Project); ok && len(p.Notes) > 0 {
			// tools keep one project note; take the last, like "last wins"
			return p.Notes[len(p.Notes)-1].Text.Tok.Val
		}
	}
	return ""
}

/* ===== enums ===== */

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

/* ===== tables ===== */

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

// compositePKColumns collects columns covered by a [pk] index (spec §6.5.3):
// primary-key membership implies NOT NULL in SQL even without the setting.
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

	typ, err := typeResolve(col.Type.Name.Schema(), col.Type.Name.Base(), g.enumTypes)
	if err != nil {
		return fmt.Errorf("column %q: %w", col.Name.Name(), err)
	}
	if typ.imp != "" {
		g.imports[typ.imp] = true
	}

	nullable := isNullable(col) && !pkCols[col.Name.Name()]
	goTypeName := typ.name
	if nullable && !typ.nilable {
		// D13: value semantics for NULL — rt.Null[T], never a pointer
		goTypeName = "rt.Null[" + goTypeName + "]"
		g.imports[rtImport] = true
	}

	if note := settingNote(col.Settings); note != "" {
		commentLines(rows, note)
	}
	rows.Row(fieldName, goTypeName, "`"+fieldTag(col)+"`")
	return nil
}

// fieldTag assembles a column's struct tag (Appendix A.5): the db scan
// contract, then the json default — the value or null, nothing omitted
// (D13) — unless a [tag:] with key json replaces it, then every other
// [tag:] passthrough verbatim in declaration order (D60).
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

// isNullable applies spec §6.3.2 plus SQL semantics: a column is NOT NULL
// when marked not null, or when it is (part of) a primary key or
// auto-increment, which imply NOT NULL in every SQL dialect.
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

/* ===== notes to comments ===== */

// tableNote returns the table's note: the body Note definition wins over
// the [note: '...'] setting when both are present.
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
	// enums carry notes only on values (spec §6.8); reserved for future
	// body-note support
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

// commentLines adds a note to an aligned block as doc-comment lines; a
// comment ends the block's alignment run, as it does under gofmt.
func commentLines(rows *align.Block, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			rows.Line("//")
			continue
		}
		rows.Line("// " + line)
	}
}
