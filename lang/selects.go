// Groups, predicates and selects (docs/spec.md §groups to §selects-over-groups): resolution,
// the column-agreement rule, the typed closed expression language, and
// the lowering to the runtime's predicate trees, which the one renderer
// turns into the generated statements (§predicates rule 6).
package lang

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/rt"
)

// GroupInfo is one resolved Group (§groups): its declaration and member
// tables in first-addition order.
type GroupInfo struct {
	Decl    *ast.Group
	Members []*check.TableInfo
}

// SelectParam is one parameter of a generated select method, in first
// appearance order (§selects-over-groups).
type SelectParam struct {
	GoName string // Go parameter name, the name written after ':'
	GoType string // e.g. "string", "int64", "time.Time"
}

// SelectInfo is one checked Select (§selects-over-groups), lowered to the
// runtime's trees (§predicates rule 6), which the generator renders with
// each member's table and columns. The agreement rule
// (§selects-over-groups) is what makes one tree serve every member.
type SelectInfo struct {
	Decl         *ast.Select
	MethodSuffix string // GoName of the select name, e.g. "Rows"
	Members      []*check.TableInfo
	Where        rt.GenPred    // the where clause; the zero value when none
	Order        []rt.GenOrder // the order: terms, in declared order
	WhereSQL     string        // Where as the renderer spells it; "" when none
	Binds        []string      // the Go expressions behind WhereSQL's placeholders, in order
	OrderSQL     string        // Order as the renderer spells it; "" when none
	Params       []SelectParam
	Cols         []string // explicit projection, declared order (§selects-over-groups)
	Excluded     []string // star-form exclusions (§selects-over-groups)
	Shared       string   // shared row type name; "" unless explicit list
}

func (c *checker) dataQueries(pkg *Package) {
	c.pkg = pkg
	info := c.schemas[pkg.Path]

	pkg.Groups = map[string]*GroupInfo{}
	pkg.Preds = map[string]*ast.Pred{}
	groupDecls := map[string]*ast.Group{}
	var groupOrder []string
	var selects []*ast.Select

	for _, d := range pkg.merged.Decls {
		switch d := d.(type) {
		case *ast.Group:
			name := d.Name.Name()
			if _, dup := groupDecls[name]; dup {
				c.errorf(d.Name.Pos(), "groups", "group %q redeclared (§groups)", name)
				continue
			}
			groupDecls[name] = d
			groupOrder = append(groupOrder, name)
		case *ast.Pred:
			name := d.Name.Name()
			if _, dup := pkg.Preds[name]; dup {
				c.errorf(d.Name.Pos(), "predicates", "predicate %q redeclared (§predicates)", name)
				continue
			}
			pkg.Preds[name] = d
		case *ast.Select:
			selects = append(selects, d)
		}
	}

	if info == nil {
		return
	}
	for _, name := range groupOrder {
		c.groupResolve(name, groupDecls, info, map[string]bool{})
	}
	c.predRefCycles(pkg)

	minted := pkg.plan.Names()

	var memo *selectsMemo
	if c.memo != nil {
		memo = &c.memo.selects
		memo.Hits, memo.Misses = 0, 0
		memo.next = make(map[*ast.Select]*selectEntry, len(selects))
	}
	seen := map[string]*ast.Select{}
	for _, sel := range selects {
		if si := c.selectCheckMemo(sel, info, minted, memo); si != nil {
			for _, m := range si.Members {
				key := m.Key + "." + si.MethodSuffix
				if prev, dup := seen[key]; dup {
					c.errorf(sel.Name.Pos(), "selects-over-groups",
						"select %q collides with select %q on table %q — both generate %s%s (§selects-over-groups)",
						sel.Name.Name(), prev.Name.Name(), m.Decl.Name.Base(),
						modelOrBase(m), si.MethodSuffix)
					break
				}
				seen[key] = sel
			}
			pkg.Selects = append(pkg.Selects, si)
		}
	}
	if memo != nil {
		memo.prev, memo.next = memo.next, nil
	}
	pkg.selectIndex()
}

type nameLookup struct {
	name, desc string
	dup        bool
}

type nameAdd struct{ name, desc string }

type selectsMemo struct {
	prev, next   map[*ast.Select]*selectEntry
	Hits, Misses int
}

type selectEntry struct {
	members []*check.TableInfo
	models  []any
	preds   map[string]*ast.Pred
	lookups []nameLookup
	adds    []nameAdd
	si      *SelectInfo
	diags   []diag.Diagnostic
}

