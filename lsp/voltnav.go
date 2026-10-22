// Project-wide navigation for Volt-layer symbols.
package lsp

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

type voltSym struct {
	kind string
	pkg  string
	name string
}

type voltSpan struct {
	file     string
	pos, end token.Position
}

type voltRef struct {
	span voltSpan
	edit voltSpan
	sym  voltSym
	decl bool
	text string
}

type voltDef struct {
	span  voltSpan
	table *ast.Table
	pipe  *ast.Pipeline
	group *ast.Group
	pred  *ast.Pred
	gofn  *lang.GoFunc
	md    string
}

type voltIndex struct {
	defs    map[voltSym]voltDef
	refs    []voltRef
	texts   map[string]string
	gofuncs map[string]map[string]lang.GoFunc
	gosrcs  map[string][]lang.GoSource
	goscans map[string]goScanEntry
}

func (ix *voltIndex) goStale() bool {
	for dir, srcs := range ix.gosrcs {
		if !slices.Equal(lang.GoSourcesRead(dir), srcs) {
			return true
		}
	}
	return false
}

func spanOf(n ast.Node) voltSpan {
	p := n.Pos()
	return voltSpan{file: p.Filename(), pos: p, end: n.End()}
}

type voltIndexMemo struct {
	index   map[string]*IndexMemo
	hovers  map[*lang.SelectInfo]string
	next    map[*lang.SelectInfo]string
	goscans map[string]goScanEntry
}

type goScanEntry struct {
	srcs  []lang.GoSource
	funcs map[string]lang.GoFunc
}

func buildVoltIndex(pr *lang.Project, overlay map[string]string, memo *voltIndexMemo) *voltIndex {
	ix := &voltIndex{defs: map[voltSym]voltDef{}, texts: overlay}
	if memo != nil {
		if memo.index == nil {
			memo.index = map[string]*IndexMemo{}
		}
		if memo.goscans == nil {
			memo.goscans = map[string]goScanEntry{}
		}
		memo.next = make(map[*lang.SelectInfo]string, len(memo.hovers))
		ix.goscans = memo.goscans
		defer func() { memo.hovers, memo.next = memo.next, nil; ix.goscans = nil }()
	}
	hover := func(pkg *lang.Package, si *lang.SelectInfo) string {
		if memo == nil {
			return selectHoverMD(pkg, si)
		}
		md, ok := memo.hovers[si]
		if !ok {
			md = selectHoverMD(pkg, si)
		}
		memo.next[si] = md
		return md
	}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.Table:
				ix.define(voltSym{"table", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), table: d})
			case *ast.Pipeline:
				ix.define(voltSym{"pipeline", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pipe: d})
			case *ast.Group:
				ix.define(voltSym{"group", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), group: d})
			case *ast.Pred:
				ix.define(voltSym{"pred", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pred: d})
			}
		}
		for _, si := range pkg.Selects {
			d := si.Decl
			ix.define(voltSym{"select", path, d.Name.Name()},
				voltDef{span: spanOf(d.Name), md: hover(pkg, si)})
		}
		if info := pkg.Schema(); info != nil {
			for i, mi := range info.Migrations {
				ix.define(voltSym{"migration", path, mi.Name},
					voltDef{span: spanOf(mi.Decl.Name), md: migrationHoverMD(mi, i, len(info.Migrations))})
			}
		}
		if len(pkg.Files) > 0 {
			first := pkg.Files[0]
			ix.defs[voltSym{"package", path, ""}] = voltDef{span: voltSpan{
				file: first.Name,
				pos:  token.At(first.Name, 0, 1, 1),
				end:  token.At(first.Name, 0, 1, 1),
			}}
		}
	}

	for path, pkg := range pr.Packages {
		info := pkg.Schema()
		if info == nil {
			continue
		}
		var im *IndexMemo
		if memo != nil {
			if im = memo.index[path]; im == nil {
				im = &IndexMemo{}
				memo.index[path] = im
			}
		}
		single := NewIndexMemo(pkg.Merged(), info, im)
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
			ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"table", path, ti.Decl.Name.Name()}, occ.IsDecl, occ.Ident.Name()})
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
			case *ast.Pipeline:
				for _, plug := range d.Plugs {
					ix.goRefAdd(pkg, path, plug.Ref)
				}
			case *ast.Table:
				ix.validatorsRefs(pkg, path, d.Body)
			case *ast.TablePartial:
				ix.validatorsRefs(pkg, path, d.Body)
			case *ast.TypeDecl:
				if d.Name != nil && d.Name.Qualifier() == "" {
					ix.goSourcesNote(pkg)
				}
			case *ast.Group:
				for _, term := range d.Terms {
					for _, id := range term.Names {
						sp := spanOf(id)
						ix.refs = append(ix.refs, voltRef{sp, sp, ix.setSym(path, id.Name(), "table", "group"), false, id.Name()})
					}
				}
			case *ast.Pred:
				ix.predExprRefs(path, d.X)
			case *ast.Select:
				sp := spanOf(d.Target)
				ix.refs = append(ix.refs, voltRef{sp, sp, ix.setSym(path, d.Target.Name(), "table", "group"), false, d.Target.Name()})
				if d.Where != nil {
					ix.predExprRefs(path, d.Where)
				}
			}
		}
	}
	return ix
}

