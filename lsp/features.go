package lsp

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/scanner"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/lsp/protocol"
)

// LSPDiagnostics converts the front end's diagnostics for publishing.
func (d *Document) LSPDiagnostics() []protocol.Diagnostic {
	return diagnosticsLSP(d.Text, d.lineOffsets, d.Diags)
}

func diagnosticsLSP(text string, starts []int, diags []diag.Diagnostic) []protocol.Diagnostic {
	out := make([]protocol.Diagnostic, 0, len(diags))
	for _, dg := range diags {
		severity := protocol.DiagnosticSeverityError
		if dg.Severity == diag.Warning {
			severity = protocol.DiagnosticSeverityWarning
		}
		sev := severity
		code := dg.Code
		source := "volt"
		out = append(out, protocol.Diagnostic{
			Range:    diagnosticRangeIn(text, starts, dg.Pos),
			Severity: &sev,
			Code:     code,
			Source:   &source,
			Message:  dg.Msg,
		})
	}
	return out
}

// DocumentSymbols builds the outline tree.
func (d *Document) DocumentSymbols() []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, decl := range d.File.Decls {
		switch n := decl.(type) {
		case *ast.Table:
			out = append(out, d.symbol(n.Name.Name(), "table", protocol.SymbolKindStruct, n, n.Name, d.tableChildren(n.Body)))
		case *ast.TablePartial:
			out = append(out, d.symbol(n.Name.Name(), "table_partial", protocol.SymbolKindInterface, n, n.Name, d.tableChildren(n.Body)))
		case *ast.TypeDecl:
			out = append(out, d.symbol(n.Name.String(), "type", protocol.SymbolKindClass, n, n.Name, nil))
		case *ast.Ref:
			out = append(out, d.symbol(refLabel(n), "ref", protocol.SymbolKindKey, n, n, nil))
		case *ast.Group:
			out = append(out, d.symbol(n.Name.Name(), "group", protocol.SymbolKindNamespace, n, n.Name, nil))
		case *ast.Records:
			label := "records"
			if n.Table != nil {
				label = "records " + n.Table.Name()
			}
			out = append(out, d.symbol(label, "", protocol.SymbolKindArray, n, n, nil))
		case *ast.Migration:
			out = append(out, d.symbol(n.Name.Name(), "migration", protocol.SymbolKindModule, n, n.Name, nil))
		}
	}
	return out
}

func (d *Document) tableChildren(body []ast.TableItem) []protocol.DocumentSymbol {
	var kids []protocol.DocumentSymbol
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Column:
			kids = append(kids, d.symbol(n.Name.Name(), n.Type.String(), protocol.SymbolKindField, n, n.Name, nil))
		case *ast.PartialRef:
			kids = append(kids, d.symbol("~"+n.Name.Name(), "injected partial", protocol.SymbolKindInterface, n, n.Name, nil))
		case *ast.IndexesBlock:
			kids = append(kids, d.symbol("indexes", "", protocol.SymbolKindProperty, n, n, nil))
		case *ast.ValidatorsBlock:
			kids = append(kids, d.symbol("validators", "", protocol.SymbolKindProperty, n, n, nil))
		}
	}
	return kids
}

func (d *Document) symbol(name, detail string, kind protocol.SymbolKind, full, sel ast.Node, children []protocol.DocumentSymbol) protocol.DocumentSymbol {
	s := protocol.DocumentSymbol{
		Name:           name,
		Kind:           kind,
		Range:          d.RangeOf(full),
		SelectionRange: d.RangeOf(sel),
		Children:       children,
	}
	if detail != "" {
		s.Detail = &detail
	}
	return s
}

func refLabel(n *ast.Ref) string {
	left, right := "?", "?"
	if n.Left != nil {
		left = n.Left.Table.Name() + "." + strings.Join(identNames(n.Left.Columns), ",")
	}
	if n.Right != nil {
		right = n.Right.Table.Name() + "." + strings.Join(identNames(n.Right.Columns), ",")
	}
	return fmt.Sprintf("%s > %s", left, right)
}

func identNames(ids []*ast.Ident) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Name()
	}
	return out
}

