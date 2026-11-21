// Package golang renders a checked package's naming plan (package naming)
// as the Go data files: the models with their changesets and column
// handles (spec/changesets), the static and dynamic query layers, the
// selects, the validators and the migrations. The output is
// gofmt-canonical by construction, carries no comment but the header and
// the doc comments (spec/doc-comments-in-generated-code), and imports only the
// standard library, the rt runtime package and the packages of declared
// types.
package golang

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/gen/layout"
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
	// PackageDoc is the package's doc text, written as the package doc
	// comment of the models file (spec/doc-comments-in-generated-code).
	PackageDoc string
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
// is for Dyn (spec/changesets).
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
	g.body.Grow(fieldCount(tables)*750 + len(tables)*2500)
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
// and which scans through that type and binds by kind (spec/types).
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
// struct (spec/changesets): the column and params aliases, the column
// handles, the desc, the changeset type with its typed methods, and the
// functions the desc carries.
func (g *generator) changesetEmit(t *naming.Table) {
	if len(t.Fields) == 0 {
		return
	}
	g.imports[rtImport] = true
	b := &g.body
	m, n := t.Model, &t.Names

	fmt.Fprintf(b, "type %s = rt.ColumnOf[%s]\n\n", n.Column, m)
	fmt.Fprintf(b, "type %s = rt.Params[%s]\n\n", n.Params, m)

	handles := make([]layout.Spec, len(t.Fields))
	for i, f := range t.Fields {
		column := literal(fmt.Sprintf("rt.Column[%s, %s]", m, f.BaseType), []string{"Name"}, text(strconv.Quote(f.ColName)))
		if f.Nullable {
			column = literal(fmt.Sprintf("rt.NullColumn[%s, %s]", m, f.BaseType), []string{"Column"}, column)
		}
		handles[i] = layout.Spec{Name: t.Handle(f), Value: column}
	}
	b.WriteString("var (\n")
	layout.SpecsWrite(b, 1, handles)
	b.WriteString(")\n\n")

	g.descEmit(t)

	if embedded := "rt.Changeset[" + m + "]"; len(embedded) <= gofmtOneLineFieldMax {
		fmt.Fprintf(b, "type %s struct{ %s }\n\n", n.Changeset, embedded)
	} else {
		fmt.Fprintf(b, "type %s struct {\n\t%s\n}\n\n", n.Changeset, embedded)
	}
	wrap := func(e elem) elem { return stmt("return ", values(n.Changeset, e)) }
	funcEmit(b, "func "+n.New, nil, n.Changeset, wrap(call("rt.NewChangeset", text(n.Desc))))
	funcEmit(b, "func (r "+m+") Changeset", nil, n.Changeset, wrap(calls("rt.ChangesetOf", n.Desc, "r")))
	funcEmit(b, "func (c "+n.Changeset+") From", []string{"r " + m}, n.Changeset, wrap(call("c.Changeset.From", text("r"))))
	for _, f := range t.Fields {
		set := funcLit([]string{"voltR *" + m}, "", text("voltR."+f.GoField+" = v"))
		setter := call("rt.Set", append(texts("c.Changeset", strconv.Itoa(t.Index(f))), set)...)
		funcEmit(b, "func (c "+n.Changeset+") "+f.GoField+"Set", []string{"v " + f.GoType}, n.Changeset, wrap(setter))
	}
	funcEmit(b, "func (c "+n.Changeset+") Cast", []string{"p " + n.Params, "allow ..." + n.Column}, n.Changeset,
		wrap(calls("c.Changeset.Cast", "p", "allow...")))
	funcEmit(b, "func (c "+n.Changeset+") ErrorAdd", []string{"col " + n.Column, "msg string"}, n.Changeset,
		wrap(calls("c.Changeset.ErrorAdd", "col", "msg")))

	targets := make(elems, len(t.Fields))
	binds := make(elems, len(t.Fields))
	for i, f := range t.Fields {
		targets[i] = text(scanTarget(f))
		binds[i] = text(f.BindOf("voltR." + f.GoField))
	}
	funcEmit(b, "func "+n.Scan, []string{"r rt.Row"}, "("+m+", error)",
		text("var voltV "+m),
		stmt("voltErr := ", call("r.Scan", targets...)),
		stmt("return voltV, ", calls("rt.Refused", strconv.Quote(m), "voltErr")))
	funcEmit(b, "func "+n.Values, []string{"voltR " + m}, "[]any", stmt("return ", values("[]any", binds...)))

	sig := layout.Signature("func "+n.Apply, []string{"voltData, voltVals " + m, "voltSet []bool"}, []string{m}, " {")
	layout.WriteLine(b, 0, "", sig, "\n")
	b.WriteString("\tvoltR := voltData\n")
	for _, f := range t.Fields {
		fmt.Fprintf(b, "\tif voltSet[%d] {\n\t\tvoltR.%s = voltVals.%s\n\t}\n", t.Index(f), f.GoField, f.GoField)
	}
	b.WriteString("\treturn voltR\n}\n\n")

	g.castEmit(t)
}