func (ix *voltIndex) validatorsRefs(pkg *lang.Package, path string, body []ast.TableItem) {
	for _, item := range body {
		vb, ok := item.(*ast.ValidatorsBlock)
		if !ok {
			continue
		}
		for _, v := range vb.Validators {
			ix.goRefAdd(pkg, path, v.Ref)
		}
	}
}

func (ix *voltIndex) goRefAdd(pkg *lang.Package, path string, ref *ast.GoRef) {
	if ref == nil || len(ref.Parts) == 0 {
		return
	}
	if ref.Qualifier() != "" {
		return
	}
	name := ref.Base()
	sym := voltSym{"gofunc", path, name}
	hit, edit := spanOf(ref), spanOf(ref.Parts[len(ref.Parts)-1])
	ix.refs = append(ix.refs, voltRef{hit, edit, sym, false, name})
	if _, done := ix.defs[sym]; done {
		return
	}
	if ix.gofuncs == nil {
		ix.gofuncs = map[string]map[string]lang.GoFunc{}
	}
	funcs, ok := ix.gofuncs[path]
	if !ok {
		srcs := ix.goSourcesNote(pkg)
		if e, kept := ix.goscans[pkg.Dir]; kept && slices.Equal(e.srcs, srcs) {
			funcs = e.funcs
		} else {
			funcs, _, _ = lang.GoFuncsOf(pkg.Dir, srcs)
			if ix.goscans != nil {
				ix.goscans[pkg.Dir] = goScanEntry{srcs: srcs, funcs: funcs}
			}
		}
		ix.gofuncs[path] = funcs
	}
	if gf, found := funcs[name]; found {
		gfCopy := gf
		span := voltSpan{file: gf.File, pos: gf.Pos, end: gf.End}
		ix.defs[sym] = voltDef{span: span, gofn: &gfCopy, md: goFuncHover(name, pkg.Name, &gfCopy)}
	} else {
		ix.defs[sym] = voltDef{md: goFuncHover(name, pkg.Name, nil)}
	}
}

func (ix *voltIndex) goSourcesNote(pkg *lang.Package) []lang.GoSource {
	if srcs, ok := ix.gosrcs[pkg.Dir]; ok {
		return srcs
	}
	srcs := lang.GoSourcesRead(pkg.Dir)
	if ix.gosrcs == nil {
		ix.gosrcs = map[string][]lang.GoSource{}
	}
	ix.gosrcs[pkg.Dir] = srcs
	return srcs
}

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
		return
	}
	ix.defs[sym] = def
	ix.refs = append(ix.refs, voltRef{def.span, def.span, sym, true, sym.name})
}

func (ix *voltIndex) scopeRefs(pkg *lang.Package, path string, sc *ast.Scope) {
	ix.settingRefs(pkg, path, sc.Settings)
	for _, item := range sc.Items {
		switch it := item.(type) {
		case *ast.Scope:
			ix.scopeRefs(pkg, path, it)
		case *ast.Resources:
			target := path
			if it.Pkg != nil {
				t, ok := pkg.Imports[it.Pkg.Name()]
				if !ok {
					continue
				}
				target = t
			}
			edit := spanOf(it.Name)
			hit := edit
			if it.Pkg != nil {
				hit.pos = it.Pkg.Pos()
			}
			ix.refs = append(ix.refs, voltRef{hit, edit, voltSym{"table", target, it.Name.Name()}, false, it.Name.Name()})
			ix.settingRefs(pkg, path, it.Settings)
		case *ast.Dataset:
			target, ok := path, true
			if it.Pkg != nil {
				target, ok = pkg.Imports[it.Pkg.Name()]
			}
			if ok {
				edit := spanOf(it.Name)
				hit := edit
				if it.Pkg != nil {
					hit.pos = it.Pkg.Pos()
				}
				ix.refs = append(ix.refs, voltRef{hit, edit, voltSym{"select", target, it.Name.Name()}, false, it.Name.Name()})
			}
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
		case "error_handler":
			ix.goSourcesNote(pkg)
		}
	}
}