func (e *selectEntry) holds(members []*check.TableInfo, models []any, preds map[string]*ast.Pred, names *golang.Names) bool {
	if len(members) != len(e.members) || len(preds) != len(e.preds) {
		return false
	}
	for i, m := range members {
		if m != e.members[i] || models[i] != e.models[i] {
			return false
		}
	}
	for name, d := range preds {
		if old, ok := e.preds[name]; !ok || old != d {
			return false
		}
	}
	for _, l := range e.lookups {
		if desc, dup := names.Lookup(l.name); dup != l.dup || desc != l.desc {
			return false
		}
	}
	return true
}

func (c *checker) selectCheckMemo(sel *ast.Select, info *check.Info, minted *golang.Names, memo *selectsMemo) *SelectInfo {
	members, ok := c.selectTarget(sel, info)
	if !ok {
		return nil
	}
	models := make([]memberModel, len(members))
	refs := make([]any, len(members))
	for i, m := range members {
		model, fields, err := c.pkg.plan.ModelFields(m.Key)
		models[i] = memberModel{model: model, fields: fields, err: err}
		refs[i] = c.pkg.plan.ModelRef(m.Key)
	}
	var where []ast.PredExpr
	if sel.Where != nil {
		where = append(where, sel.Where)
	}
	preds := predsIn(where, c.pkg.Preds)
	if memo != nil {
		if e := memo.prev[sel]; e != nil && e.holds(members, refs, preds, minted) {
			for _, a := range e.adds {
				minted.Add(a.name, a.desc)
			}
			c.diags = append(c.diags, e.diags...)
			memo.next[sel] = e
			memo.Hits++
			return e.si
		}
	}
	si, lookups, adds, diags := selectCheck(sel, members, models, preds, minted)
	for _, a := range adds {
		minted.Add(a.name, a.desc)
	}
	c.diags = append(c.diags, diags...)
	if memo != nil {
		memo.next[sel] = &selectEntry{members: members, models: refs, preds: preds, lookups: lookups, adds: adds, si: si, diags: diags}
		memo.Misses++
	}
	return si
}

func (c *checker) selectTarget(sel *ast.Select, info *check.Info) ([]*check.TableInfo, bool) {
	want := sel.Target.Name()
	var members []*check.TableInfo
	if g, isGroup := c.pkg.Groups[want]; isGroup {
		members = g.Members
	} else if ti := tableByBase(info, want); ti != nil {
		members = []*check.TableInfo{ti}
	} else if tg := info.TableGroup(want); tg != nil {
		members = tg.Members
		if len(members) == 0 {
			c.errorf(sel.Target.Pos(), "selects-over-groups", "TableGroup %q has no members to select from (§selects-over-groups)", want)
			return nil, false
		}
	} else {
		c.nameMiss(sel.Target, info, "selects-over-groups", "select target")
		return nil, false
	}
	return members, len(members) > 0
}

type memberModel struct {
	model  string
	fields []golang.FieldSig
	err    error
}

type nameScope interface {
	Lookup(name string) (desc string, dup bool)
}

type selectLowering struct {
	sel     *ast.Select
	members []*check.TableInfo
	models  []memberModel
	preds   map[string]*ast.Pred
	names   nameScope

	lookups []nameLookup
	adds    []nameAdd
	diags   []diag.Diagnostic
}

func (l *selectLowering) errorf(pos token.Position, format string, args ...any) {
	l.diags = append(l.diags, diag.Errorf(pos, "spec/selects-over-groups", format, args...))
}

func (l *selectLowering) lookup(name string) (string, bool) {
	for _, a := range l.adds {
		if a.name == name {
			return a.desc, true
		}
	}
	desc, dup := l.names.Lookup(name)
	l.lookups = append(l.lookups, nameLookup{name, desc, dup})
	return desc, dup
}

func (l *selectLowering) add(name, desc string) {
	l.adds = append(l.adds, nameAdd{name, desc})
}

type selectMember struct {
	sel    *SelectInfo
	member *check.TableInfo
}

func (p *Package) selectIndex() {
	p.selectByMethod = make(map[string]selectMember, len(p.Selects))
	p.selectByName = make(map[string]*SelectInfo, len(p.Selects))
	for _, si := range p.Selects {
		p.selectByName[si.Decl.Name.Name()] = si
		for _, m := range si.Members {
			name := modelOrBase(m) + si.MethodSuffix
			if _, taken := p.selectByMethod[name]; !taken {
				p.selectByMethod[name] = selectMember{si, m}
			}
		}
	}
}

func modelOrBase(t *check.TableInfo) string {
	if m, err := golang.ModelName(t.Decl); err == nil {
		return m
	}
	return t.Decl.Name.Base()
}