func (g *generator) castEmit(t *naming.Table) {
	b := &g.body
	m, n := t.Model, &t.Names
	params := []string{"c rt.Changeset[" + m + "]", "p rt.Params[" + m + "]", "i int"}
	sig := layout.Signature("func "+n.Cast, params, []string{"rt.Changeset[" + m + "]", "error"}, " {")
	layout.WriteLine(b, 0, "", sig, "\n")
	b.WriteString("\tswitch i {\n")
	for _, f := range t.Fields {
		fmt.Fprintf(b, "\tcase %d:\n", t.Index(f))
		cast := layout.ListStrings(n.Changeset+"{", []string{"c"}, "}.volt"+f.GoField+"Cast(p)")
		layout.WriteLine(b, 2, "return ", cast, "\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("\treturn c, nil\n}\n\n")
	for _, f := range t.Fields {
		get := funcLit([]string{"voltR *" + m}, f.GoType, text("return voltR."+f.GoField))
		field := call("rt.Field", append(texts("p", strconv.Quote(f.Wire), strconv.Quote(f.Col.Type.String()), nullability(f.Nullable)), get)...)
		funcEmit(b, "func (c "+n.Changeset+") volt"+f.GoField+"Cast", []string{"p " + n.Params}, "(rt.Changeset["+m+"], error)",
			stmt("voltV, voltOK, voltErr := ", field),
			text("if voltErr != nil || !voltOK {\n\t\treturn c.Changeset, voltErr\n\t}"),
			text("return c."+f.GoField+"Set(voltV).Changeset, nil"))
	}
}

func (g *generator) descEmit(t *naming.Table) {
	b := &g.body
	m, n := t.Model, &t.Names
	descs := make(elems, len(t.Fields))
	for i, f := range t.Fields {
		keys := []string{"Column", "Wire"}
		vals := texts(strconv.Quote(f.ColName), strconv.Quote(f.Wire))
		for _, flag := range []struct {
			key string
			set bool
		}{{"Key", t.IsPK(f)}, {"RowidAlias", f.RowidAlias}, {"Assigned", f.Assigned}} {
			if flag.set {
				keys, vals = append(keys, flag.key), append(vals, text("true"))
			}
		}
		descs[i] = layout.Fields("{", keys, vals, "}")
	}
	keys := []string{"Model", "Table", "Columns", "Fields"}
	vals := elems{text(strconv.Quote(m)), text("`" + t.TableSQL() + "`"), sqlFragment(t.ColumnList()), values("[]rt.FieldDesc", descs...)}
	if seedKeys, seeds := defaultsGo(t); len(seedKeys) > 0 {
		keys, vals = append(keys, "Defaults"), append(vals, literal(m, seedKeys, seeds...))
	}
	keys = append(keys, "Scan", "Values", "Apply", "Cast")
	vals = append(vals, texts(n.Scan, n.Values, n.Apply, n.Cast)...)
	before := "var " + n.Desc + " = "
	layout.WriteLine(b, 0, before, literal("&rt.Desc["+m+"]", keys, vals...), "\n\n")
}

func (g *generator) header() {
	fmt.Fprintf(&g.out, "// Code generated by volt gen from %s. DO NOT EDIT.\n\n", g.opts.Source)
	if g.opts.PackageDoc != "" {
		commentWrite(&g.out, g.opts.PackageDoc)
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

func (g *generator) tableEmit(t *naming.Table) {
	if doc := t.Info.Decl.Doc.Text(); doc != "" {
		commentWrite(&g.body, doc)
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
		if doc := f.Col.Doc.Text(); doc != "" {
			commentLines(&rows, doc)
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

func defaultsGo(t *naming.Table) ([]string, elems) {
	var keys []string
	var seeds elems
	for _, f := range t.Fields {
		if expr, ok := defaultGo(f); ok {
			keys, seeds = append(keys, f.GoField), append(seeds, text(expr))
		}
	}
	return keys, seeds
}

// defaultGo renders the literal default a <Model>New seeds for one column
// (spec/changesets): a Go constant of the column's type, wrapped
// in rt.Some when nullable, converted to a kinded declared type there so
// the wrapper takes the column's type. A zero literal on a non-nullable
// column is the zero value and needs no seed; on a nullable column it is
// a value, not NULL, and is seeded. A declared type that is not kinded
// gets no seed: its default is the database's.
func defaultGo(f *naming.Field) (string, bool) {
	s := f.Col.Settings.Get("default")
	if s == nil || (f.Declared && !f.Kinded) || f.Assigned {
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
		if f.Kinded || f.BaseType == "int64" {
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

func nullability(nullable bool) string {
	if nullable {
		return "rt.Nullable"
	}
	return "rt.NotNull"
}
