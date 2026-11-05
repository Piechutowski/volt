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

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
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
	md   string
}

type voltDef struct {
	span   voltSpan
	table  *ast.Table
	pkg    *lang.Package
	pipe   *ast.Pipeline
	group  *ast.Group
	pred   *ast.Pred
	gofn   *lang.GoFunc
	md     string
	column string
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
		if !slices.Equal(lang.GoSourcesRead(disk.Source{}, dir), srcs) {
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
				ix.define(voltSym{"table", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), table: d, pkg: pkg})
			case *ast.Pipeline:
				ix.define(voltSym{"pipeline", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pipe: d})
			case *ast.Group:
				ix.define(voltSym{"group", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), group: d, pkg: pkg})
			case *ast.Pred:
				ix.define(voltSym{"pred", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pred: d})
			}
		}
		for _, si := range pkg.Selects {
			d := si.Decl
			ix.define(voltSym{"select", path, d.Name.Name()},
				voltDef{span: spanOf(d.Name), md: hover(pkg, si)})
			ix.paramRefs(path, si)
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
			sp := spanOf(occ.Ident)
			if sp.file == "" {
				continue
			}
			switch occ.ID.Kind {
			case SymTable:
				ti, ok := single.Tables[occ.ID.Name]
				if !ok {
					continue
				}
				md := ""
				if via := single.Via(occ.Ident); via.Kind == SymRelation {
					md = ix.relationDef(path, single.Info, via).md
				}
				ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"table", path, ti.Decl.Name.Name()}, occ.IsDecl, occ.Ident.Name(), md})
			case SymRelation:
				sym := voltSym{"relation", path, occ.ID.Container + "." + occ.ID.Name}
				if _, done := ix.defs[sym]; !done {
					ix.defs[sym] = ix.relationDef(path, single.Info, occ.ID)
				}
				ix.refs = append(ix.refs, voltRef{sp, sp, sym, occ.IsDecl, occ.Ident.Name(), ""})
			}
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
					ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"package", target, ""}, false, last.Name(), ""})
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
						ix.refs = append(ix.refs, voltRef{sp, sp, ix.setSym(path, id.Name(), "table", "group"), false, id.Name(), ""})
					}
				}
			case *ast.Pred:
				ix.predExprRefs(path, d.X)
			case *ast.Select:
				sp := spanOf(d.Target)
				ix.refs = append(ix.refs, voltRef{sp, sp, ix.setSym(path, d.Target.Name(), "table", "group"), false, d.Target.Name(), ""})
				if d.Where != nil {
					ix.predExprRefs(path, d.Where)
				}
			}
		}
	}
	return ix
}

// relationDef is where a relation is declared and how it hovers (spec
// §relationships-ref): at the as: or reverse: that names it, else at the
// column its name derives from, which it remembers, and as the
// relationship it walks followed by the table it leads to.
func (ix *voltIndex) relationDef(path string, info *check.Info, id SymbolID) voltDef {
	r, _ := info.Relation(id.Container, id.Name)
	if r == nil {
		return voltDef{}
	}
	var def voltDef
	named, column := relationNaming(r)
	switch {
	case named != nil:
		def.span = spanOf(named)
	case column != "":
		def.column = column
		if cd := r.Ref.FK.Table.Column(column); cd != nil {
			def.span = spanOf(cd.Col.Name)
		}
	}
	var b strings.Builder
	owner := id.Container
	if t := info.Table(id.Container); t != nil {
		owner = t.Decl.Name.Name()
	}
	target := r.Table.Decl.Name.Name()
	fk := r.Ref.FK.Table.Decl.Name.Name() + "." + strings.Join(r.Ref.FK.Columns, ", ")
	if r.Forward {
		fmt.Fprintf(&b, "**relation `%s`** of `%s`: the one `%s` row `%s` references", r.Name, owner, target, fk)
		if r.Optional {
			b.WriteString(", or none")
		}
	} else {
		fmt.Fprintf(&b, "**relation `%s`** of `%s`: the `%s` rows whose `%s` references it", r.Name, owner, target, fk)
	}
	if column != "" {
		fmt.Fprintf(&b, "; named after the column `%s`", column)
	}
	b.WriteString(" (§relationships-ref)\n\n---\n")
	b.WriteString(tableModelHover(voltSym{"table", path, target}, ix.defs[voltSym{"table", path, target}]))
	def.md = b.String()
	return def
}