func (c *checker) groupResolve(name string, decls map[string]*ast.Group, info *check.Info, path map[string]bool) *GroupInfo {
	if g, done := c.pkg.Groups[name]; done {
		return g
	}
	d := decls[name]
	if path[name] {
		c.errorf(d.Name.Pos(), "groups", "group %q is part of a reference cycle (§groups)", name)
		c.pkg.Groups[name] = &GroupInfo{Decl: d}
		return c.pkg.Groups[name]
	}
	path[name] = true
	defer delete(path, name)

	g := &GroupInfo{Decl: d}
	index := map[string]int{}
	add := func(pos token.Position, ti *check.TableInfo) {
		if i, present := index[ti.Key]; present && i >= 0 {
			c.errorf(pos, "groups", "table %q is already a member (§groups)", ti.Decl.Name.Base())
			return
		}
		index[ti.Key] = len(g.Members)
		g.Members = append(g.Members, ti)
	}
	remove := func(pos token.Position, ti *check.TableInfo) {
		i, present := index[ti.Key]
		if !present || i < 0 {
			c.errorf(pos, "groups", "table %q is not a member; nothing to remove (§groups)", ti.Decl.Name.Base())
			return
		}
		g.Members = append(g.Members[:i], g.Members[i+1:]...)
		for k, j := range index {
			if j > i {
				index[k] = j - 1
			}
		}
		index[ti.Key] = -1
	}

	apply := func(pos token.Position, neg bool, ti *check.TableInfo) {
		if neg {
			remove(pos, ti)
		} else {
			add(pos, ti)
		}
	}
	for _, term := range d.Terms {
		for _, id := range term.Names {
			want := id.Name()
			if ti := tableByBase(info, want); ti != nil {
				apply(id.Pos(), term.Neg, ti)
				continue
			}
			if _, isGroup := decls[want]; isGroup {
				sg := c.groupResolve(want, decls, info, path)
				for _, ti := range sg.Members {
					apply(id.Pos(), term.Neg, ti)
				}
				continue
			}
			if tg := info.TableGroup(want); tg != nil {
				for _, ti := range tg.Members {
					apply(id.Pos(), term.Neg, ti)
				}
				continue
			}
			c.nameMiss(id, info, "groups", "group term")
		}
	}
	if len(g.Members) == 0 {
		c.errorf(d.Name.Pos(), "groups", "group %q has no members (§groups)", name)
	}
	c.pkg.Groups[name] = g
	return g
}

func tableByBase(info *check.Info, want string) *check.TableInfo {
	return info.TableByBase(want)
}

func (c *checker) nameMiss(id *ast.Ident, info *check.Info, section, what string) {
	want := id.Name()
	for _, ti := range info.Tables {
		if strings.EqualFold(ti.Decl.Name.Base(), want) {
			c.errorf(id.Pos(), section, "unknown %s %q; did you mean %q? names are case-sensitive (§groups)", what, want, ti.Decl.Name.Base())
			return
		}
	}
	for _, ti := range info.Tables {
		if m, err := golang.ModelName(ti.Decl); err == nil && m == want {
			c.errorf(id.Pos(), section, "unknown %s %q; %q is the model of table %q — name the table (§groups)", what, want, want, ti.Decl.Name.Base())
			return
		}
	}
	c.errorf(id.Pos(), section, "unknown %s %q: no such table, group or TableGroup in package %q (§groups)", what, want, c.pkg.Path)
}

func (c *checker) predRefCycles(pkg *Package) {
	state := map[string]int{}
	var visit func(name string, at token.Position)
	visit = func(name string, at token.Position) {
		switch state[name] {
		case 1:
			c.errorf(at, "predicates", "predicate %q is part of a reference cycle (§predicates)", name)
			return
		case 2:
			return
		}
		state[name] = 1
		if d := pkg.Preds[name]; d != nil {
			predWalk(d.X, func(r *ast.PredRef) {
				if pkg.Preds[r.Name.Name()] == nil {
					c.errorf(r.Name.Pos(), "predicates", "unknown predicate %q (§predicates)", r.Name.Name())
					return
				}
				visit(r.Name.Name(), r.Name.Pos())
			})
		}
		state[name] = 2
	}
	names := make([]string, 0, len(pkg.Preds))
	for name := range pkg.Preds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		visit(name, pkg.Preds[name].Name.Pos())
	}
}