// Definition returns the location of the declaration of the symbol at pos;
// on the name of a type declaration, the Go type it admits (§types).
func (d *Document) Definition(pos protocol.Position) *protocol.Location {
	if loc := d.voltDefinition(pos); loc != nil {
		return loc
	}
	if loc := d.goTypeDefinition(d.FromLSP(pos)); loc != nil {
		return loc
	}
	occ := d.Index.At(d.FromLSP(pos))
	if occ == nil {
		return nil
	}
	decl := d.Index.Decl(occ.ID)
	if decl == nil {
		return nil
	}
	return &protocol.Location{URI: d.URI, Range: d.tokenRange(decl.Tok)}
}

// References returns every occurrence of the symbol at pos.
func (d *Document) References(pos protocol.Position, includeDecl bool) []protocol.Location {
	out := d.voltReferences(pos, includeDecl)
	seen := map[protocol.Location]bool{}
	for _, l := range out {
		seen[l] = true
	}
	if occ := d.Index.At(d.FromLSP(pos)); occ != nil {
		for _, o := range d.Index.OccurrencesOf(occ.ID) {
			if o.IsDecl && !includeDecl {
				continue
			}
			l := protocol.Location{URI: d.URI, Range: d.tokenRange(o.Ident.Tok)}
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	return out
}

func validIdentifier(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}

// Hover renders documentation for the symbol at pos.
func (d *Document) Hover(pos protocol.Position) *protocol.Hover {
	if h := d.voltHover(pos); h != nil {
		return h
	}
	if h := d.migrationHover(pos); h != nil {
		return h
	}
	occ := d.Index.At(d.FromLSP(pos))
	if occ == nil {
		return d.docHover(pos)
	}
	var md string
	switch occ.ID.Kind {
	case SymTable:
		md = d.tableHover(occ.ID.Name)
	case SymPartial:
		md = d.partialHover(occ.ID.Name)
	case SymColumn:
		md = d.columnHover(occ.ID)
	case SymType:
		md = d.typeHover(occ.ID.Name)
	}
	if md == "" {
		return d.docHover(pos)
	}
	rng := d.tokenRange(occ.Ident.Tok)
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		Range:    &rng,
	}
}

func (d *Document) tableHover(key string) string {
	ti, ok := d.Index.Tables[key]
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "```volt\ntable %s\n```\n", ti.Decl.Name.Name())
	for _, cd := range ti.Columns {
		line := "- `" + cd.Col.Name.Name() + " " + cd.Col.Type.String() + "`" + settingsSuffix(cd.Col.Settings)
		if cd.Partial != nil {
			line += " *(from ~" + cd.Partial.Name.Name() + ")*"
		}
		b.WriteString(line + "\n")
	}
	return b.String() + docSuffix(ti.Decl)
}

func (d *Document) migrationHover(pos protocol.Position) *protocol.Hover {
	off := d.FromLSP(pos)
	for i, mi := range d.Info.Migrations {
		name := mi.Decl.Name
		if off < int(name.Pos().Offset()) || off > int(name.End().Offset()) {
			continue
		}
		rng := d.tokenRange(name.Tok)
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: migrationHoverMD(mi, i, len(d.Info.Migrations)) + docSuffix(mi.Decl)},
			Range:    &rng,
		}
	}
	return nil
}

func migrationHoverMD(mi *check.MigrationInfo, i, n int) string {
	way := "reversible"
	if !mi.Reversible {
		way = "irreversible"
	}
	return fmt.Sprintf("```volt\nmigration %s\n```\nmigration %d of %d, %s\n", mi.Name, i+1, n, way)
}

func (d *Document) typeHover(key string) string {
	td, ok := d.Index.Types[key]
	if !ok {
		return ""
	}
	md := "```volt\ntype " + td.Key + " " + td.Decl.Storage.String()
	if td.Path != "" {
		if q, ok := scanner.Quote(td.Path); ok {
			md += " from " + q
		}
	}
	return md + "\n```\n" + docSuffix(td.Decl)
}

func (d *Document) partialHover(name string) string {
	pi, ok := d.Index.Partials[name]
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "```volt\ntable_partial %s\n```\n", name)
	for _, item := range pi.Decl.Body {
		if col, ok := item.(*ast.Column); ok {
			b.WriteString("- `" + col.Name.Name() + " " + col.Type.String() + "`" + settingsSuffix(col.Settings) + "\n")
		}
	}
	fmt.Fprintf(&b, "\nInjected %d time(s).\n", pi.Uses)
	return b.String() + docSuffix(pi.Decl)
}