// paramRefs records every parameter a select names, in its where and as
// its limit:, hovering as the Go parameter the select's functions take
// (spec §predicates, §selects-over-groups).
func (ix *voltIndex) paramRefs(path string, si *lang.SelectInfo) {
	d := si.Decl
	var params []*ast.Param
	var walk func(x ast.PredExpr)
	operand := func(o ast.Operand) {
		if p, ok := o.(*ast.Param); ok {
			params = append(params, p)
		}
	}
	walk = func(x ast.PredExpr) {
		switch x := x.(type) {
		case *ast.PredBinary:
			walk(x.X)
			walk(x.Y)
		case *ast.PredNot:
			walk(x.X)
		case *ast.PredParen:
			walk(x.X)
		case *ast.PredExists:
			walk(x.X)
		case *ast.PredCompare:
			operand(x.X)
			operand(x.Y)
		case *ast.PredIn:
			if x.List != nil {
				params = append(params, x.List)
			}
		case *ast.PredLike:
			operand(x.Pattern)
		}
	}
	if d.Where != nil {
		walk(d.Where)
	}
	if s := d.Settings.Get("limit"); s != nil {
		if p, ok := s.Value.(*ast.Param); ok {
			params = append(params, p)
		}
	}
	for _, p := range params {
		name := p.Name.Name()
		for _, sp := range si.Params {
			if sp.GoName != name {
				continue
			}
			sym := voltSym{"param", path, d.Name.Name() + ":" + name}
			if _, done := ix.defs[sym]; !done {
				what := "a parameter"
				switch {
				case name == si.LimitParam:
					what = "the limit: of the select, at least 1 (§selects-over-groups)"
				case sp.Optional:
					what = "an optional parameter: absent, the terms that read it are taken out of the predicate (§predicates)"
				}
				ix.defs[sym] = voltDef{md: fmt.Sprintf("```go\n%s %s\n```\n%s of select `%s`\n", name, sp.Type(), what, d.Name.Name())}
			}
			sp := spanOf(p)
			ix.refs = append(ix.refs, voltRef{sp, sp, sym, false, name, ""})
		}
	}
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
	ix.refs = append(ix.refs, voltRef{hit, edit, sym, false, name, ""})
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
	srcs := lang.GoSourcesRead(disk.Source{}, pkg.Dir)
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
	case *ast.PredExists:
		ix.predExprRefs(path, x.X)
	case *ast.PredRef:
		sp := spanOf(x.Name)
		ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"pred", path, x.Name.Name()}, false, x.Name.Name(), ""})
	}
}