func predWalk(x ast.PredExpr, f func(*ast.PredRef)) {
	switch x := x.(type) {
	case *ast.PredBinary:
		predWalk(x.X, f)
		predWalk(x.Y, f)
	case *ast.PredNot:
		predWalk(x.X, f)
	case *ast.PredParen:
		predWalk(x.X, f)
	case *ast.PredRef:
		f(x)
	}
}

type typeClass int

const (
	classForbidden typeClass = iota
	classNumeric
	classText
	classTime
	classBool
	classEqOnly
)

func classOfType(t check.ColumnType) typeClass {
	return classOfName(t.Class())
}

func classOfName(class string) typeClass {
	switch class {
	case "numeric":
		return classNumeric
	case "text":
		return classText
	case "time":
		return classTime
	case "bool":
		return classBool
	case "eq":
		return classEqOnly
	}
	return classForbidden
}

type colBinding struct {
	name      string
	goType    string
	storageGo string
	class     typeClass
	storage   typeClass
}

func (b *colBinding) litClass() typeClass {
	if b.class != classEqOnly {
		return b.class
	}
	if b.storage == classTime {
		return classForbidden
	}
	return b.storage
}

func selectCheck(sel *ast.Select, members []*check.TableInfo, models []memberModel, preds map[string]*ast.Pred, names nameScope) (*SelectInfo, []nameLookup, []nameAdd, []diag.Diagnostic) {
	l := &selectLowering{sel: sel, members: members, models: models, preds: preds, names: names}
	si := l.check()
	return si, l.lookups, l.adds, l.diags
}

func (l *selectLowering) check() *SelectInfo {
	sel := l.sel
	si := &SelectInfo{Decl: sel, Members: l.members}

	suffix, err := golang.GoName(sel.Name.Name())
	if err != nil {
		l.errorf(sel.Name.Pos(), "select name %q does not map to a Go identifier: %v (§selects-over-groups)", sel.Name.Name(), err)
		return nil
	}
	si.MethodSuffix = suffix
	if crudMethodSuffixes[suffix] {
		l.errorf(sel.Name.Pos(), "select name %q collides with the generated CRUD surface (§selects-over-groups)", suffix)
		return nil
	}

	if sel.Projected() && !l.projectionCheck(si) {
		return nil
	}

	env := &selectEnv{
		l:       l,
		sel:     sel,
		members: si.Members,
		cols:    map[string]*colBinding{},
		params:  map[string]*SelectParam{},
	}

	var where rt.GenPred
	if sel.Where != nil {
		where = env.exprLower(sel.Where, map[string]bool{})
	}

	order := l.selectSettings(env)
	if env.failed {
		return nil
	}
	whereSQL, slots, err := rt.WhereRender([]rt.GenPred{where})
	if err != nil {
		l.errorf(sel.Name.Pos(), "select %q: %v (§predicates)", sel.Name.Name(), err)
		return nil
	}
	binds := make([]string, len(slots))
	for i, slot := range slots {
		binds[i] = fmt.Sprint(slot)
	}
	si.Where = where
	si.Order = order
	si.WhereSQL = whereSQL
	si.Binds = binds
	si.OrderSQL = rt.OrderRender(order)
	si.Params = env.paramList
	return si
}

var crudMethodSuffixes = map[string]bool{
	"Get": true, "GetMany": true, "List": true, "Create": true,
	"Update": true, "Delete": true, "Count": true, "Exists": true,
	"Query": true, "DeleteWhere": true, "UpdateWhere": true,
	"Limit": true, "Offset": true, "OrderBy": true,
	"Changeset": true, "Column": true, "Params": true, "New": true,
}

