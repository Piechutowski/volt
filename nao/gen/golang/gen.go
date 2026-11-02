// Package golang renders a checked package's naming plan (package naming)
// as the Go data files: the models with their changesets and column
// handles (spec §changesets), the static and dynamic query layers, the
// selects, the validators and the migrations. The output is
// gofmt-canonical by construction, carries no comment but the header and
// the notes (spec §notes-become-doc-comments), and imports only the
// standard library, the rt runtime package and the packages of declared
// types.
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
	g.body.Grow(fieldCount(tables)*500 + len(tables)*2000)
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
// handles, the desc, the changeset type with its typed methods, and the
// functions the desc carries.
func (g *generator) changesetEmit(t *naming.Table) {
	if len(t.Fields) == 0 {
		return
	}
	g.imports[rtImport] = true
	b := &g.body
	m, n := t.Model, &t.Names
	settable := t.Settable()

	fmt.Fprintf(b, "type %s = rt.ColumnOf[%s]\n\n", n.Column, m)
	fmt.Fprintf(b, "type %s = rt.Params[%s]\n\n", n.Params, m)

	b.WriteString("var (\n")
	var handles align.Block
	for _, f := range t.Fields {
		if f.Nullable {
			handles.Row(t.Handle(f), fmt.Sprintf("= rt.NullColumn[%s, %s]{Column: rt.Column[%s, %s]{Name: %q}}",
				m, f.BaseType, m, f.BaseType, f.ColName))
			continue
		}
		handles.Row(t.Handle(f), fmt.Sprintf("= rt.Column[%s, %s]{Name: %q}", m, f.BaseType, f.ColName))
	}
	handles.WriteTo(b, "\t")
	b.WriteString(")\n\n")

	g.descEmit(t)

	fmt.Fprintf(b, "type %s struct{ rt.Changeset[%s] }\n\n", n.Changeset, m)
	funcLine(b, fmt.Sprintf("func %s() %s", n.New, n.Changeset), fmt.Sprintf("return %s{rt.NewChangeset(%s)}", n.Changeset, n.Desc))
	funcLine(b, fmt.Sprintf("func (r %s) Changeset() %s", m, n.Changeset), fmt.Sprintf("return %s{rt.ChangesetOf(%s, r)}", n.Changeset, n.Desc))
	funcLine(b, fmt.Sprintf("func (c %s) From(r %s) %s", n.Changeset, m, n.Changeset), fmt.Sprintf("return %s{c.Changeset.From(r)}", n.Changeset))
	for _, f := range settable {
		funcLine(b, fmt.Sprintf("func (c %s) %sSet(v %s) %s", n.Changeset, f.GoField, f.GoType, n.Changeset),
			fmt.Sprintf("return %s{rt.Set(c.Changeset, %d, func(voltR *%s) { voltR.%s = v })}", n.Changeset, t.Index(f), m, f.GoField))
	}
	funcLine(b, fmt.Sprintf("func (c %s) Cast(p %s, allow ...%s) %s", n.Changeset, n.Params, n.Column, n.Changeset),
		fmt.Sprintf("return %s{c.Changeset.Cast(p, allow...)}", n.Changeset))
	funcLine(b, fmt.Sprintf("func (c %s) ErrorAdd(col %s, msg string) %s", n.Changeset, n.Column, n.Changeset),
		fmt.Sprintf("return %s{c.Changeset.ErrorAdd(col, msg)}", n.Changeset))

	fmt.Fprintf(b, "func %s(r rt.Row) (%s, error) {\n\tvar voltV %s\n\tvoltErr := r.Scan(", n.Scan, m, m)
	for i, f := range t.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(scanTarget(f))
	}
	fmt.Fprintf(b, ")\n\treturn voltV, rt.Refused(%q, voltErr)\n}\n\n", m)

	vals := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		vals[i] = f.BindOf("voltR." + f.GoField)
	}
	funcLine(b, fmt.Sprintf("func %s(voltR %s) []any", n.Values, m), "return []any{"+strings.Join(vals, ", ")+"}")

	fmt.Fprintf(b, "func %s(voltData, voltVals %s, voltSet []bool) %s {\n", n.Apply, m, m)
	if len(settable) == 0 {
		b.WriteString("\treturn voltData\n}\n\n")
	} else {
		b.WriteString("\tvoltR := voltData\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\tif voltSet[%d] {\n\t\tvoltR.%s = voltVals.%s\n\t}\n", t.Index(f), f.GoField, f.GoField)
		}
		b.WriteString("\treturn voltR\n}\n\n")
	}

	fmt.Fprintf(b, "func %s(c rt.Changeset[%s], p rt.Params[%s], i int) (rt.Changeset[%s], error) {\n", n.Cast, m, m, m)
	if len(settable) > 0 {
		b.WriteString("\tswitch i {\n")
		for _, f := range settable {
			fmt.Fprintf(b, "\tcase %d:\n", t.Index(f))
			fmt.Fprintf(b, "\t\tvoltV, voltOK, voltErr := rt.Field(p, %q, %q, %t, func(voltR *%s) %s { return voltR.%s })\n", f.Wire, f.Col.Type.String(), f.Nullable, m, f.GoType, f.GoField)
			b.WriteString("\t\tif voltErr != nil || !voltOK {\n\t\t\treturn c, voltErr\n\t\t}\n")
			fmt.Fprintf(b, "\t\treturn %s{c}.%sSet(voltV).Changeset, nil\n", n.Changeset, f.GoField)
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\treturn c, nil\n}\n\n")
}

// descEmit writes the desc of one table (spec §changesets): the names,
// the columns, the literal defaults and the generated functions.
func (g *generator) descEmit(t *naming.Table) {
	b := &g.body
	m, n := t.Model, &t.Names
	fmt.Fprintf(b, "var %s = &rt.Desc[%s]{\n", n.Desc, m)
	var head align.Block
	head.Row("Model:", strconv.Quote(m)+",")
	head.Row("Table:", "`"+t.TableSQL()+"`,")
	head.Row("Columns:", "`"+t.ColumnList()+"`,")
	head.WriteTo(b, "\t")
	b.WriteString("\tFields: []rt.FieldDesc{\n")
	for _, f := range t.Fields {
		fmt.Fprintf(b, "\t\t{Column: %q, Wire: %q, Type: %q", f.ColName, f.Wire, f.Col.Type.String())
		if f.Nullable {
			b.WriteString(", Nullable: true")
		}
		if t.IsPK(f) {
			b.WriteString(", Key: true")
		}
		if !f.Increment {
			b.WriteString(", Settable: true")
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t},\n")
	var tail align.Block
	if seed := defaultsGo(t); seed != "" {
		tail.Row("Defaults:", m+seed+",")
	}
	tail.Row("Scan:", n.Scan+",")
	tail.Row("Values:", n.Values+",")
	tail.Row("Apply:", n.Apply+",")
	tail.Row("Cast:", n.Cast+",")
	tail.WriteTo(b, "\t")
	b.WriteString("}\n\n")
}

func (g *generator) header() {
	fmt.Fprintf(&g.out, "// Code generated by volt gen from %s. DO NOT EDIT.\n\n", g.opts.Source)
	if note := g.projectNote(); note != "" {
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
	if note := naming.SettingNote(t.Info.Decl.Settings); note != "" {
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