func (ix *voltIndex) define(sym voltSym, def voltDef) {
	if _, dup := ix.defs[sym]; dup {
		return
	}
	ix.defs[sym] = def
	ix.refs = append(ix.refs, voltRef{def.span, def.span, sym, true, sym.name, ""})
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
			ix.refs = append(ix.refs, voltRef{hit, edit, voltSym{"table", target, it.Name.Name()}, false, it.Name.Name(), ""})
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
				ix.refs = append(ix.refs, voltRef{hit, edit, voltSym{"select", target, it.Name.Name()}, false, it.Name.Name(), ""})
			}
			ix.settingRefs(pkg, path, it.Settings)
		case *ast.Route:
			ix.settingRefs(pkg, path, it.Settings)
		case *ast.ScopeLoad:
			ix.goRefAdd(pkg, path, it.Ref)
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
				ix.refs = append(ix.refs, voltRef{sp, sp, voltSym{"pipeline", path, id.Name()}, false, id.Name(), ""})
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
	seen := map[protocol.Location]bool{}
	for _, r := range d.vindex.refs {
		if r.sym != ref.sym || (r.decl && !includeDecl) {
			continue
		}
		if loc := *d.vindex.location(r.span); !seen[loc] {
			seen[loc] = true
			out = append(out, loc)
		}
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

	md := ref.md
	switch {
	case md != "":
	case ref.sym.kind == "table":
		md = tableModelHover(ref.sym, def)
	case ref.sym.kind == "pipeline":
		md = pipelineHover(ref.sym, def)
	case ref.sym.kind == "package":
		md = "**package** `" + ref.sym.pkg + "`"
	case ref.sym.kind == "group":
		md = groupHover(ref.sym, def, d.vindex)
	case ref.sym.kind == "pred":
		md = predHover(ref.sym, def, d.vindex)
	case slices.Contains([]string{"select", "gofunc", "migration", "param", "relation"}, ref.sym.kind):
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
	memberFn := func(key string) naming.SelectFn {
		return naming.SelectFn{
			TableKey: key, MethodSuffix: si.MethodSuffix,
			Items: si.Items, Excluded: si.Excluded, SharedType: si.Shared, Joins: si.JoinFns(), Group: si.Group, Lookup: si.Lookup, With: si.WithFn(),
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
		plan = naming.PlanBuild(pkg.Merged(), pkg.Schema())
	}
	if si.Unresolved {
		b.WriteString("```volt\n" + selectHead(si.Decl) + "\n```\n")
	}
	keyType := map[string]string{}
	for _, m := range shown {
		row, fields, err := plan.SelectRowType(memberFn(m.Key))
		if err != nil {
			continue
		}
		b.WriteString("```go\n")
		fmt.Fprintf(&b, "type %s struct {\n", row)
		for _, f := range fields {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, fieldTypeMD(m, f))
			if si.With != nil && f.Col == si.With.ParentKey {
				keyType[m.Key] = f.Type
			}
		}
		b.WriteString("}\n```\n")
	}
	child := ""
	if w := si.With; w != nil && w.Table != nil {
		child = w.Table.Decl.Name.Name()
		if mn, err := naming.ModelName(w.Table.Decl); err == nil {
			child = mn
		}
		if len(shown) > 0 {
			row, err := plan.SelectRowName(memberFn(shown[0].Key))
			if err == nil {
				fmt.Fprintf(&b, "```go\ntype %s struct {\n\t%s\n\t%s []%s `json:%q`\n}\n```\n", w.RowType, row, w.Field, child, w.Name)
			}
		}
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
	if si.Limit != "" {
		b.WriteString("```sql\nLIMIT ?\n```\nbinds: `" + si.Limit + "`\n\n")
	}

	var params strings.Builder
	for _, p := range si.Params {
		fmt.Fprintf(&params, ", %s %s", p.GoName, p.Type())
	}
	b.WriteString("```go\n")
	for _, m := range si.Members {
		model := m.Decl.Name.Name()
		if mn, err := naming.ModelName(m.Decl); err == nil {
			model = mn
		}
		row := model
		if r, err := plan.SelectRowName(memberFn(m.Key)); err == nil {
			row = r
		}
		effect := "rt.Query[" + row + "]"
		switch {
		case si.Lookup:
			effect = "rt.Lookup[" + row + "]"
		case si.With != nil && child != "":
			effect = fmt.Sprintf("rt.With[%s, %s, %s, %s]", row, child, strings.TrimPrefix(keyType[m.Key], "rt.Null["), si.With.RowType)
		}
		fmt.Fprintf(&b, "func %s%s(%s) %s\n",
			model, si.MethodSuffix, strings.TrimPrefix(params.String(), ", "), effect)
	}
	b.WriteString("```\n")
	return b.String()
}

func fieldTypeMD(m *check.TableInfo, f naming.FieldSig) string {
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
	head := "select " + d.Name.Name()
	if d.Projected() {
		if d.Star {
			cols := make([]string, len(d.Cols))
			for i, cr := range d.Cols {
				cols[i] = cr.String()
			}
			head += " (* \\ " + strings.Join(cols, " \\ ") + ")"
		} else {
			items := make([]string, len(d.Items))
			for i, item := range d.Items {
				items[i] = item.String()
			}
			head += " (" + strings.Join(items, ", ") + ")"
		}
	}
	head += " for " + d.Target.Name()
	for _, rel := range d.Joins {
		head += " join " + rel.Name()
	}
	if d.With != nil {
		head += " with " + d.With.Name()
	}
	return head
}

func (ix *voltIndex) setSym(path, name string, kinds ...string) voltSym {
	for _, k := range kinds {
		if _, ok := ix.defs[voltSym{k, path, name}]; ok {
			return voltSym{k, path, name}
		}
	}
	return voltSym{kinds[len(kinds)-1], path, name}
}

func groupHover(sym voltSym, def voltDef, ix *voltIndex) string {
	g := def.group
	if g == nil {
		return ""
	}
	decl := "group " + sym.name
	if text := ix.text(def.span.file); g.Pos().Offset() >= 0 && int(g.End().Offset()) <= len(text) && g.Pos().Offset() < g.End().Offset() {
		decl = strings.TrimSpace(text[g.Pos().Offset():g.End().Offset()])
	}
	var b strings.Builder
	b.WriteString("```volt\n" + decl + "\n```\n")
	if def.pkg == nil {
		return b.String()
	}
	gi := def.pkg.Groups[sym.name]
	if gi == nil || len(gi.Members) == 0 {
		return b.String()
	}
	names := make([]string, len(gi.Members))
	for i, m := range gi.Members {
		names[i] = "`" + m.Key + "`"
	}
	b.WriteString("\nMembers: " + strings.Join(names, ", ") + "\n")
	return b.String()
}

func predHover(sym voltSym, def voltDef, ix *voltIndex) string {
	p := def.pred
	if p == nil {
		return ""
	}
	md := "```volt\npred " + sym.name + "\n```\n"
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
	model, _ := naming.ModelName(t)
	fmt.Fprintf(&b, "**`table %s`** in package `%s`", t.Name.Name(), sym.pkg)
	if model != "" {
		fmt.Fprintf(&b, " — Go model `%s`", model)
	}
	b.WriteString("\n\n")

	pk, pkType := tablePK(def)
	if pk != "" {
		if pkType != "" {
			fmt.Fprintf(&b, "key: `%s` → route parameter type `%s`\n\n", pk, pkType)
		} else {
			fmt.Fprintf(&b, "key: `%s`\n\n", pk)
		}
	}

	b.WriteString("```volt\n")
	fmt.Fprintf(&b, "table %s {\n", t.Name.Name())
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
	fmt.Fprintf(&b, "**pipeline `%s`** — %d plug(s), outermost first\n\n```volt\n", sym.name, len(p.Plugs))
	fmt.Fprintf(&b, "pipeline %s {\n", sym.name)
	for _, plug := range p.Plugs {
		fmt.Fprintf(&b, "\tuse %s\n", plug.Ref.String())
	}
	b.WriteString("}\n```")
	return b.String()
}

// tablePK is a table's single-column key and the route parameter type
// it gives, nothing for a composite key or none (spec §columns,
// §table-resolution).
func tablePK(def voltDef) (name, goType string) {
	if def.table == nil || def.pkg == nil {
		return "", ""
	}
	key := tableKey(def.pkg, def.table)
	if len(key) != 1 {
		return "", ""
	}
	col := key[0]
	if lang.KnownParamType(col.Type.String()) {
		goType = col.Type.String()
	}
	return col.Name.Name(), goType
}

// tableKey is the columns of a table's primary key (spec §columns): the
// checked table's when the package is checked, else the pk columns of
// its body with its partials injected in source order, a column the
// body declares overriding a partial's.
func tableKey(pkg *lang.Package, d *ast.Table) []*ast.Column {
	if info := pkg.Schema(); info != nil {
		if ti := info.Table(d.Name.Name()); ti != nil && ti.Decl == d {
			var out []*ast.Column
			for _, cd := range ti.PK() {
				out = append(out, cd.Col)
			}
			return out
		}
	}
	partials := map[string]*ast.TablePartial{}
	for _, decl := range pkg.Merged().Decls {
		if p, ok := decl.(*ast.TablePartial); ok {
			partials[p.Name.Name()] = p
		}
	}
	var order []string
	byName := map[string]*ast.Column{}
	direct := map[string]bool{}
	add := func(col *ast.Column, fromBody bool) {
		name := col.Name.Name()
		if _, seen := byName[name]; !seen {
			order = append(order, name)
		} else if direct[name] && !fromBody {
			return
		}
		byName[name] = col
		direct[name] = direct[name] || fromBody
	}
	for _, item := range d.Body {
		switch item := item.(type) {
		case *ast.Column:
			add(item, true)
		case *ast.PartialRef:
			if p := partials[item.Name.Name()]; p != nil {
				for _, pit := range p.Body {
					if col, ok := pit.(*ast.Column); ok {
						add(col, false)
					}
				}
			}
		}
	}
	var out []*ast.Column
	for _, name := range order {
		if col := byName[name]; col.Settings.Get("pk") != nil {
			out = append(out, col)
		}
	}
	return out
}

var voltQualRefRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z0-9_]*)$`)

var voltResourcesRE = regexp.MustCompile(`^\s*resources\s+([A-Za-z0-9_]*)$`)

func (d *Document) voltComplete(prefix string, inPred bool) []protocol.CompletionItem {
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
	inExists := existsOpen(prefix)
	if m := voltExistsRE.FindStringSubmatch(prefix); m != nil && !inExists {
		return d.relationItems(m[1], m[2])
	}
	if partial, ok := predTermAt(prefix, inPred); ok {
		items := d.namedDefItems("pred", partial, protocol.CompletionItemKindFunction)
		if inExists {
			items = d.predsOver(d.existsTable(prefix), items)
		}
		if !inExists && strings.HasPrefix("exists", strings.ToLower(partial)) {
			items = append(items, keywordItems([]string{"exists "})...)
		}
		return items
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

var voltSelectForRE = regexp.MustCompile(`^\s*select\s+\w+\s+for\s+([A-Za-z0-9_]*)$`)

var voltWhereRE = regexp.MustCompile(`(?:\b(?:where|and|or|not)\s+|\bpred\s+[A-Za-z0-9_]+\s*\{\s*)([A-Za-z0-9_]*)$`)

var voltPredLineRE = regexp.MustCompile(`^\s*([A-Za-z0-9_]*)$`)

var voltExistsRE = regexp.MustCompile(`^\s*select\s+\w+.*?\bfor\s+([A-Za-z0-9_]+)\b.*\bexists\s+([A-Za-z0-9_]*)$`)

var voltPredParenRE = regexp.MustCompile(`\(\s*([A-Za-z0-9_]*)$`)

var voltPredOpenerRE = regexp.MustCompile(`(?:\b(?:where|and|or|not)|\{|\bexists\s+[A-Za-z0-9_]+)$`)

var voltPredContextRE = regexp.MustCompile(`\bwhere\b|\{`)

var voltExistsOpenRE = regexp.MustCompile(`\bexists\s+[A-Za-z0-9_]+\s*\(`)

// predTermAt is the partial word of prefix where a predicate term starts
// (spec §predicates): after where, and, or, not, a pred's { or a
// predicate's (, and, on a line of a pred body the { opened on an
// earlier line, at the line's start.
func predTermAt(prefix string, inPred bool) (string, bool) {
	if m := voltWhereRE.FindStringSubmatch(prefix); m != nil {
		return m[1], true
	}
	if m := voltPredLineRE.FindStringSubmatch(prefix); m != nil && inPred {
		return m[1], true
	}
	m := voltPredParenRE.FindStringSubmatchIndex(prefix)
	if m == nil {
		return "", false
	}
	before := strings.TrimRight(prefix[:m[0]], " \t(")
	opener := inPred && before == "" || voltPredOpenerRE.MatchString(before)
	if !opener || !inPred && !voltPredContextRE.MatchString(before) {
		return "", false
	}
	return prefix[m[2]:m[3]], true
}

var voltForRE = regexp.MustCompile(`^\s*select\s+\w+.*?\bfor\s+([A-Za-z0-9_]+)\b`)

// existsTable is the table the open exists of prefix leads to (spec
// §predicates), nil when the line names no table and relation of it.
func (d *Document) existsTable(prefix string) *check.TableInfo {
	info := d.vpkg.Schema()
	opens := voltExistsOpenRE.FindAllString(prefix, -1)
	target := voltForRE.FindStringSubmatch(prefix)
	if info == nil || len(opens) == 0 || target == nil {
		return nil
	}
	rel := strings.Fields(strings.TrimSuffix(opens[len(opens)-1], "("))[1]
	r, _ := info.Relation(target[1], rel)
	if r == nil {
		return nil
	}
	return r.Table
}

// predsOver keeps the preds of items that type over t inside an exists
// (spec §predicates): every bare column they name is t's, and none of
// them holds an exists, which does not nest.
func (d *Document) predsOver(t *check.TableInfo, items []protocol.CompletionItem) []protocol.CompletionItem {
	if t == nil {
		return nil
	}
	var out []protocol.CompletionItem
	fitting := map[string]bool{}
	for _, it := range items {
		if d.predFits(it.Label, t, fitting, map[string]bool{}) {
			out = append(out, it)
		}
	}
	return out
}

// predFits reports whether the pred name types over t inside an exists,
// remembering each pred's answer in fitting, so a pred named twice,
// directly or through two others, is read once and fits as it did;
// visiting holds the preds being read, which a cycle reaches again.
func (d *Document) predFits(name string, t *check.TableInfo, fitting, visiting map[string]bool) bool {
	if fits, known := fitting[name]; known {
		return fits
	}
	def, ok := d.vindex.defs[voltSym{"pred", d.vpkg.Path, name}]
	if !ok || def.pred == nil || visiting[name] {
		return false
	}
	visiting[name] = true
	defer delete(visiting, name)
	fits := true
	var walk func(x ast.PredExpr)
	col := func(cr *ast.ColRef) {
		if cr != nil && (cr.Qual != nil || t.Column(cr.Name.Name()) == nil) {
			fits = false
		}
	}
	walk = func(x ast.PredExpr) {
		switch x := x.(type) {
		case *ast.PredBinary:
			walk(x.X)
			walk(x.Y)
		case *ast.PredNot:
			walk(x.X)
		case *ast.PredParen:
			walk(x.X)
		case *ast.PredExists:
			fits = false
		case *ast.PredRef:
			fits = fits && d.predFits(x.Name.Name(), t, fitting, visiting)
		case *ast.PredCompare:
			for _, side := range []ast.Operand{x.X, x.Y} {
				if cr, ok := side.(*ast.ColRef); ok {
					col(cr)
				}
			}
		case *ast.PredIn:
			col(x.Col)
		case *ast.PredLike:
			col(x.Col)
		case *ast.PredNull:
			col(x.Col)
		}
	}
	walk(def.pred.X)
	fitting[name] = fits
	return fits
}

func existsOpen(prefix string) bool {
	opens := voltExistsOpenRE.FindAllStringIndex(prefix, -1)
	if len(opens) == 0 {
		return false
	}
	depth := 0
	for _, r := range prefix[opens[len(opens)-1][1]-1:] {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return false
			}
		}
	}
	return depth > 0
}

func (d *Document) relationItems(table, partial string) []protocol.CompletionItem {
	info := d.vpkg.Schema()
	if info == nil || info.Table(table) == nil {
		return nil
	}
	var items []protocol.CompletionItem
	for _, name := range info.RelationNames(table) {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
			items = append(items, item(name, protocol.CompletionItemKindField, "relation of "+table))
		}
	}
	return items
}

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
			detail := "table"
			if pk, goType := tablePK(def); pk != "" && goType != "" {
				detail = "table — key " + pk + " " + goType
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