func (l *selectLowering) projectionCheck(si *SelectInfo) bool {
	sel := l.sel
	type memberFields struct {
		ti     *check.TableInfo
		model  string
		fields []golang.FieldSig
		byCol  map[string]golang.FieldSig
	}
	members := make([]memberFields, 0, len(si.Members))
	for i, m := range si.Members {
		model, fields, err := l.models[i].model, l.models[i].fields, l.models[i].err
		if err != nil {
			l.errorf(sel.Name.Pos(), "select %q: %v (§selects-over-groups)", sel.Name.Name(), err)
			return false
		}
		byCol := make(map[string]golang.FieldSig, len(fields))
		for _, f := range fields {
			byCol[f.Col] = f
		}
		members = append(members, memberFields{m, model, fields, byCol})
	}

	ok := true
	seen := map[string]bool{}
	for _, id := range sel.Cols {
		name := id.Name()
		if seen[name] {
			l.errorf(id.Pos(), "column %q appears twice in the projection (§selects-over-groups)", name)
			ok = false
			continue
		}
		seen[name] = true

		var missing []string
		types := map[string][]string{}
		for _, mf := range members {
			f, has := mf.byCol[name]
			if !has {
				missing = append(missing, mf.ti.Decl.Name.Base())
				continue
			}
			types[f.Type] = append(types[f.Type], mf.ti.Decl.Name.Base())
		}
		if len(missing) > 0 {
			l.errorf(id.Pos(), "column %q is missing from %s — every member of %q must have it (§selects-over-groups)",
				name, tableList(missing), sel.Target.Name())
			ok = false
			continue
		}
		if !sel.Star && len(types) > 1 {
			var goTypes []string
			for t := range types {
				goTypes = append(goTypes, t)
			}
			sort.Strings(goTypes)
			var parts []string
			for _, t := range goTypes {
				sort.Strings(types[t])
				parts = append(parts, fmt.Sprintf("%s in %s", t, tableList(types[t])))
			}
			l.errorf(id.Pos(), "column %q disagrees on field type across %q: %s — a shared row type needs one type, nullability included (§selects-over-groups)",
				name, sel.Target.Name(), strings.Join(parts, "; "))
			ok = false
			continue
		}
	}
	if !ok {
		return false
	}

	cols := make([]string, len(sel.Cols))
	for i, id := range sel.Cols {
		cols[i] = id.Name()
	}

	if !sel.Star {
		si.Cols = cols
		si.Shared = si.MethodSuffix
		if prev, dup := l.lookup(si.Shared); dup {
			l.errorf(sel.Name.Pos(), "select %q mints the shared row type %s, which collides with %s (§selects-over-groups)",
				sel.Name.Name(), si.Shared, prev)
			return false
		}
		for _, mf := range members {
			for _, f := range mf.fields {
				if mf.model+f.Name == si.Shared {
					l.errorf(sel.Name.Pos(), "select %q mints the shared row type %s, which collides with the dynamic column handle for %s.%s (§selects-over-groups)",
						sel.Name.Name(), si.Shared, mf.ti.Decl.Name.Base(), f.Col)
					return false
				}
			}
		}
		l.add(si.Shared, fmt.Sprintf("select %q's shared row type", sel.Name.Name()))
		return true
	}

	si.Excluded = cols
	for _, mf := range members {
		kept := 0
		for _, f := range mf.fields {
			if !seen[f.Col] {
				kept++
			}
			if f.Name == si.MethodSuffix {
				l.errorf(sel.Name.Pos(), "select %q mints %s%s, which collides with the dynamic column handle for %s.%s (§selects-over-groups)",
					sel.Name.Name(), mf.model, si.MethodSuffix, mf.ti.Decl.Name.Base(), f.Col)
				return false
			}
		}
		if kept == 0 {
			l.errorf(sel.Name.Pos(), "select %q excludes every column of %q — nothing is left to project (§selects-over-groups)",
				sel.Name.Name(), mf.ti.Decl.Name.Base())
			return false
		}
		name := mf.model + si.MethodSuffix
		if prev, dup := l.lookup(name); dup {
			l.errorf(sel.Name.Pos(), "select %q mints the row type %s, which collides with %s (§selects-over-groups)",
				sel.Name.Name(), name, prev)
			return false
		}
		l.add(name, fmt.Sprintf("select %q's row type for table %q", sel.Name.Name(), mf.ti.Decl.Name.Base()))
	}
	return true
}

func (l *selectLowering) selectSettings(env *selectEnv) []rt.GenOrder {
	sel := l.sel
	if sel.Settings == nil {
		return nil
	}
	var terms []rt.GenOrder
	for _, s := range sel.Settings.Settings {
		switch strings.ToLower(s.Name) {
		case "order":
			list, ok := s.Value.(*ast.IdentList)
			if !ok {
				l.errorf(s.Pos(), "order: takes a parenthesized column list (§selects-over-groups)")
				continue
			}
			seenOrder := map[string]bool{}
			for i, id := range list.Names {
				b := env.columnBind(id)
				if b == nil {
					continue
				}
				if seenOrder[b.name] {
					l.errorf(id.Pos(), "column %q appears twice in order: (§selects-over-groups)", b.name)
					continue
				}
				seenOrder[b.name] = true
				if b.class == classEqOnly {
					l.errorf(id.Pos(), "column %q has declared type %s; a declared type cannot be ordered (§selects-over-groups)", b.name, b.goType)
					continue
				}
				if b.class != classNumeric && b.class != classTime && b.class != classText {
					l.errorf(id.Pos(), "column %q (%s) is not orderable (§selects-over-groups)", b.name, b.goType)
					continue
				}
				mod := list.Mods[i]
				if mod == nil {
					l.errorf(id.Pos(), "order: every column states its direction — write %q or %q (§selects-over-groups)", id.Name()+" asc", id.Name()+" desc")
					continue
				}
				desc := false
				switch strings.ToLower(mod.Name()) {
				case "asc":
				case "desc":
					desc = true
				default:
					l.errorf(mod.Pos(), "order direction is asc or desc, not %q (§selects-over-groups)", mod.Name())
					continue
				}
				terms = append(terms, rt.OrderTerm(b.name, desc))
			}
		default:
			l.errorf(s.Pos(), "unknown select setting %q; selects take [order: (...)] (§selects-over-groups)", s.Name)
		}
	}
	return terms
}

