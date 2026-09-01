// Project-wide navigation for Volt-layer symbols. The single-file index
// (index.go) resolves DBML names inside one document; this one spans the
// project, so `model: db.Post` in app/routes.volt reaches `Table posts`
// in db/schema.volt, and asking for references on that table finds every
// resources declaration that names it.
package lsp

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// voltSym names a symbol reachable from the Volt layer.
type voltSym struct {
	kind string // "table" | "pipeline" | "package"
	pkg  string // declaring package path; the target path for "package"
	name string // model name (tables) or pipeline name; "" for packages
}

// voltSpan is one identifier's extent in a file.
type voltSpan struct {
	file     string
	pos, end token.Position
}

type voltRef struct {
	span voltSpan // hit-test area (spans the qualifier on `db.posts`)
	edit voltSpan // exactly the identifier a rename may rewrite
	sym  voltSym
	decl bool
	text string // the identifier as written here, for rename
}

// voltDef is a declaration: where it is, and the node itself so hover
// can describe what was declared.
type voltDef struct {
	span  voltSpan
	table *ast.Table
	pipe  *ast.Pipeline
	group *ast.Group
	pred  *ast.Pred
	md    string // prebuilt hover (selects: signatures + rendered SQL)
}

// voltIndex is the whole project's Volt-layer symbol graph.
type voltIndex struct {
	defs  map[voltSym]voltDef
	refs  []voltRef
	texts map[string]string // open buffers, for position conversion
}

func spanOf(n ast.Node) voltSpan {
	p := n.Pos()
	return voltSpan{file: p.Filename, pos: p, end: n.End()}
}

// buildVoltIndex walks every package's declarations twice: definitions
// first (tables and pipelines), then the references that name them.
func buildVoltIndex(pr *lang.Project, overlay map[string]string) *voltIndex {
	ix := &voltIndex{defs: map[voltSym]voltDef{}, texts: overlay}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.Table:
				// Keyed by the name as declared: references spell the
				// table exactly (§V5.1).
				ix.define(voltSym{"table", path, d.Name.Base()}, voltDef{span: spanOf(d.Name), table: d})
			case *ast.Pipeline:
				ix.define(voltSym{"pipeline", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pipe: d})
			case *ast.Group:
				ix.define(voltSym{"group", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), group: d})
			case *ast.Pred:
				ix.define(voltSym{"pred", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pred: d})
			}
		}
		// Selects: definition + a prebuilt hover with the generated
		// signatures and the rendered WHERE/ORDER — the editor answers
		// "what does this mint, and which way does id sort" (§V11.6).
		for _, si := range pkg.Selects {
			d := si.Decl
			ix.define(voltSym{"select", path, d.Name.Name()},
				voltDef{span: spanOf(d.Name), md: selectHoverMD(pkg, si)})
		}
		// A package "declaration" is its first file, so an import can
		// jump somewhere useful.
		if len(pkg.Files) > 0 {
			first := pkg.Files[0]
			ix.defs[voltSym{"package", path, ""}] = voltDef{span: voltSpan{
				file: first.Name,
				pos:  token.Position{Filename: first.Name, Line: 1, Column: 1},
				end:  token.Position{Filename: first.Name, Line: 1, Column: 1},
			}}
		}
	}

	// The schema layer's own references to a table — Ref endpoints,
	// TableGroup members, Records targets — indexed per package over the
	// merged file, so every file of the package is covered with its real
	// positions. Without this a rename would miss half the uses.
	for path, pkg := range pr.Packages {
		info := pkg.Schema()
		if info == nil {
			continue
		}
		single := NewIndex(pkg.Merged(), info)
		for _, occ := range single.Occs {
			if occ.ID.Kind != SymTable {
				continue
			}
			ti, ok := single.Tables[occ.ID.Name]
			if !ok {
				continue
			}
			sp := spanOf(occ.Ident)
			if sp.file == "" {
				continue
			}
			ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"table", path, ti.Decl.Name.Base()}, occ.IsDecl, occ.Ident.Name()})
		}
	}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.ImportDecl:
				for _, spec := range d.Specs {
					target := spec.PathString()
					if _, ok := pr.Packages[target]; !ok {
						continue
					}
					last := spec.Path[len(spec.Path)-1]
					sp := spanOf(last)
					ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"package", target, ""}, false, last.Name()})
				}
			case *ast.Scope:
				ix.scopeRefs(pkg, path, d)
			case *ast.Group:
				// Terms resolve tables first, then groups (§V9.2).
				for _, term := range d.Terms {
					sp := spanOf(term.Name)
					kind := "group"
					if _, isTable := ix.defs[voltSym{"table", path, term.Name.Name()}]; isTable {
						kind = "table"
					}
					ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{kind, path, term.Name.Name()}, false, term.Name.Name()})
				}
			case *ast.Pred:
				ix.predExprRefs(path, d.X)
			case *ast.Select:
				// The target resolves groups first (§V11.2).
				sp := spanOf(d.Target)
				kind := "table"
				if _, isGroup := ix.defs[voltSym{"group", path, d.Target.Name()}]; isGroup {
					kind = "group"
				}
				ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{kind, path, d.Target.Name()}, false, d.Target.Name()})
				if d.Where != nil {
					ix.predExprRefs(path, d.Where)
				}
			}
		}
	}
	return ix
}