func (ix *voltIndex) at(file string, offset int) *voltRef {
	for i := range ix.refs {
		r := &ix.refs[i]
		if r.span.file == file && offset >= int(r.span.pos.Offset()) && offset <= int(r.span.end.Offset()) {
			return r
		}
	}
	return nil
}

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
			Start: offsetToLSP(text, int(sp.pos.Offset())),
			End:   offsetToLSP(text, int(sp.end.Offset())),
		},
	}
}

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

func (d *Document) voltDefinition(pos protocol.Position) *protocol.Location {
	if d.vindex == nil {
		return nil
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	def, ok := d.vindex.defs[ref.sym]
	if !ok || def.span.file == "" {
		return nil
	}
	return d.vindex.location(def.span)
}

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
	case "select", "gofunc", "migration":
		md = def.md
	}
	if md == "" {
		return nil
	}
	rng := protocol.Range{
		Start: offsetToLSP(d.Text, int(ref.span.pos.Offset())),
		End:   offsetToLSP(d.Text, int(ref.span.end.Offset())),
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		Range:    &rng,
	}
}

func selectHoverMD(pkg *lang.Package, si *lang.SelectInfo) string {
	const structCap = 4
	var b strings.Builder
	memberFn := func(key string) golang.SelectFn {
		return golang.SelectFn{
			TableKey: key, MethodSuffix: si.MethodSuffix,
			Cols: si.Cols, Excluded: si.Excluded, SharedType: si.Shared,
		}
	}

	shown := si.Members
	if si.Shared != "" {
		shown = shown[:1]
	} else if len(shown) > structCap {
		shown = shown[:structCap]
	}
	plan := pkg.Plan()
	if plan == nil {
		plan = golang.PlanBuild(pkg.Merged(), pkg.Schema())
	}
	if si.Unresolved {
		b.WriteString("```volt\n" + selectHead(si.Decl) + "\n```\n")
	}
	for _, m := range shown {
		row, fields, err := plan.SelectRowType(memberFn(m.Key))
		if err != nil {
			continue
		}
		b.WriteString("```go\n")
		fmt.Fprintf(&b, "type %s struct {\n", row)
		for _, f := range fields {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, fieldTypeMD(m, f))
		}
		b.WriteString("}\n```\n")
	}
	if si.Shared == "" {
		if n := len(si.Members) - len(shown); n > 0 {
			fmt.Fprintf(&b, "*… and %d more member structs*\n", n)
		}
	}
	if si.Unresolved {
		noun, verb := "column", "has"
		if len(si.UnknownCols) > 1 {
			noun, verb = "columns", "have"
		}
		fmt.Fprintf(&b, "unresolved: %s %s %s an unknown type (§columns)\n", noun, strings.Join(si.UnknownCols, ", "), verb)
		return b.String()
	}

	if si.WhereSQL != "" {
		b.WriteString("```sql\nWHERE " + si.WhereSQL + "\n```\n")
		if len(si.Binds) > 0 {
			b.WriteString("binds: `" + strings.Join(si.Binds, "`, `") + "`\n\n")
		}
	}
	if si.OrderSQL != "" {
		b.WriteString("```sql\nORDER BY " + si.OrderSQL + "\n```\n")
	}

	var params strings.Builder
	for _, p := range si.Params {
		fmt.Fprintf(&params, ", %s %s", p.GoName, p.GoType)
	}
	b.WriteString("```go\n")
	for _, m := range si.Members {
		model := m.Decl.Name.Name()
		if mn, err := golang.ModelName(m.Decl); err == nil {
			model = mn
		}
		row := model
		if r, err := plan.SelectRowName(memberFn(m.Key)); err == nil {
			row = r
		}
		fmt.Fprintf(&b, "func (q *Queries) %s%s(ctx context.Context%s) ([]%s, error)\n",
			model, si.MethodSuffix, params.String(), row)
	}
	b.WriteString("```\n")
	return b.String()
}