type selectEnv struct {
	l         *selectLowering
	sel       *ast.Select
	members   []*check.TableInfo
	cols      map[string]*colBinding
	params    map[string]*SelectParam
	paramList []SelectParam
	failed    bool
}

func (e *selectEnv) errorf(pos token.Position, format string, args ...any) {
	e.failed = true
	e.l.errorf(pos, format, args...)
}

func (e *selectEnv) columnBind(id *ast.Ident) *colBinding {
	name := id.Name()
	if b, done := e.cols[name]; done {
		return b
	}
	var missing []string
	types := map[string][]string{}
	classes := map[typeClass]bool{}
	var class, storage typeClass
	var storageGo string
	for _, m := range e.members {
		cd := m.Column(name)
		if cd == nil {
			missing = append(missing, m.Decl.Name.Base())
			continue
		}
		declType := cd.Col.Type.String()
		if cd.Type.Enum != nil {
			e.errorf(id.Pos(), "column %q of table %q is enum-typed (%s); predicates do not compare enums in v1 — filter on a text column or use the dynamic layer (§predicates, hypotheses H5)", name, m.Decl.Name.Base(), declType)
			return nil
		}
		goType := cd.Type.Go
		types[goType] = append(types[goType], m.Decl.Name.Base())
		class = classOfType(cd.Type)
		storage = classOfName(cd.Type.StorageClass())
		storageGo = cd.Type.Storage
		classes[class] = true
	}
	if len(missing) > 0 {
		e.errorf(id.Pos(), "column %q is missing from %s — every member of %q must have it (§selects-over-groups)",
			name, tableList(missing), e.sel.Target.Name())
		return nil
	}
	if len(types) > 1 {
		var parts []string
		var goTypes []string
		for t := range types {
			goTypes = append(goTypes, t)
		}
		sort.Strings(goTypes)
		for _, t := range goTypes {
			sort.Strings(types[t])
			parts = append(parts, fmt.Sprintf("%s in %s", t, tableList(types[t])))
		}
		e.errorf(id.Pos(), "column %q disagrees on type across %q: %s — the predicate cannot be checked for every member (§selects-over-groups)",
			name, e.sel.Target.Name(), strings.Join(parts, "; "))
		return nil
	}
	if len(classes) > 1 {
		if classes[classEqOnly] {
			class = classEqOnly
		}
	}
	if class == classForbidden {
		e.errorf(id.Pos(), "column %q ([]byte) cannot appear in a predicate (§predicates)", name)
		return nil
	}
	var goType string
	for t := range types {
		goType = t
	}
	b := &colBinding{name: name, goType: goType, storageGo: storageGo, class: class, storage: storage}
	e.cols[name] = b
	return b
}

func tableList(names []string) string {
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ", "), len(names)-3)
	}
	return strings.Join(names, ", ")
}

func (e *selectEnv) paramBind(p *ast.Param, goType string) string {
	name := p.Name.Name()
	if prev, seen := e.params[name]; seen {
		if prev.GoType != goType {
			e.errorf(p.Pos(), "parameter :%s used as %s here but %s earlier — one name, one type (§predicates)", name, goType, prev.GoType)
		}
		return name
	}
	if !goParamOK(name) {
		e.errorf(p.Name.Pos(), "parameter name %q does not work as a Go parameter (§predicates)", name)
		return name
	}
	sp := SelectParam{GoName: name, GoType: goType}
	e.params[name] = &sp
	e.paramList = append(e.paramList, sp)
	return name
}

func goParamOK(name string) bool {
	if name == "" || goKeywords[name] || reservedParamNames[name] || name == "ctx" || name == "q" {
		return false
	}
	return goIdentOK(name)
}