// predExprRefs indexes the Pred references inside an expression (§V10.2).
func (ix *voltIndex) predExprRefs(path string, x ast.PredExpr) {
	switch x := x.(type) {
	case *ast.PredBinary:
		ix.predExprRefs(path, x.X)
		ix.predExprRefs(path, x.Y)
	case *ast.PredNot:
		ix.predExprRefs(path, x.X)
	case *ast.PredParen:
		ix.predExprRefs(path, x.X)
	case *ast.PredRef:
		sp := spanOf(x.Name)
		ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"pred", path, x.Name.Name()}, false, x.Name.Name()})
	}
}

func (ix *voltIndex) define(sym voltSym, def voltDef) {
	if _, dup := ix.defs[sym]; dup {
		return // duplicate declaration: the checker reports it
	}
	ix.defs[sym] = def
	ix.refs = append(ix.refs, voltRef{def.span, def.span, sym, true, sym.name})
}

// scopeRefs records the symbol references a scope subtree makes: pipe:
// settings name pipelines, model: settings name tables.
func (ix *voltIndex) scopeRefs(pkg *lang.Package, path string, sc *ast.Scope) {
	ix.settingRefs(pkg, path, sc.Settings)
	for _, item := range sc.Items {
		switch it := item.(type) {
		case *ast.Scope:
			ix.scopeRefs(pkg, path, it)
		case *ast.Resources:
			// The declaration itself names the model (§V5.1).
			target := path
			if it.Pkg != nil {
				t, ok := pkg.Imports[it.Pkg.Name()]
				if !ok {
					continue
				}
				target = t
			}
			// Hover and gd hit anywhere on `db.posts`; a rename must
			// rewrite only the name, never the qualifier.
			edit := spanOf(it.Name)
			hit := edit
			if it.Pkg != nil {
				hit.pos = it.Pkg.Pos()
			}
			ix.refs = append(ix.refs, voltRef{hit, edit, voltSym{"table", target, it.Name.Name()}, false, it.Name.Name()})
			ix.settingRefs(pkg, path, it.Settings)
		case *ast.Route:
			ix.settingRefs(pkg, path, it.Settings)
		}
	}
}

func (ix *voltIndex) settingRefs(pkg *lang.Package, path string, list *ast.SettingList) {
	if list == nil {
		return
	}
	for _, s := range list.Settings {
		switch s.Name {
		case "pipe":
			if id, ok := s.Value.(*ast.Ident); ok {
				sp := spanOf(id)
				ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"pipeline", path, id.Name()}, false, id.Name()})
			}
		}
	}
}