func (d *Document) columnHover(id SymbolID) string {
	col := d.columnFind(id)
	if col == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "```volt\n%s %s\n```\n", col.Name.Name(), col.Type.String())
	if s := settingsSuffix(col.Settings); s != "" {
		b.WriteString(strings.TrimSpace(s) + "\n")
	}
	if strings.HasPrefix(id.Container, "partial:") {
		b.WriteString("\nDefined in table_partial `" + strings.TrimPrefix(id.Container, "partial:") + "`.\n")
	}
	return b.String() + docSuffix(col)
}

func (d *Document) columnFind(id SymbolID) *ast.Column {
	if name, ok := strings.CutPrefix(id.Container, "partial:"); ok {
		pi, ok := d.Index.Partials[name]
		if !ok {
			return nil
		}
		for _, item := range pi.Decl.Body {
			if col, ok := item.(*ast.Column); ok && col.Name.Name() == id.Name {
				return col
			}
		}
		return nil
	}
	key := strings.TrimPrefix(id.Container, "table:")
	ti, ok := d.Index.Tables[key]
	if !ok {
		return nil
	}
	if cd := ti.Column(id.Name); cd != nil {
		return cd.Col
	}
	return nil
}

func settingsSuffix(s *ast.SettingList) string {
	var parts []string
	if s != nil {
		for _, setting := range s.Settings {
			part := setting.Name
			if lit, ok := setting.Value.(*ast.BasicLit); ok && lit.Tok.Kind == token.STRING {
				part += ": " + lit.Tok.Text()
			} else if setting.Value != nil {
				part += ": " + valueString(setting.Value)
			}
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " — " + strings.Join(parts, ", ")
}

func valueString(v ast.Node) string {
	switch v := v.(type) {
	case *ast.BasicLit:
		return v.Tok.Val
	case *ast.Ident:
		return v.Name()
	case *ast.FuncExpr:
		return "`" + v.Text() + "`"
	case *ast.NegNumber:
		return "-" + v.Num.Tok.Val
	case *ast.QualIdent:
		return v.Qual.Name() + "." + v.Name.Name()
	case *ast.RefEndpoint:
		cols := identNames(v.Columns)
		if len(cols) > 1 {
			return v.Table.Name() + ".(" + strings.Join(cols, ", ") + ")"
		}
		return v.Table.Name() + "." + strings.Join(cols, ", ")
	}
	return ""
}

func docSuffix(n ast.Node) string {
	if text := ast.DocOf(n).Text(); text != "" {
		return "\n\n---\n\n" + text + "\n"
	}
	return ""
}

func (d *Document) docHover(pos protocol.Position) *protocol.Hover {
	if d.File == nil {
		return nil
	}
	line := int(pos.Line) + 1
	var found ast.Node
	visit := func(n ast.Node) {
		if n != nil && ast.DocOf(n) != nil && n.Pos().Line() == line {
			found = n
		}
	}
	for _, decl := range d.File.Decls {
		if decl.Pos().Line() > line || decl.End().Line() < line {
			continue
		}
		docWalk(decl, visit)
	}
	if found == nil {
		return nil
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: ast.DocOf(found).Text()},
	}
}

func docWalk(n ast.Node, visit func(ast.Node)) {
	switch x := n.(type) {
	case *ast.ImportDecl:
		visit(x)
		for _, s := range x.Specs {
			visit(s)
		}
	case *ast.Pipeline:
		visit(x)
		for _, p := range x.Plugs {
			visit(p)
		}
	case *ast.Scope:
		visit(x)
		for _, item := range x.Items {
			docWalk(item, visit)
		}
	case *ast.Route:
		visit(x)
		for _, ld := range x.Loads {
			visit(ld)
		}
	case *ast.Group:
		visit(x)
		for _, term := range x.Terms {
			visit(term)
		}
	case *ast.Resources, *ast.Dataset, *ast.ScopeLoad, *ast.PackageClause, *ast.Pred, *ast.Select:
		visit(x)
	default:
		ast.Inspect(n, func(c ast.Node) bool {
			visit(c)
			return true
		})
	}
}