func (e *selectEnv) exprLower(x ast.PredExpr, preds map[string]bool) rt.GenPred {
	switch x := x.(type) {
	case *ast.PredBinary:
		operands := e.operandsLower(x, x.Op, preds, nil)
		if x.Op == "or" {
			return rt.Or(operands...)
		}
		return rt.And(operands...)
	case *ast.PredNot:
		return rt.Not(e.exprLower(x.X, preds))
	case *ast.PredParen:
		return e.exprLower(x.X, preds)
	case *ast.PredRef:
		name := x.Name.Name()
		d := e.l.preds[name]
		if d == nil {
			e.errorf(x.Name.Pos(), "unknown predicate %q (§predicates)", name)
			return rt.GenPred{}
		}
		if preds[name] {
			return rt.GenPred{}
		}
		preds[name] = true
		p := e.exprLower(d.X, preds)
		delete(preds, name)
		return p
	case *ast.PredCompare:
		return e.compareLower(x)
	case *ast.PredIn:
		return e.inLower(x)
	case *ast.PredLike:
		return e.likeLower(x)
	case *ast.PredNull:
		b := e.columnBind(x.Col)
		if b == nil {
			return rt.GenPred{}
		}
		if x.Not {
			return rt.PredNotNull(b.name)
		}
		return rt.PredNull(b.name)
	}
	return rt.GenPred{}
}

func (e *selectEnv) operandsLower(x ast.PredExpr, op string, preds map[string]bool, acc []rt.GenPred) []rt.GenPred {
	if b, ok := x.(*ast.PredBinary); ok && b.Op == op {
		acc = e.operandsLower(b.X, op, preds, acc)
		return e.operandsLower(b.Y, op, preds, acc)
	}
	return append(acc, e.exprLower(x, preds))
}

var compareSQL = map[token.Kind]string{
	token.EQ: "=", token.NEQ: "!=", token.LT: "<",
	token.LE: "<=", token.GT: ">", token.GE: ">=",
}

func compareFlip(op token.Kind) token.Kind {
	switch op {
	case token.LT:
		return token.GT
	case token.LE:
		return token.GE
	case token.GT:
		return token.LT
	case token.GE:
		return token.LE
	}
	return op
}

func (e *selectEnv) compareLower(x *ast.PredCompare) rt.GenPred {
	ordering := x.Op != token.EQ && x.Op != token.NEQ

	var b *colBinding
	colCount := 0
	for _, side := range []ast.Operand{x.X, x.Y} {
		if cr, isCol := side.(*ast.ColRef); isCol {
			colCount++
			if bb := e.columnBind(cr.Name); bb != nil {
				b = bb
			}
		}
	}
	if colCount == 0 {
		e.errorf(x.Pos(), "a comparison needs a column on one side (§predicates)")
		return rt.GenPred{}
	}
	if b == nil {
		return rt.GenPred{}
	}
	if ordering {
		switch b.class {
		case classNumeric, classTime:
		case classEqOnly:
			e.errorf(x.Pos(), "column %q has declared type %s; a declared type compares with =, !=, in and is null only (§predicates)", b.name, b.goType)
			return rt.GenPred{}
		default:
			e.errorf(x.Pos(), "column %q (%s) is not orderable; < <= > >= need numeric or date/time operands (§predicates)", b.name, b.goType)
			return rt.GenPred{}
		}
	}

	left, right, op := x.X, x.Y, x.Op
	if _, isCol := left.(*ast.ColRef); !isCol {
		left, right, op = right, left, compareFlip(op)
	}
	col := e.columnBind(left.(*ast.ColRef).Name)
	if col == nil {
		return rt.GenPred{}
	}
	var p rt.GenPred
	var err error
	switch o := right.(type) {
	case *ast.ColRef:
		other := e.columnBind(o.Name)
		if other == nil {
			return rt.GenPred{}
		}
		if other.goType != col.goType {
			e.errorf(o.Pos(), "cannot compare column %q (%s) with column %q (%s) (§predicates)", col.name, col.goType, other.name, other.goType)
			return rt.GenPred{}
		}
		p, err = rt.PredColCmp(col.name, compareSQL[op], other.name)
	case *ast.Param:
		p, err = rt.PredCmp(col.name, compareSQL[op], e.paramBind(o, col.goType))
	case *ast.Lit:
		slot, ok := e.litSlot(o, col)
		if !ok {
			return rt.GenPred{}
		}
		p, err = rt.PredCmp(col.name, compareSQL[op], slot)
	}
	if err != nil {
		e.errorf(x.Pos(), "%v (§predicates)", err)
		return rt.GenPred{}
	}
	return p
}