// at returns the reference under the byte offset in file, if any.
func (ix *voltIndex) at(file string, offset int) *voltRef {
	for i := range ix.refs {
		r := &ix.refs[i]
		if r.span.file == file && offset >= r.span.pos.Offset && offset <= r.span.end.Offset {
			return r
		}
	}
	return nil
}

// text returns a file's current content: the open buffer when there is
// one (so positions match what the user sees), else the saved file.
func (ix *voltIndex) text(file string) string {
	if t, ok := ix.texts[file]; ok {
		return t
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(b)
}

func (ix *voltIndex) location(sp voltSpan) *protocol.Location {
	text := ix.text(sp.file)
	return &protocol.Location{
		URI: "file://" + sp.file,
		Range: protocol.Range{
			Start: offsetToLSP(text, sp.pos.Offset),
			End:   offsetToLSP(text, sp.end.Offset),
		},
	}
}

// offsetToLSP converts a byte offset into text to an LSP position
// (0-based line, 0-based UTF-16 column).
func offsetToLSP(text string, off int) protocol.Position {
	if off > len(text) {
		off = len(text)
	}
	if off < 0 {
		off = 0
	}
	line := strings.Count(text[:off], "\n")
	lineStart := strings.LastIndexByte(text[:off], '\n') + 1
	col := len(utf16.Encode([]rune(text[lineStart:off])))
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

/* ===== the two features ===== */

// voltDefinition resolves the Volt-layer symbol under the cursor to its
// declaration, in whichever file of the project declares it.
func (d *Document) voltDefinition(pos protocol.Position) *protocol.Location {
	if d.vindex == nil {
		return nil
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	def, ok := d.vindex.defs[ref.sym]
	if !ok {
		return nil
	}
	return d.vindex.location(def.span)
}

// voltReferences lists every project-wide use of the symbol under the
// cursor — including the resources declarations naming a table.
func (d *Document) voltReferences(pos protocol.Position, includeDecl bool) []protocol.Location {
	if d.vindex == nil {
		return nil
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	var out []protocol.Location
	for _, r := range d.vindex.refs {
		if r.sym != ref.sym || (r.decl && !includeDecl) {
			continue
		}
		out = append(out, *d.vindex.location(r.span))
	}
	return out
}

/* ===== hover ===== */

// voltHover describes the Volt-layer symbol under the cursor. For a
// table it spells out the mapping that is otherwise invisible in
// routes.volt: `model: db.Post` names the *Go model* nao derives from
// `Table posts`, not the table's own name (§V5.4.2).
func (d *Document) voltHover(pos protocol.Position) *protocol.Hover {
	if d.vindex == nil {
		return nil
	}
	path := pathFromURI(d.URI)
	ref := d.vindex.at(path, d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	def, ok := d.vindex.defs[ref.sym]
	if !ok {
		return nil
	}

	var md string
	switch ref.sym.kind {
	case "table":
		md = tableModelHover(ref.sym, def)
	case "pipeline":
		md = pipelineHover(ref.sym, def)
	case "package":
		md = "**package** `" + ref.sym.pkg + "`"
	case "group":
		md = groupHover(ref.sym, def)
	case "pred":
		md = predHover(ref.sym, def, d.vindex)
	case "select":
		md = def.md
	}
	if md == "" {
		return nil
	}
	rng := protocol.Range{
		Start: offsetToLSP(d.Text, ref.span.pos.Offset),
		End:   offsetToLSP(d.Text, ref.span.end.Offset),
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		Range:    &rng,
	}
}

// selectHoverMD renders a select's generated surface: one signature
// per member (one parameter list for all — §V11.4), the WHERE/ORDER
// exactly as emitted, and the output row structs, so the hover answers
// "what comes back from this query for each table" (§V11.6).
func selectHoverMD(pkg *lang.Package, si *lang.SelectInfo) string {
	const structCap = 4
	var b strings.Builder
	b.WriteString("```go\n")
	var params strings.Builder
	for _, p := range si.Params {
		fmt.Fprintf(&params, ", %s %s", p.GoName, p.GoType)
	}
	for _, m := range si.Members {
		model := m.Decl.Name.Base()
		if mn, err := golang.ModelName(m.Decl); err == nil {
			model = mn
		}
		fmt.Fprintf(&b, "func (q *Queries) %s%s(ctx context.Context%s) ([]%s, error)\n",
			model, si.MethodSuffix, params.String(), model)
	}
	b.WriteString("```\n")
	if si.WhereSQL != "" {
		b.WriteString("```sql\nWHERE " + si.WhereSQL + "\n```\n")
	}
	if si.OrderSQL != "" {
		b.WriteString("```sql\nORDER BY " + si.OrderSQL + "\n```\n")
	}
	// The output structs, from the same plan the generator runs.
	shown := si.Members
	if len(shown) > structCap {
		shown = shown[:structCap]
	}
	for _, m := range shown {
		model, fields, err := golang.ModelFields(pkg.Merged(), pkg.Schema(), m.Key)
		if err != nil {
			continue
		}
		b.WriteString("```go\n")
		fmt.Fprintf(&b, "type %s struct {\n", model)
		for _, f := range fields {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, f.Type)
		}
		b.WriteString("}\n```\n")
	}
	if n := len(si.Members) - len(shown); n > 0 {
		fmt.Fprintf(&b, "*… and %d more member structs*\n", n)
	}
	return b.String()
}

// groupHover lists a group's terms as written (§V9).
func groupHover(sym voltSym, def voltDef) string {
	g := def.group
	if g == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "```volt\nGroup %s\n```\n", sym.name)
	for _, t := range g.Terms {
		op := "+"
		if t.Neg {
			op = "-"
		}
		fmt.Fprintf(&b, "- %s %s\n", op, t.Name.Name())
	}
	return b.String()
}

// predHover shows the predicate's source expression when the buffer is
// open, else just its name (§V10).
func predHover(sym voltSym, def voltDef, ix *voltIndex) string {
	p := def.pred
	if p == nil {
		return ""
	}
	md := "```volt\nPred " + sym.name + "\n```\n"
	if text, ok := ix.texts[def.span.file]; ok && p.X != nil {
		start, end := p.X.Pos().Offset, p.X.End().Offset
		if start >= 0 && end <= len(text) && start < end {
			md += "```volt\n" + text[start:end] + "\n```\n"
		}
	}
	return md
}

func tableModelHover(sym voltSym, def voltDef) string {
	t := def.table
	if t == nil {
		return ""
	}
	var b strings.Builder
	model, _ := golang.ModelName(t)
	fmt.Fprintf(&b, "**`Table %s`** in package `%s`", t.Name.String(), sym.pkg)
	if model != "" {
		fmt.Fprintf(&b, " — Go model `%s`", model)
	}
	b.WriteString("\n\n")

	pk, pkType := tablePK(t)
	if pk != "" {
		if pkType != "" {
			fmt.Fprintf(&b, "key: `%s` → route parameter type `%s`\n\n", pk, pkType)
		} else {
			fmt.Fprintf(&b, "key: `%s`\n\n", pk)
		}
	}

	b.WriteString("```dbml\n")
	fmt.Fprintf(&b, "Table %s {\n", t.Name.String())
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		mark := ""
		if col.Name.Name() == pk {
			mark = " [pk]"
		}
		fmt.Fprintf(&b, "\t%s %s%s\n", col.Name.Name(), col.Type.String(), mark)
	}
	b.WriteString("}\n```\n\n")
	b.WriteString("_Routes and helpers derive from this: the URL keeps the table's name, the member helper takes the model's (§V5.4)._")
	return b.String()
}

func pipelineHover(sym voltSym, def voltDef) string {
	p := def.pipe
	if p == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Pipeline `%s`** — %d plug(s), outermost first\n\n```volt\n", sym.name, len(p.Plugs))
	fmt.Fprintf(&b, "Pipeline %s {\n", sym.name)
	for _, plug := range p.Plugs {
		fmt.Fprintf(&b, "\tuse %s\n", plug.Ref.String())
	}
	b.WriteString("}\n```")
	return b.String()
}

// tablePK returns the single-column primary key and the Go type it maps
// to, mirroring the checker's rule (§V5.4.3); "" when there is none or
// the key is composite.
func tablePK(t *ast.Table) (name, goType string) {
	var names []string
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		isPK := col.Settings.Get("pk") != nil || col.Settings.Get("primary key") != nil
		for _, f := range col.LegacyFlags {
			if strings.EqualFold(f.Name(), "pk") {
				isPK = true
			}
		}
		if isPK {
			names = append(names, col.Name.Name())
		}
	}
	if len(names) != 1 {
		return "", ""
	}
	name = names[0]
	for _, item := range t.Body {
		if col, ok := item.(*ast.Column); ok && col.Name.Name() == name {
			if gt, known := golang.GoTypeName(strings.ToLower(col.Type.Name.String())); known {
				goType = gt
			}
		}
	}
	return name, goType
}

/* ===== rename ===== */

// voltRename renames a Volt-layer symbol everywhere it appears in the
// project: the declaration, the schema layer's own references, and the
// resources declarations that name it — across files. Only occurrences
// spelled like the one under the cursor are rewritten, so a table alias
// keeps its own name (matching the single-file rule).
func (d *Document) voltRename(pos protocol.Position, newName string) (*protocol.WorkspaceEdit, bool) {
	if d.vindex == nil {
		return nil, false
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil, false
	}
	if ref.sym.kind == "package" {
		return nil, false // renaming a package means moving a directory
	}

	spelling := ref.text
	changes := map[protocol.DocumentUri][]protocol.TextEdit{}
	seen := map[string]bool{}
	for _, r := range d.vindex.refs {
		if r.sym != ref.sym || r.text != spelling {
			continue
		}
		key := fmt.Sprintf("%s:%d", r.edit.file, r.edit.pos.Offset)
		if seen[key] {
			continue // the declaration is recorded by both passes
		}
		seen[key] = true
		loc := d.vindex.location(r.edit)
		changes[protocol.DocumentUri(loc.URI)] = append(changes[protocol.DocumentUri(loc.URI)],
			protocol.TextEdit{Range: loc.Range, NewText: newName})
	}
	if len(changes) == 0 {
		return nil, false
	}
	return &protocol.WorkspaceEdit{Changes: changes}, true
}

/* ===== completion ===== */

// voltQualRefRE matches a possibly-partial qualified reference at the
// end of the line: `db.`, `db.P`. The qualifier decides whether the
// Volt layer owns the completion — only import qualifiers do, so DBML
// dot-chains (`status.active`, `users.id`) fall through untouched.
var voltQualRefRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z0-9_]*)$`)

// voltResourcesRE matches a resources declaration being typed, with the
// partial table name (possibly empty) after it.
var voltResourcesRE = regexp.MustCompile(`^\s*resources\s+([A-Za-z0-9_]*)$`)

// voltComplete offers project-aware completions: after an import
// qualifier's dot, the tables of that package; after `resources`, the
// local tables and the import qualifiers. Nil means "not ours" and the
// single-file DBML completion proceeds.
func (d *Document) voltComplete(prefix string) []protocol.CompletionItem {
	if d.vpkg == nil || d.vindex == nil {
		return nil
	}

	if m := voltQualRefRE.FindStringSubmatch(prefix); m != nil {
		if target, ok := d.vpkg.Imports[m[1]]; ok {
			return d.packageTableItems(target, m[2])
		}
	}
	if m := voltSelectForRE.FindStringSubmatch(prefix); m != nil {
		items := d.packageTableItems(d.vpkg.Path, m[1])
		items = append(items, d.namedDefItems("group", m[1], protocol.CompletionItemKindStruct)...)
		return items
	}
	if m := voltWhereRE.FindStringSubmatch(prefix); m != nil {
		return d.namedDefItems("pred", m[1], protocol.CompletionItemKindFunction)
	}
	if m := voltResourcesRE.FindStringSubmatch(prefix); m != nil {
		items := d.packageTableItems(d.vpkg.Path, m[1])
		quals := make([]string, 0, len(d.vpkg.Imports))
		for q := range d.vpkg.Imports {
			quals = append(quals, q)
		}
		sort.Strings(quals)
		for _, q := range quals {
			detail := "package " + d.vpkg.Imports[q]
			items = append(items, protocol.CompletionItem{
				Label:  q + ".",
				Kind:   kindPtr(protocol.CompletionItemKindModule),
				Detail: &detail,
			})
		}
		return items
	}
	return nil
}

// voltSelectForRE matches a select's target being typed (§V11.2).
var voltSelectForRE = regexp.MustCompile(`^\s*[Ss]elect\s+\w+\s+for\s+([A-Za-z0-9_]*)$`)

// voltWhereRE matches an identifier being typed inside a where clause
// or a Pred body opened on the same line (§V10).
var voltWhereRE = regexp.MustCompile(`\b(?:where|and|or|not|\{)\s+([A-Za-z0-9_]*)$`)

// namedDefItems lists this package's definitions of one kind.
func (d *Document) namedDefItems(kind, partial string, itemKind protocol.CompletionItemKind) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for sym := range d.vindex.defs {
		if sym.kind != kind || sym.pkg != d.vpkg.Path {
			continue
		}
		if partial != "" && !strings.HasPrefix(strings.ToLower(sym.name), strings.ToLower(partial)) {
			continue
		}
		detail := kind + " " + sym.name
		items = append(items, protocol.CompletionItem{
			Label:  sym.name,
			Kind:   kindPtr(itemKind),
			Detail: &detail,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

// packageTableItems lists the tables a package declares, filtered by
// the partial the user has typed so a manual invoke mid-word works.
// The filter is case-insensitive — someone typing `db.P` from model
// muscle memory should see `posts`, which is also how they learn the
// spelling — and when it would empty the list, everything is offered
// instead (the client fuzzy-filters anyway).
func (d *Document) packageTableItems(pkgPath, partial string) []protocol.CompletionItem {
	type row struct{ name, detail string }
	collect := func(filter string) []row {
		var rows []row
		for sym, def := range d.vindex.defs {
			if sym.kind != "table" || sym.pkg != pkgPath || def.table == nil {
				continue
			}
			if filter != "" && !strings.HasPrefix(strings.ToLower(sym.name), strings.ToLower(filter)) {
				continue
			}
			detail := "Table"
			if pk, goType := tablePK(def.table); pk != "" && goType != "" {
				detail = "Table — key " + pk + " " + goType
			}
			rows = append(rows, row{sym.name, detail})
		}
		return rows
	}
	rows := collect(partial)
	if len(rows) == 0 && partial != "" {
		rows = collect("")
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	items := make([]protocol.CompletionItem, 0, len(rows))
	for _, r := range rows {
		detail := r.detail
		items = append(items, protocol.CompletionItem{
			Label:  r.name,
			Kind:   kindPtr(protocol.CompletionItemKindStruct),
			Detail: &detail,
		})
	}
	return items
}

func kindPtr(k protocol.CompletionItemKind) *protocol.CompletionItemKind { return &k }