func fieldTypeMD(m *check.TableInfo, f golang.FieldSig) string {
	cd := m.Column(f.Col)
	switch {
	case cd != nil && !cd.Type.Known():
		return cd.Col.Type.String() + " // unknown type (§columns)"
	case f.Type == "":
		return "// unknown type (§columns)"
	}
	return f.Type
}

func selectHead(d *ast.Select) string {
	head := "Select " + d.Name.Name()
	if d.Projected() {
		cols := identNames(d.Cols)
		if d.Star {
			head += " (* \\ " + strings.Join(cols, " \\ ") + ")"
		} else {
			head += " (" + strings.Join(cols, ", ") + ")"
		}
	}
	return head + " for " + d.Target.Name()
}

func (ix *voltIndex) setSym(path, name string, kinds ...string) voltSym {
	for _, k := range kinds {
		if _, ok := ix.defs[voltSym{k, path, name}]; ok {
			return voltSym{k, path, name}
		}
	}
	return voltSym{kinds[len(kinds)-1], path, name}
}

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
			op = "\\"
		}
		names := make([]string, len(t.Names))
		for i, id := range t.Names {
			names[i] = id.Name()
		}
		term := names[0]
		if t.Set() {
			term = "(" + strings.Join(names, ", ") + ")"
		}
		fmt.Fprintf(&b, "- %s %s\n", op, term)
	}
	return b.String()
}

func predHover(sym voltSym, def voltDef, ix *voltIndex) string {
	p := def.pred
	if p == nil {
		return ""
	}
	md := "```volt\nPred " + sym.name + "\n```\n"
	if text, ok := ix.texts[def.span.file]; ok && p.X != nil {
		start, end := int(p.X.Pos().Offset()), int(p.X.End().Offset())
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
	fmt.Fprintf(&b, "**`Table %s`** in package `%s`", t.Name.Name(), sym.pkg)
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

	b.WriteString("```volt\n")
	fmt.Fprintf(&b, "Table %s {\n", t.Name.Name())
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		var marks []string
		for _, name := range []string{"pk", "null"} {
			if col.Settings.Get(name) != nil {
				marks = append(marks, name)
			}
		}
		mark := ""
		if len(marks) > 0 {
			mark = " [" + strings.Join(marks, ", ") + "]"
		}
		fmt.Fprintf(&b, "\t%s %s%s\n", col.Name.Name(), col.Type.String(), mark)
	}
	b.WriteString("}\n```\n\n")
	b.WriteString("_Routes and helpers derive from this: the URL keeps the table's name, the member helper takes the model's (§table-resolution)._")
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

func tablePK(t *ast.Table) (name, goType string) {
	var names []string
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		if col.Settings.Get("pk") != nil {
			names = append(names, col.Name.Name())
		}
	}
	if len(names) != 1 {
		return "", ""
	}
	name = names[0]
	for _, item := range t.Body {
		if col, ok := item.(*ast.Column); ok && col.Name.Name() == name {
			if lang.KnownParamType(col.Type.String()) {
				goType = col.Type.String()
			}
		}
	}
	return name, goType
}

func (d *Document) voltRename(pos protocol.Position, newName string) (*protocol.WorkspaceEdit, bool) {
	if d.vindex == nil {
		return nil, false
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil, false
	}
	if ref.sym.kind == "package" {
		return nil, false
	}

	spelling := ref.text
	changes := map[protocol.DocumentUri][]protocol.TextEdit{}
	seen := map[string]bool{}
	for _, r := range d.vindex.refs {
		if r.sym != ref.sym || r.text != spelling {
			continue
		}
		key := fmt.Sprintf("%s:%d", r.edit.file, r.edit.pos.Offset())
		if seen[key] {
			continue
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

var voltQualRefRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z0-9_]*)$`)

var voltResourcesRE = regexp.MustCompile(`^\s*resources\s+([A-Za-z0-9_]*)$`)

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

var voltSelectForRE = regexp.MustCompile(`^\s*Select\s+\w+\s+for\s+([A-Za-z0-9_]*)$`)

var voltWhereRE = regexp.MustCompile(`\b(?:where|and|or|not|\{)\s+([A-Za-z0-9_]*)$`)

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