func (e *selectEnv) litSlot(l *ast.Lit, b *colBinding) (string, bool) {
	lit := b.litClass()
	switch l.Tok.Kind {
	case token.NUMBER:
		if lit != classNumeric {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the number %s (§predicates)", b.name, b.goType, l.Tok.Val)
			return "", false
		}
		slot, err := numberGo(l.Tok.Val, b.storageGo)
		if err != nil {
			e.errorf(l.Pos(), "literal %s does not fit %s, the storage type of column %q (§predicates)", l.Tok.Val, b.storageGo, b.name)
			return "", false
		}
		return slot, true
	case token.STRING:
		if lit != classText && lit != classTime {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the string %q (§predicates)", b.name, b.goType, l.Tok.Val)
			return "", false
		}
		return fmt.Sprintf("%q", l.Tok.Val), true
	default:
		if lit != classBool {
			e.errorf(l.Pos(), "column %q (%s) is not boolean (§predicates)", b.name, b.goType)
			return "", false
		}
		if strings.EqualFold(l.Tok.Val, "true") {
			return "true", true
		}
		return "false", true
	}
}

func numberGo(val, storage string) (string, error) {
	if storage == "float64" {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return "", err
		}
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s, nil
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n, 10), nil
}

func (e *selectEnv) inLower(x *ast.PredIn) rt.GenPred {
	b := e.columnBind(x.Col)
	if b == nil {
		return rt.GenPred{}
	}
	if x.List != nil {
		if b.class == classTime {
			e.errorf(x.Col.Pos(), "column %q (%s) cannot take a list parameter: JSON carries no date/time value (§predicates)", b.name, b.goType)
			return rt.GenPred{}
		}
		if b.class == classEqOnly && b.storage == classTime {
			e.errorf(x.Col.Pos(), "column %q has declared type %s stored as time.Time; it cannot take a list parameter: JSON carries no date/time value (§predicates)", b.name, b.goType)
			return rt.GenPred{}
		}
		return rt.PredInList(b.name, "rt.JSONList("+e.paramBind(x.List, "[]"+b.goType)+")")
	}
	slots := make([]any, 0, len(x.Items))
	ok := true
	for _, it := range x.Items {
		slot, good := e.litSlot(it, b)
		if !good {
			ok = false
			continue
		}
		slots = append(slots, slot)
	}
	if !ok {
		return rt.GenPred{}
	}
	return rt.PredIn(b.name, slots...)
}

func (e *selectEnv) likeLower(x *ast.PredLike) rt.GenPred {
	b := e.columnBind(x.Col)
	if b == nil {
		return rt.GenPred{}
	}
	if b.class != classText {
		e.errorf(x.Col.Pos(), "like needs a text column; %q is %s (§predicates)", b.name, b.goType)
		return rt.GenPred{}
	}
	switch p := x.Pattern.(type) {
	case *ast.Lit:
		return rt.PredLike(b.name, fmt.Sprintf("%q", p.Tok.Val))
	case *ast.Param:
		return rt.PredLike(b.name, e.paramBind(p, "string"))
	}
	return rt.GenPred{}
}

// SelectFns lowers the package's checked selects into per-member
// generator instructions (§selects-over-groups).
func (p *Package) SelectFns() []golang.SelectFn {
	var out []golang.SelectFn
	for _, si := range p.Selects {
		for _, m := range si.Members {
			fn := golang.SelectFn{
				TableKey:     m.Key,
				MethodSuffix: si.MethodSuffix,
				Where:        si.Where,
				Order:        si.Order,
				Cols:         si.Cols,
				Excluded:     si.Excluded,
				SharedType:   si.Shared,
			}
			for _, prm := range si.Params {
				fn.Params = append(fn.Params, golang.SelectParam(prm))
			}
			out = append(out, fn)
		}
	}
	return out
}

func predsIn(exprs []ast.PredExpr, all map[string]*ast.Pred) map[string]*ast.Pred {
	out := map[string]*ast.Pred{}
	var walk func(x ast.PredExpr)
	walk = func(x ast.PredExpr) {
		switch x := x.(type) {
		case *ast.PredBinary:
			walk(x.X)
			walk(x.Y)
		case *ast.PredNot:
			walk(x.X)
		case *ast.PredParen:
			walk(x.X)
		case *ast.PredRef:
			name := x.Name.Name()
			if _, seen := out[name]; seen {
				return
			}
			d := all[name]
			out[name] = d
			if d != nil {
				walk(d.X)
			}
		}
	}
	for _, x := range exprs {
		walk(x)
	}
	return out
}
