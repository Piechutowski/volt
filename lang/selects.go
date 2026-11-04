// Groups, predicates and selects (docs/spec.md §groups to §selects-over-groups): resolution,
// the column-agreement rule, the typed closed expression language, and
// the lowering to the runtime's predicate trees, which the one renderer
// turns into the generated statements (§predicates).
package lang

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
	"github.com/Piechutowski/volt/nao/rt"
)

// GroupInfo is one resolved Group (§groups): its declaration and member
// tables in the order of their last addition.
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
// runtime's trees (§predicates), which the generator renders with
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
	Limit        string        // the Go expression LIMIT ? binds, the literal or the parameter; "" when none (§selects-over-groups)
	LimitParam   string        // the limit parameter, "" when the limit is a literal or absent
	Params       []SelectParam
	Items        []naming.SelectItem // explicit projection, declared order (§selects-over-groups)
	Group        []string            // the group: items as the statement names them (§selects-over-groups)
	Lookup       bool                // aggregates without group:, one row (§selects-over-groups)
	Excluded     []string            // star-form exclusions (§selects-over-groups)
	Shared       string              // shared row type name; "" unless explicit list
	Unresolved   bool                // a column of unknown type stopped the lowering (§columns); never generated
	UnknownCols  []string            // the columns of unknown type behind Unresolved, sorted
	Joins        []JoinInfo          // the relations joined, in declaration order (§selects-over-groups)
	With         *WithInfo           // the nesting, nil when none (§selects-over-groups)

	droppedParams map[string]bool
}

// JoinInfo is one join of a select (§selects-over-groups): the relation
// joined, which is the statement's alias, the table it leads to, whether
// it joins LEFT, and the column pairs matched, the target's own column
// first and the joined table's second, as the relation lists them.
type JoinInfo struct {
	Name  string
	Table *check.TableInfo
	Left  bool
	On    [][2]string
}

func relationEqual(a, b *check.RelationInfo) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Name == b.Name && a.Forward == b.Forward && a.Table == b.Table && a.Optional == b.Optional && slices.Equal(a.On, b.On)
}

// WithInfo is the nesting of a select (§selects-over-groups): the reverse
// relation, its table, the key columns, the parent's and the child's,
// and the Go names of the field holding the children and of the row
// type.
type WithInfo struct {
	Name      string
	Table     *check.TableInfo
	ParentKey string
	ChildKey  string
	Field     string
	RowType   string
}

// joinTarget is one join as selectCheckMemo resolves it: the relation,
// nil when the target has none of that name or the name is ambiguous
// (why says which), and the joined table's model.
type joinTarget struct {
	rel   *check.RelationInfo
	why   string
	model memberModel
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
		if info.Table(name) != nil {
			c.errorf(groupDecls[name].Name.Pos(), "groups", "group %q takes the name of a table: tables and groups share one namespace (§groups)", name)
		}
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
				method := naming.SelectMethodName(pkg.modelOf(m), si.MethodSuffix)
				what, dup := pkg.plan.QueryName(method)
				if !dup {
					what, dup = minted.Generated(method)
				}
				if dup {
					c.errorf(sel.Name.Pos(), "selects-over-groups",
						"select %q mints %s for table %q, which is already %s (§selects-over-groups)",
						sel.Name.Name(), method, m.Decl.Name.Name(), what)
					break
				}
				if prev, dup := seen[method]; dup {
					c.errorf(sel.Name.Pos(), "selects-over-groups",
						"select %q collides with select %q: both generate %s (§selects-over-groups)",
						sel.Name.Name(), prev.Name.Name(), method)
					break
				}
				seen[method] = sel
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
	joins   []*check.RelationInfo
	joined  []any
	with    *check.RelationInfo
	withRef any
	whys    []string // why each join or the with resolved to no relation, the relation names last
	preds   map[string]*ast.Pred
	lookups []nameLookup
	adds    []nameAdd
	si      *SelectInfo
	diags   []diag.Diagnostic
}

func (e *selectEntry) holds(members []*check.TableInfo, models []any, joins []*check.RelationInfo, joined []any, with *check.RelationInfo, withRef any, whys []string, preds map[string]*ast.Pred, names *naming.Names) bool {
	if len(members) != len(e.members) || len(preds) != len(e.preds) || len(joins) != len(e.joins) || !slices.Equal(whys, e.whys) {
		return false
	}
	if !relationEqual(with, e.with) || withRef != e.withRef {
		return false
	}
	for i, m := range members {
		if m != e.members[i] || models[i] != e.models[i] {
			return false
		}
	}
	for i, j := range joins {
		if !relationEqual(j, e.joins[i]) || joined[i] != e.joined[i] {
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

func (c *checker) selectCheckMemo(sel *ast.Select, info *check.Info, minted *naming.Names, memo *selectsMemo) *SelectInfo {
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
	joins := make([]joinTarget, len(sel.Joins))
	rels := make([]*check.RelationInfo, len(sel.Joins))
	joined := make([]any, len(sel.Joins))
	for i, id := range sel.Joins {
		if info.Table(sel.Target.Name()) == nil {
			continue
		}
		rel, named := info.Relation(members[0].Key, id.Name())
		if rel == nil {
			joins[i].why = relationAmbiguity(members[0].Key, id.Name(), named)
			continue
		}
		model, fields, err := c.pkg.plan.ModelFields(rel.Table.Key)
		joins[i] = joinTarget{rel: rel, model: memberModel{model: model, fields: fields, err: err}}
		rels[i], joined[i] = rel, c.pkg.plan.ModelRef(rel.Table.Key)
	}
	var with joinTarget
	var withRel *check.RelationInfo
	var withRef any
	if sel.With != nil && info.Table(sel.Target.Name()) != nil {
		rel, named := info.Relation(members[0].Key, sel.With.Name())
		if rel == nil {
			with.why = relationAmbiguity(members[0].Key, sel.With.Name(), named)
		} else {
			model, fields, err := c.pkg.plan.ModelFields(rel.Table.Key)
			with = joinTarget{rel: rel, model: memberModel{model: model, fields: fields, err: err}}
			withRel, withRef = rel, c.pkg.plan.ModelRef(rel.Table.Key)
		}
	}
	var where []ast.PredExpr
	if sel.Where != nil {
		where = append(where, sel.Where)
	}
	preds := predsIn(where, c.pkg.Preds)
	var relationNames []string
	if len(sel.Joins) > 0 || sel.With != nil {
		relationNames = info.RelationNames(members[0].Key)
	}
	whys := make([]string, 0, len(joins)+2)
	for _, j := range joins {
		whys = append(whys, j.why)
	}
	whys = append(whys, with.why, strings.Join(relationNames, ","))
	if memo != nil {
		if e := memo.prev[sel]; e != nil && e.holds(members, refs, rels, joined, withRel, withRef, whys, preds, minted) {
			for _, a := range e.adds {
				minted.Add(a.name, a.desc)
			}
			c.diags = append(c.diags, e.diags...)
			memo.next[sel] = e
			memo.Hits++
			return e.si
		}
	}
	si, lookups, adds, diags := selectCheck(sel, members, models, joins, with, relationNames, preds, minted)
	for _, a := range adds {
		minted.Add(a.name, a.desc)
	}
	c.diags = append(c.diags, diags...)
	if memo != nil {
		memo.next[sel] = &selectEntry{members: members, models: refs, joins: rels, joined: joined, with: withRel, withRef: withRef, whys: whys, preds: preds, lookups: lookups, adds: adds, si: si, diags: diags}
		memo.Misses++
	}
	return si
}

// selectTarget resolves a select's target by the one rule of the
// tables-and-groups namespace (spec §groups): a table, else a group.
func (c *checker) selectTarget(sel *ast.Select, info *check.Info) ([]*check.TableInfo, bool) {
	want := sel.Target.Name()
	var members []*check.TableInfo
	if ti := info.Table(want); ti != nil {
		members = []*check.TableInfo{ti}
	} else if g, isGroup := c.pkg.Groups[want]; isGroup {
		members = g.Members
	} else {
		c.nameMiss(sel.Target, info, "selects-over-groups", "select target")
		return nil, false
	}
	return members, len(members) > 0
}

type memberModel struct {
	model  string
	fields []naming.FieldSig
	err    error
}

type nameScope interface {
	Lookup(name string) (desc string, dup bool)
}

type selectLowering struct {
	si            *SelectInfo
	with          joinTarget
	goFields      map[string]string // the Go fields projected so far, by name
	joins         []joinTarget
	relationNames string // the target table's relations, for the join diagnostics
	sel           *ast.Select
	members       []*check.TableInfo
	models        []memberModel
	preds         map[string]*ast.Pred
	names         nameScope

	lookups []nameLookup
	adds    []nameAdd
	diags   []diag.Diagnostic
	unknown []string
	ordered map[string]map[string]bool // the names order: lists, by qualifier, "" for the target's own and the aliases
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

func (l *selectLowering) unknownAdd(name string) {
	for _, n := range l.unknown {
		if n == name {
			return
		}
	}
	l.unknown = append(l.unknown, name)
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
			name := p.modelOf(m) + si.MethodSuffix
			if _, taken := p.selectByMethod[name]; !taken {
				p.selectByMethod[name] = selectMember{si, m}
			}
		}
	}
}

func (p *Package) modelOf(t *check.TableInfo) string {
	if p.plan != nil {
		if model, _, err := p.plan.CRUDMethods(t.Key); err == nil {
			return model
		}
	}
	return modelOrBase(t)
}

func modelOrBase(t *check.TableInfo) string {
	if m, err := naming.ModelName(t.Decl); err == nil {
		return m
	}
	return t.Decl.Name.Name()
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
			c.errorf(pos, "groups", "table %q is already a member (§groups)", ti.Decl.Name.Name())
			return
		}
		index[ti.Key] = len(g.Members)
		g.Members = append(g.Members, ti)
	}
	remove := func(pos token.Position, ti *check.TableInfo) {
		i, present := index[ti.Key]
		if !present || i < 0 {
			c.errorf(pos, "groups", "table %q is not a member; nothing to remove (§groups)", ti.Decl.Name.Name())
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
			if ti := info.Table(want); ti != nil {
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
			c.nameMiss(id, info, "groups", "group term")
		}
	}
	if len(g.Members) == 0 && !d.Incomplete {
		c.errorf(d.Name.Pos(), "groups", "group %q has no members (§groups)", name)
	}
	c.pkg.Groups[name] = g
	return g
}

func (c *checker) nameMiss(id *ast.Ident, info *check.Info, section, what string) {
	want := id.Name()
	for _, ti := range info.Tables {
		if strings.EqualFold(ti.Decl.Name.Name(), want) {
			c.errorf(id.Pos(), section, "unknown %s %q; did you mean %q? names are case-sensitive (§groups)", what, want, ti.Decl.Name.Name())
			return
		}
	}
	for _, d := range c.pkg.merged.Decls {
		if g, ok := d.(*ast.Group); ok && strings.EqualFold(g.Name.Name(), want) {
			c.errorf(id.Pos(), section, "unknown %s %q; did you mean the group %q? names are case-sensitive (§groups)", what, want, g.Name.Name())
			return
		}
	}
	for _, ti := range info.Tables {
		if m, err := naming.ModelName(ti.Decl); err == nil && m == want {
			c.errorf(id.Pos(), section, "unknown %s %q; %q is the model of table %q: name the table (§groups)", what, want, want, ti.Decl.Name.Name())
			return
		}
	}
	c.errorf(id.Pos(), section, "unknown %s %q: no such table or group in package %q (§groups)", what, want, c.pkg.Path)
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
	nullable  bool
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

func selectCheck(sel *ast.Select, members []*check.TableInfo, models []memberModel, joins []joinTarget, with joinTarget, relationNames []string, preds map[string]*ast.Pred, names nameScope) (*SelectInfo, []nameLookup, []nameAdd, []diag.Diagnostic) {
	l := &selectLowering{sel: sel, members: members, models: models, joins: joins, with: with, relationNames: relationList(relationNames), preds: preds, names: names}
	si := l.check()
	return si, l.lookups, l.adds, l.diags
}

func (l *selectLowering) check() *SelectInfo {
	sel := l.sel
	si := &SelectInfo{Decl: sel, Members: l.members}
	l.si = si

	suffix, err := naming.GoName(sel.Name.Name())
	if err != nil {
		l.errorf(sel.Name.Pos(), "select name %q does not map to a Go identifier: %v (§selects-over-groups)", sel.Name.Name(), err)
		return nil
	}
	si.MethodSuffix = suffix
	if crudMethodSuffixes[suffix] {
		l.errorf(sel.Name.Pos(), "select name %q collides with the generated CRUD surface (§selects-over-groups)", suffix)
		return nil
	}

	if !l.joinsCheck(si) {
		return nil
	}
	env := &selectEnv{
		l:       l,
		sel:     sel,
		members: si.Members,
		joins:   si.Joins,
		cols:    map[string]*colBinding{},
		params:  map[string]*SelectParam{},
		aliases: map[string]*ast.ProjItem{},
	}
	if len(si.Joins) > 0 {
		env.base = si.Members[0].Key
	}
	if sel.Projected() && !l.projectionCheck(si, env) {
		return nil
	}
	if !l.withCheck(si) {
		return nil
	}
	l.keptUnknown()

	var where rt.GenPred
	if sel.Where != nil {
		where = env.exprLower(sel.Where, map[string]bool{})
	}
	si.droppedParams = env.dropped

	order := l.selectSettings(env)
	if env.failed {
		return nil
	}
	for _, it := range si.Items {
		if it.Aggregate() && si.Group == nil {
			si.Lookup = true
		}
	}
	if si.Lookup {
		one := false
		if len(order) > 0 {
			l.errorf(sel.Settings.Get("order").Pos(), "select %q aggregates without group:, so it yields one row, which order: cannot sort (§selects-over-groups)", sel.Name.Name())
			one = true
		}
		if si.Limit != "" {
			l.errorf(sel.Settings.Get("limit").Pos(), "select %q aggregates without group:, so it yields one row, which limit: has nothing to cap (§selects-over-groups)", sel.Name.Name())
			one = true
		}
		if one {
			return nil
		}
	}
	if si.Limit != "" {
		if sel.Settings.Get("order") == nil {
			l.errorf(sel.Settings.Get("limit").Pos(), "select %q takes limit: without order:, so the rows it keeps would be arbitrary; order them, [order: (...), limit: ...] (§selects-over-groups)", sel.Name.Name())
		} else {
			l.limitOrderTotal()
		}
	}
	if len(l.unknown) > 0 {
		si.Unresolved = true
		si.UnknownCols = l.unknown
		sort.Strings(si.UnknownCols)
		si.Params = env.paramList
		return si
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

// joinsCheck resolves the select's joins (§selects-over-groups):
// relations of a table target, each once, under an explicit projection.
func (l *selectLowering) joinsCheck(si *SelectInfo) bool {
	sel := l.sel
	if len(sel.Joins) == 0 {
		return true
	}
	if len(l.members) != 1 || l.members[0].Key != sel.Target.Name() {
		l.errorf(sel.Joins[0].Pos(), "join needs a table: %q is a group, and a group has no relations (§selects-over-groups)", sel.Target.Name())
		return false
	}
	target := l.members[0]
	seen := map[string]bool{}
	for i, id := range sel.Joins {
		name := id.Name()
		jt := l.joins[i]
		if jt.rel == nil {
			if jt.why != "" {
				l.errorf(id.Pos(), "%s (§relationships-ref, §selects-over-groups)", jt.why)
				return false
			}
			l.errorf(id.Pos(), "table %q has no relation %q to join; its relations: %s (§selects-over-groups, §relationships-ref)", target.Key, name, l.relationNames)
			return false
		}
		if seen[name] {
			l.errorf(id.Pos(), "relation %q is joined twice (§selects-over-groups)", name)
			return false
		}
		seen[name] = true
		if jt.model.err != nil {
			l.errorf(id.Pos(), "select %q: %v (§selects-over-groups)", sel.Name.Name(), jt.model.err)
			return false
		}
		si.Joins = append(si.Joins, JoinInfo{Name: name, Table: jt.rel.Table, Left: jt.rel.Optional, On: jt.rel.On})
	}
	if !sel.Projected() || sel.Star {
		l.errorf(sel.Name.Pos(), "select %q joins, so it lists the columns it projects, its own and the joined relations' as relation.column (§selects-over-groups)", sel.Name.Name())
		return false
	}
	return true
}

// relationAmbiguity says why a relation name cannot be joined (spec
// §relationships-ref): "" when the table simply has none.
func relationAmbiguity(table, name string, named []*check.RelationInfo) string {
	if len(named) == 0 {
		return ""
	}
	parts := make([]string, len(named))
	for i, r := range named {
		fix := "reverse"
		if r.Forward {
			fix = "as"
		}
		parts[i] = fmt.Sprintf("the %s relation to %q (at %s; name it with [%s: name] on a ref element)", r.Side(), r.Table.Key, r.Pos, fix)
	}
	if strings.EqualFold(name, table) {
		return fmt.Sprintf("relation %q of table %q takes the table's own name, which the join's alias needs, read as SQLite reads it, without regard to case: it is %s", name, table, strings.Join(parts, " and "))
	}
	return fmt.Sprintf("relation %q of table %q is ambiguous, read as SQLite reads an alias, without regard to case: it is %s", name, table, strings.Join(parts, " and "))
}

// withCheck resolves the select's with (spec §selects-over-groups): a
// reverse relation of a table target over one key, which the parent row
// carries, under no aggregate; it mints the row type, shared with any
// select that nests the same relation under the same parent row type.
func (l *selectLowering) withCheck(si *SelectInfo) bool {
	sel := l.sel
	if sel.With == nil {
		return true
	}
	id := sel.With
	name := id.Name()
	if len(l.members) != 1 || l.members[0].Key != sel.Target.Name() {
		l.errorf(id.Pos(), "with needs a table: %q is a group, and a group has no relations (§selects-over-groups)", sel.Target.Name())
		return false
	}
	target := l.members[0]
	wt := l.with
	if wt.rel == nil {
		if wt.why != "" {
			l.errorf(id.Pos(), "%s (§relationships-ref, §selects-over-groups)", wt.why)
			return false
		}
		l.errorf(id.Pos(), "table %q has no relation %q to nest; its relations: %s (§selects-over-groups, §relationships-ref)", target.Key, name, l.relationNames)
		return false
	}
	if wt.rel.Forward {
		l.errorf(id.Pos(), "with takes a reverse relation, the rows that refer to each parent; %q leads to the one row %s refers to, which join reaches (§selects-over-groups)", name, target.Key)
		return false
	}
	if len(wt.rel.On) != 1 {
		l.errorf(id.Pos(), "with takes a relation over one key column; %q matches %d (§selects-over-groups)", name, len(wt.rel.On))
		return false
	}
	if wt.model.err != nil {
		l.errorf(id.Pos(), "select %q: %v (§selects-over-groups)", sel.Name.Name(), wt.model.err)
		return false
	}
	for _, it := range si.Items {
		if it.Aggregate() {
			l.errorf(id.Pos(), "with nests rows under each parent row, and an aggregate yields none to nest under (§selects-over-groups)")
			return false
		}
	}
	parentKey, childKey := wt.rel.On[0][0], wt.rel.On[0][1]
	if cd := target.Column(parentKey); cd != nil {
		switch {
		case cd.Nullable:
			l.errorf(id.Pos(), "with %s matches the parent's %q, which carries null; a NULL key matches no child, so the key is a column that is never NULL (§selects-over-groups)", name, parentKey)
			return false
		case cd.Type.Known() && cd.Type.Storage == "[]byte":
			l.errorf(id.Pos(), "with %s matches the parent's %q (%s), whose keys cannot travel as one JSON list, as a list parameter cannot (§selects-over-groups, §predicates)", name, parentKey, cd.Col.Type.String())
			return false
		}
	}
	excluded := map[string]bool{}
	for _, cr := range sel.Cols {
		excluded[cr.String()] = true
	}
	if sel.Projected() {
		carried := sel.Star && !excluded[parentKey]
		for _, item := range sel.Items {
			if !item.Derived() && item.Col.Qual == nil && item.Col.Name.Name() == parentKey {
				carried = true
			}
		}
		if !carried {
			l.errorf(id.Pos(), "with %s matches the parent's %q, which the projection must carry as a bare column (§selects-over-groups)", name, parentKey)
			return false
		}
	}
	field, err := naming.GoName(name)
	if err != nil {
		l.errorf(id.Pos(), "relation %q does not map to a Go name: %v (§relationships-ref)", name, err)
		return false
	}
	collides := l.goFields[field] != ""
	if !sel.Projected() || sel.Star {
		for _, f := range l.models[0].fields {
			if f.Name == field && !excluded[f.Col] {
				collides = true
			}
		}
	}
	if collides {
		l.errorf(id.Pos(), "the parent row's field %s and the children's field, named after %q, collide (§selects-over-groups)", field, name)
		return false
	}
	parentRow := l.models[0].model
	switch {
	case si.Shared != "":
		parentRow = si.Shared
	case sel.Star:
		parentRow = l.models[0].model + si.MethodSuffix + "Row"
	}
	rowType := parentRow + "With" + field
	desc := fmt.Sprintf("the nested row type of %s with %s", parentRow, name)
	if prev, dup := l.lookup(rowType); dup && prev != desc {
		l.errorf(sel.Name.Pos(), "select %q mints the row type %s, which collides with %s (§selects-over-groups)", sel.Name.Name(), rowType, prev)
		return false
	} else if !dup {
		l.add(rowType, desc)
	}
	si.With = &WithInfo{Name: name, Table: wt.rel.Table, ParentKey: parentKey, ChildKey: childKey, Field: field, RowType: rowType}
	return true
}

func relationList(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func (l *selectLowering) join(name string) *joinTarget {
	for i, id := range l.sel.Joins {
		if id.Name() == name && l.joins[i].rel != nil {
			return &l.joins[i]
		}
	}
	return nil
}

func (l *selectLowering) projectionCheck(si *SelectInfo, env *selectEnv) bool {
	sel := l.sel
	type memberFields struct {
		ti     *check.TableInfo
		model  string
		fields []naming.FieldSig
		byCol  map[string]naming.FieldSig
	}
	members := make([]memberFields, 0, len(si.Members))
	for i, m := range si.Members {
		model, fields, err := l.models[i].model, l.models[i].fields, l.models[i].err
		if err != nil {
			l.errorf(sel.Name.Pos(), "select %q: %v (§selects-over-groups)", sel.Name.Name(), err)
			return false
		}
		byCol := make(map[string]naming.FieldSig, len(fields))
		for _, f := range fields {
			byCol[f.Col] = f
		}
		members = append(members, memberFields{m, model, fields, byCol})
	}

	ok := true
	seen := map[string]bool{}
	l.goFields = map[string]string{}
	for _, item := range sel.Items {
		if item.Derived() {
			name := item.As.Name()
			if seen[name] {
				l.errorf(item.As.Pos(), "%q appears twice in the projection (§selects-over-groups)", name)
				ok = false
				continue
			}
			seen[name] = true
			it, good := l.deriveItem(si, env, item)
			if !good {
				ok = false
				continue
			}
			si.Items = append(si.Items, it)
			continue
		}
		cr := item.Col
		name := cr.String()
		if seen[name] {
			l.errorf(cr.Pos(), "column %q appears twice in the projection (§selects-over-groups)", name)
			ok = false
			continue
		}
		seen[name] = true
		si.Items = append(si.Items, naming.SelectItem{Col: name})
		if cr.Qual != nil {
			jt := l.join(cr.Qual.Name())
			switch {
			case len(si.Joins) == 0:
				l.errorf(cr.Pos(), "%s names a column of a relation, and the select joins none (§selects-over-groups)", name)
				ok = false
			case jt == nil:
				l.errorf(cr.Qual.Pos(), "%q is no joined relation of this select; joined: %s (§selects-over-groups)", cr.Qual.Name(), joinList(sel))
				ok = false
			default:
				f, has := fieldByCol(jt.model.fields, cr.Name.Name())
				if !has {
					l.errorf(cr.Name.Pos(), "table %q, joined as %q, has no column %q (§selects-over-groups)", jt.rel.Table.Key, cr.Qual.Name(), cr.Name.Name())
					ok = false
					continue
				}
				if cd := jt.rel.Table.Column(cr.Name.Name()); cd != nil && !cd.Type.Known() {
					l.unknownAdd(name)
					continue
				}
				prefix, err := naming.GoName(cr.Qual.Name())
				if err != nil {
					l.errorf(cr.Qual.Pos(), "relation %q does not map to a Go name: %v (§relationships-ref)", cr.Qual.Name(), err)
					ok = false
					continue
				}
				if !l.fieldOnce(cr.Pos(), name, prefix+f.Name) {
					ok = false
				}
			}
			continue
		}
		id := cr.Name
		if f, has := members[0].byCol[id.Name()]; has && !l.fieldOnce(cr.Pos(), name, f.Name) {
			ok = false
			continue
		}

		var missing []string
		types := map[string][]string{}
		for _, mf := range members {
			f, has := mf.byCol[name]
			if !has {
				missing = append(missing, mf.ti.Decl.Name.Name())
				continue
			}
			if cd := mf.ti.Column(name); cd != nil && !cd.Type.Known() {
				l.unknownAdd(name)
				continue
			}
			types[f.Type] = append(types[f.Type], mf.ti.Decl.Name.Name())
		}
		if len(missing) > 0 {
			l.errorf(id.Pos(), "column %q is missing from %s — every member of %q must have it (§selects-over-groups)",
				name, tableList(missing), sel.Target.Name())
			ok = false
			continue
		}
		if len(types) > 1 {
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
	for _, cr := range sel.Cols {
		name := cr.String()
		if seen[name] {
			l.errorf(cr.Pos(), "column %q appears twice in the projection (§selects-over-groups)", name)
			ok = false
			continue
		}
		seen[name] = true
		var missing []string
		for _, mf := range members {
			if _, has := mf.byCol[name]; !has {
				missing = append(missing, mf.ti.Decl.Name.Name())
			}
		}
		if len(missing) > 0 {
			l.errorf(cr.Pos(), "column %q is missing from %s — every member of %q must have it (§selects-over-groups)",
				name, tableList(missing), sel.Target.Name())
			ok = false
		}
	}
	if !ok {
		return false
	}

	if !sel.Star {
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
						sel.Name.Name(), si.Shared, mf.ti.Decl.Name.Name(), f.Col)
					return false
				}
			}
		}
		l.add(si.Shared, fmt.Sprintf("select %q's shared row type", sel.Name.Name()))
		return true
	}

	cols := make([]string, len(sel.Cols))
	for i, cr := range sel.Cols {
		cols[i] = cr.String()
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
					sel.Name.Name(), mf.model, si.MethodSuffix, mf.ti.Decl.Name.Name(), f.Col)
				return false
			}
		}
		if kept == 0 {
			l.errorf(sel.Name.Pos(), "select %q excludes every column of %q — nothing is left to project (§selects-over-groups)",
				sel.Name.Name(), mf.ti.Decl.Name.Name())
			return false
		}
		name := mf.model + si.MethodSuffix
		if prev, dup := l.lookup(name); dup {
			l.errorf(sel.Name.Pos(), "select %q mints the row type %s, which collides with %s (§selects-over-groups)",
				sel.Name.Name(), name, prev)
			return false
		}
		l.add(name, fmt.Sprintf("select %q's row type for table %q", sel.Name.Name(), mf.ti.Decl.Name.Name()))
	}
	return true
}

// fieldOnce records a projected item's Go field (spec
// §selects-over-groups): two items with one field are an error.
func (l *selectLowering) fieldOnce(pos token.Position, written, goField string) bool {
	if prev, dup := l.goFields[goField]; dup {
		l.errorf(pos, "columns %s and %s both map to the Go field %s (§selects-over-groups)", prev, written, goField)
		return false
	}
	l.goFields[goField] = written
	return true
}

// deriveItem lowers an aggregate or an arithmetic item (spec
// §selects-over-groups): its alias, its Go type and the renderer's item.
func (l *selectLowering) deriveItem(si *SelectInfo, env *selectEnv, item *ast.ProjItem) (naming.SelectItem, bool) {
	alias := item.As.Name()
	goField, err := naming.GoName(alias)
	if err != nil || alias[0] < 'a' || alias[0] > 'z' {
		l.errorf(item.As.Pos(), "alias %q is not a lower-case identifier that maps to a Go name (§selects-over-groups)", alias)
		return naming.SelectItem{}, false
	}
	for _, m := range l.members {
		if m.Column(alias) != nil {
			l.errorf(item.As.Pos(), "alias %q names a column of %q; an alias names no column of the target or of a joined table, which SQLite would group by instead (§selects-over-groups)", alias, m.Key)
			return naming.SelectItem{}, false
		}
	}
	for _, j := range si.Joins {
		if j.Table.Column(alias) != nil {
			l.errorf(item.As.Pos(), "alias %q names a column of %q, joined as %q; an alias names no column of the target or of a joined table, which SQLite would group by instead (§selects-over-groups)", alias, j.Table.Key, j.Name)
			return naming.SelectItem{}, false
		}
	}
	if !l.fieldOnce(item.As.Pos(), alias, goField) {
		return naming.SelectItem{}, false
	}
	env.aliases[alias] = item
	it := naming.SelectItem{Alias: alias}
	nullable := false
	if item.Agg != nil {
		it.Agg = item.Agg.Name()
		var b *colBinding
		if item.Col != nil {
			var unknown bool
			b, unknown = env.columnBind(item.Col)
			if b == nil {
				return it, unknown
			}
			it.Col = item.Col.String()
			nullable = b.nullable
		}
		switch it.Agg {
		case "count":
			it.GoType = "int"
			nullable = false
		case "sum", "avg":
			if b.class != classNumeric {
				l.errorf(item.Col.Pos(), "%s takes a numeric column; %q is %s (§selects-over-groups)", it.Agg, b.name, b.goType)
				return it, false
			}
			it.GoType = b.goType
			if it.Agg == "avg" {
				it.GoType = "float64"
			}
		default:
			if b.class != classNumeric && b.class != classTime && b.class != classText {
				l.errorf(item.Col.Pos(), "%s takes a numeric, date/time or text column; %q is %s (§selects-over-groups)", it.Agg, b.name, b.goType)
				return it, false
			}
			it.GoType = b.goType
		}
		if it.Agg != "count" && !si.hasGroup() {
			nullable = true
		}
	} else if cr, renamed := item.Expr.(*ast.ColRef); renamed {
		b, unknown := env.columnBind(cr)
		if b == nil {
			return it, unknown
		}
		it.Col, it.Expr, it.GoType, nullable = cr.String(), rt.ExprCol(b.name), b.goType, b.nullable
	} else {
		var cols []*colBinding
		unknown := false
		for _, cr := range arithCols(item.Expr, nil) {
			b, u := env.columnBind(cr)
			if u {
				unknown = true
			}
			if b != nil {
				cols = append(cols, b)
			}
		}
		if unknown {
			return it, true
		}
		if len(cols) == 0 {
			l.errorf(item.Expr.Pos(), "an expression names a column, which types it (§selects-over-groups)")
			return it, false
		}
		typ := cols[0]
		for _, b := range cols {
			if b.class != classNumeric {
				l.errorf(item.Expr.Pos(), "arithmetic takes numeric columns; %q is %s (§selects-over-groups)", b.name, b.goType)
				return it, false
			}
			if b.goType != typ.goType {
				l.errorf(item.Expr.Pos(), "an expression has one type: %q is %s and %q is %s (§selects-over-groups)", typ.name, typ.goType, b.name, b.goType)
				return it, false
			}
			if b.nullable {
				nullable = true
			}
		}
		expr, good := env.arithLower(item.Expr, typ)
		if !good {
			return it, false
		}
		it.Expr, it.GoType = expr, typ.goType
	}
	if nullable {
		it.GoType = "rt.Null[" + it.GoType + "]"
	}
	return it, true
}

func (si *SelectInfo) hasGroup() bool {
	if si.Decl.Settings == nil {
		return false
	}
	return si.Decl.Settings.Get("group") != nil
}

func arithCols(x ast.ArithExpr, acc []*ast.ColRef) []*ast.ColRef {
	switch x := x.(type) {
	case *ast.ColRef:
		return append(acc, x)
	case *ast.ArithBinary:
		return arithCols(x.Y, arithCols(x.X, acc))
	case *ast.ArithParen:
		return arithCols(x.X, acc)
	}
	return acc
}

// arithLower builds the renderer's expression (spec
// §selects-over-groups): columns by their bound names, literals
// representable in the expression's storage type and parameters of its
// Go type as slots.
func (e *selectEnv) arithLower(x ast.ArithExpr, typ *colBinding) (rt.Expr, bool) {
	switch x := x.(type) {
	case *ast.ColRef:
		b, _ := e.columnBind(x)
		if b == nil {
			return rt.Expr{}, false
		}
		return rt.ExprCol(b.name), true
	case *ast.Lit:
		if x.Tok.Kind != token.NUMBER {
			e.errorf(x.Pos(), "arithmetic takes numbers; found %s (§selects-over-groups)", x.String())
			return rt.Expr{}, false
		}
		slot, err := numberGo(x.Tok.Val, typ.storageGo)
		if err != nil {
			e.errorf(x.Pos(), "literal %s does not fit %s, the type of the expression (§selects-over-groups)", x.Tok.Val, typ.storageGo)
			return rt.Expr{}, false
		}
		return rt.ExprVal(slot), true
	case *ast.Param:
		return rt.ExprVal(e.paramBind(x, typ.goType)), true
	case *ast.ArithParen:
		return e.arithLower(x.X, typ)
	case *ast.ArithBinary:
		l, ok := e.arithLower(x.X, typ)
		if !ok {
			return rt.Expr{}, false
		}
		r, ok := e.arithLower(x.Y, typ)
		if !ok {
			return rt.Expr{}, false
		}
		expr, err := rt.ExprBin(x.Op, l, r)
		if err != nil {
			e.errorf(x.Pos(), "%v (§selects-over-groups)", err)
			return rt.Expr{}, false
		}
		return expr, true
	}
	return rt.Expr{}, false
}

func (l *selectLowering) keptUnknown() {
	sel := l.sel
	if sel.Projected() && !sel.Star {
		return
	}
	excluded := map[string]bool{}
	for _, cr := range sel.Cols {
		excluded[cr.String()] = true
	}
	for _, m := range l.members {
		for _, cd := range m.Columns {
			if name := cd.Col.Name.Name(); !cd.Type.Known() && !excluded[name] {
				l.unknownAdd(name)
			}
		}
	}
}

func (l *selectLowering) selectSettings(env *selectEnv) []rt.GenOrder {
	sel := l.sel
	if sel.Settings == nil {
		return nil
	}
	var terms []rt.GenOrder
	var limitParam *ast.Param
	repeated, dupDiags := settingsRepeated(sel.Settings, "order", "group", "limit")
	l.diags = append(l.diags, dupDiags...)
	for _, s := range sel.Settings.Settings {
		if repeated[s] {
			continue
		}
		switch s.Name {
		case "limit":
			limitParam = l.limitSetting(s)
		case "group":
			l.groupSetting(env, s)
		case "order":
			list, ok := s.Value.(*ast.IdentList)
			if !ok {
				l.errorf(s.Pos(), "order: takes a parenthesized column list (§selects-over-groups)")
				continue
			}
			seenOrder := map[string]bool{}
			for i, id := range list.Names {
				cr := &ast.ColRef{Qual: list.Quals[i], Name: id}
				var b *colBinding
				var unknown bool
				if item, isAlias := env.aliases[id.Name()]; isAlias && cr.Qual == nil {
					b = env.aliasBinding(item)
				} else {
					b, unknown = env.columnBind(cr)
				}
				if b == nil && !unknown {
					continue
				}
				name := cr.String()
				if seenOrder[name] {
					l.errorf(id.Pos(), "column %q appears twice in order: (§selects-over-groups)", name)
					continue
				}
				seenOrder[name] = true
				l.orderedNote(cr)
				if b != nil && b.class == classEqOnly {
					l.errorf(id.Pos(), "column %q has declared type %s; a declared type cannot be ordered (§selects-over-groups)", b.name, b.goType)
					continue
				}
				if b != nil && b.class != classNumeric && b.class != classTime && b.class != classText {
					l.errorf(id.Pos(), "column %q (%s) is not orderable (§selects-over-groups)", b.name, b.goType)
					continue
				}
				mod := list.Mods[i]
				if mod == nil {
					l.errorf(id.Pos(), "order: every column states its direction; write %q or %q (§selects-over-groups)", id.Name()+" asc", id.Name()+" desc")
					continue
				}
				desc := false
				switch mod.Name() {
				case "asc":
				case "desc":
					desc = true
				default:
					l.errorf(mod.Pos(), "order direction is asc or desc, not %q (§selects-over-groups)", mod.Name())
					continue
				}
				if b != nil {
					terms = append(terms, rt.OrderTerm(b.name, desc))
				}
			}
		default:
			l.errorf(s.Pos(), "unknown select setting %q; selects take [order: (...)], [group: (...)] and [limit: ...] (§selects-over-groups)", s.Name)
		}
	}
	if limitParam != nil {
		name := limitParam.Name.Name()
		if _, used := env.params[name]; used || env.dropped[name] {
			l.errorf(limitParam.Pos(), "parameter :%s is the limit and a parameter of the select besides; a limit takes a parameter of its own (§selects-over-groups)", name)
		} else {
			env.paramBind(limitParam, "int")
			l.si.Limit, l.si.LimitParam = name, name
		}
	}
	return terms
}

// orderedNote records a name order: lists under its qualifier, the
// relation joined or "" for the target's own columns and the aliases.
func (l *selectLowering) orderedNote(cr *ast.ColRef) {
	qual := ""
	if cr.Qual != nil {
		qual = cr.Qual.Name()
	}
	if l.ordered == nil {
		l.ordered = map[string]map[string]bool{}
	}
	if l.ordered[qual] == nil {
		l.ordered[qual] = map[string]bool{}
	}
	l.ordered[qual][cr.Name.Name()] = true
}

// limitOrderTotal reports an order: that leaves ties under a limit:
// (spec §selects-over-groups). Without group: a row is a row of each
// member and of each joined relation of many rows, so the order names a
// key of each; with group: a row is a group, so the order names every
// group: item, or a key of the target that group: holds when no item
// reaches a relation of many rows.
func (l *selectLowering) limitOrderTotal() {
	sel, at := l.sel, l.sel.Settings.Get("limit").Pos()
	var many []int
	for i := range sel.Joins {
		if rel := l.joins[i].rel; rel != nil && !rel.Forward {
			many = append(many, i)
		}
	}
	own := l.ordered[""]
	if l.si.Group != nil {
		if l.groupKeyed(many) {
			return
		}
		for _, item := range sel.Items {
			if item.Agg == nil && !own[item.Name()] {
				l.errorf(at, "select %q takes limit: over an order that leaves ties: order: does not name the group: item %q, so two groups can tie and the ones kept at the cut would be arbitrary; name every group: item in order:, or a key of the target that group: holds (§selects-over-groups)", sel.Name.Name(), item.Name())
				return
			}
		}
		return
	}
	for _, ti := range l.members {
		if !ti.ColumnsKeyed(own) {
			l.errorf(at, "select %q takes limit: over an order that leaves ties: order: names no key of %s, so two rows can tie and the ones kept at the cut would be arbitrary; %s (§selects-over-groups)", sel.Name.Name(), ti.Decl.Name.Name(), keyHint(ti, ""))
			return
		}
	}
	for _, i := range many {
		name, ti := sel.Joins[i].Name(), l.joins[i].rel.Table
		if !ti.ColumnsKeyed(l.ordered[name]) {
			l.errorf(at, "select %q takes limit: over an order that leaves ties: the join of %q repeats a row of %s for each of its rows, and order: names no key of %s, so two rows can tie and the ones kept at the cut would be arbitrary; %s (§selects-over-groups)", sel.Name.Name(), name, l.members[0].Decl.Name.Name(), ti.Decl.Name.Name(), keyHint(ti, name+"."))
			return
		}
	}
}

// groupKeyed reports whether a grouped select's order names a key of
// every member that group: holds as plain columns, while no group: item
// reaches a joined relation of many rows (spec §selects-over-groups): one
// group is then one row of the target.
func (l *selectLowering) groupKeyed(many []int) bool {
	sel := l.sel
	reverse := map[string]bool{}
	for _, i := range many {
		reverse[sel.Joins[i].Name()] = true
	}
	held := map[string]bool{}
	for _, item := range sel.Items {
		if item.Agg != nil {
			continue
		}
		if item.Expr != nil {
			for _, cr := range arithCols(item.Expr, nil) {
				if cr.Qual != nil && reverse[cr.Qual.Name()] {
					return false
				}
			}
			continue
		}
		if item.Col.Qual == nil && l.ordered[""][item.Col.Name.Name()] {
			held[item.Col.Name.Name()] = true
		}
	}
	for _, ti := range l.members {
		if !ti.ColumnsKeyed(held) {
			return false
		}
	}
	return true
}

// keyHint suggests the primary key to end an order with, qualified by
// prefix for a joined relation (spec §selects-over-groups).
func keyHint(ti *check.TableInfo, prefix string) string {
	pk := ti.PK()
	if len(pk) == 0 {
		return "a table without a primary key or a unique column that is never NULL takes no limit:"
	}
	names := make([]string, len(pk))
	for i, cd := range pk {
		names[i] = prefix + cd.Col.Name.Name() + " asc"
	}
	return "end it in the primary key, (..., " + strings.Join(names, ", ") + ")"
}

// limitSetting reads a select's limit: (spec §selects-over-groups): a
// positive integer literal, which it records in canonical form, or a
// parameter, which it returns for the caller to bind after every other.
func (l *selectLowering) limitSetting(s *ast.Setting) *ast.Param {
	switch v := s.Value.(type) {
	case *ast.Param:
		return v
	case *ast.NegNumber:
		l.errorf(v.Pos(), "limit: keeps at least 1 row, not -%s (§selects-over-groups)", v.Num.Tok.Val)
		return nil
	case *ast.BasicLit:
		if v.Tok.Kind == token.NUMBER && strings.Trim(v.Tok.Val, "0123456789") == "" {
			n, err := strconv.ParseInt(v.Tok.Val, 10, 64)
			switch {
			case err != nil:
				l.errorf(v.Pos(), "limit: %s does not fit int64 (§selects-over-groups)", v.Tok.Val)
			case n < 1:
				l.errorf(v.Pos(), "limit: keeps at least 1 row, not %s (§selects-over-groups)", v.Tok.Val)
			default:
				l.si.Limit = strconv.FormatInt(n, 10)
			}
			return nil
		}
	}
	l.errorf(s.Pos(), "limit: takes a positive integer or a parameter, :name (§selects-over-groups)")
	return nil
}

// aliasBinding is the binding an order: term takes for a derived item
// (spec §selects-over-groups): the alias, typed as the item is.
func (e *selectEnv) aliasBinding(item *ast.ProjItem) *colBinding {
	alias := item.As.Name()
	for _, it := range e.l.si.Items {
		if it.Alias != alias {
			continue
		}
		goType, _ := strings.CutPrefix(it.GoType, "rt.Null[")
		goType = strings.TrimSuffix(goType, "]")
		class := classForbidden
		switch goType {
		case "int", "float64":
			class = classNumeric
		case "string":
			class = classText
		case "time.Time":
			class = classTime
		default:
			if it.Agg == "min" || it.Agg == "max" {
				class = classEqOnly
			}
		}
		return &colBinding{name: alias, goType: goType, storageGo: goType, class: class, storage: class}
	}
	return nil
}

// groupSetting lowers group: (spec §selects-over-groups): every
// non-aggregate item by its name, nothing else, each once.
func (l *selectLowering) groupSetting(env *selectEnv, s *ast.Setting) {
	si := l.si
	list, ok := s.Value.(*ast.IdentList)
	if !ok {
		l.errorf(s.Pos(), "group: takes a parenthesized list of the projected items (§selects-over-groups)")
		return
	}
	if len(l.sel.Items) == 0 {
		l.errorf(s.Pos(), "group: lists projected items, and the select projects none; list the columns it groups (§selects-over-groups)")
		return
	}
	byName := map[string]int{}
	for i, item := range l.sel.Items {
		byName[item.Name()] = i
	}
	seen := map[string]bool{}
	ok = true
	var names []string
	for i, id := range list.Names {
		if list.Quals[i] != nil {
			l.errorf(id.Pos(), "group: names an item by its name or alias; project %s.%s under an alias to group by it (§selects-over-groups)", list.Quals[i].Name(), id.Name())
			ok = false
			continue
		}
		if list.Mods[i] != nil {
			l.errorf(list.Mods[i].Pos(), "group: entries are item names; %q does not belong after %q (§selects-over-groups)", list.Mods[i].Name(), id.Name())
			ok = false
			continue
		}
		name := id.Name()
		idx, found := byName[name]
		if !found {
			l.errorf(id.Pos(), "group: names %q, which the select does not project (§selects-over-groups)", name)
			ok = false
			continue
		}
		item := l.sel.Items[idx]
		if item.Agg != nil {
			l.errorf(id.Pos(), "group: names the aggregate %s; a group is made of the other items (§selects-over-groups)", item.String())
			ok = false
			continue
		}
		if seen[name] {
			l.errorf(id.Pos(), "group: names %q twice (§selects-over-groups)", name)
			ok = false
			continue
		}
		seen[name] = true
		if item.Derived() {
			names = append(names, name)
			continue
		}
		b, _ := env.columnBind(item.Col)
		if b == nil {
			ok = false
			continue
		}
		names = append(names, b.name)
	}
	for _, item := range l.sel.Items {
		if item.Agg == nil && !seen[item.Name()] {
			l.errorf(s.Pos(), "group: leaves out %s; every item that is not an aggregate is grouped by (§selects-over-groups)", item.String())
			ok = false
		}
	}
	if !ok || len(names) == 0 {
		env.failed = true
		return
	}
	si.Group = names
}

type selectEnv struct {
	l         *selectLowering
	sel       *ast.Select
	members   []*check.TableInfo
	joins     []JoinInfo
	base      string // the qualifier of the target's own columns, its name, once a join is present; "" otherwise
	aliases   map[string]*ast.ProjItem
	cols      map[string]*colBinding
	params    map[string]*SelectParam
	paramList []SelectParam
	failed    bool
	dropped   map[string]bool
}

func (e *selectEnv) errorf(pos token.Position, format string, args ...any) {
	e.failed = true
	e.l.errorf(pos, format, args...)
}

func (e *selectEnv) join(name string) *JoinInfo {
	for i := range e.joins {
		if e.joins[i].Name == name {
			return &e.joins[i]
		}
	}
	return nil
}

func joinList(sel *ast.Select) string {
	names := make([]string, len(sel.Joins))
	for i, id := range sel.Joins {
		names[i] = id.Name()
	}
	return strings.Join(names, ", ")
}

func fieldByCol(fields []naming.FieldSig, col string) (naming.FieldSig, bool) {
	for _, f := range fields {
		if f.Col == col {
			return f, true
		}
	}
	return naming.FieldSig{}, false
}

// columnBind resolves a column reference in the select's environment
// (§predicates): a bare name over every member, a qualified one
// over the joined relation's table; the binding's name is what the
// statement spells, qualified once a join is present.
func (e *selectEnv) columnBind(cr *ast.ColRef) (b *colBinding, unknown bool) {
	written := cr.String()
	if b, done := e.cols[written]; done {
		return b, false
	}
	id := cr.Name
	name := id.Name()
	tables, sqlName := e.members, name
	leftJoined := false
	if e.base != "" {
		sqlName = e.base + "." + name
	}
	if cr.Qual != nil {
		j := e.join(cr.Qual.Name())
		if j == nil {
			if len(e.joins) == 0 {
				e.errorf(cr.Pos(), "%s names a column of a relation, and the select joins none (§selects-over-groups)", written)
			} else {
				e.errorf(cr.Qual.Pos(), "%q is no joined relation of this select; joined: %s (§selects-over-groups)", cr.Qual.Name(), joinList(e.sel))
			}
			return nil, false
		}
		if j.Table.Column(name) == nil {
			e.errorf(id.Pos(), "table %q, joined as %q, has no column %q (§selects-over-groups)", j.Table.Key, j.Name, name)
			return nil, false
		}
		tables, sqlName = []*check.TableInfo{j.Table}, j.Name+"."+name
		leftJoined = j.Left
	}
	var missing []string
	types := map[string][]string{}
	classes := map[typeClass]bool{}
	var class, storage typeClass
	var forbiddenType string
	var storageGo string
	nullable := false
	for _, m := range tables {
		cd := m.Column(name)
		if cd == nil {
			missing = append(missing, m.Decl.Name.Name())
			continue
		}
		if cd.Nullable {
			nullable = true
		}
		declType := cd.Col.Type.String()
		if !cd.Type.Known() {
			unknown = true
			continue
		}
		forbiddenType = declType
		goType := cd.Type.Go
		types[goType] = append(types[goType], m.Decl.Name.Name())
		class = classOfType(cd.Type)
		storage = classOfName(cd.Type.StorageClass())
		storageGo = cd.Type.Storage
		classes[class] = true
	}
	if len(missing) > 0 {
		hint := ""
		if _, isAlias := e.aliases[name]; isAlias && cr.Qual == nil {
			hint = "; " + name + " is an alias, which where does not see, and Volt has no having (§selects-over-groups)"
		}
		e.errorf(id.Pos(), "column %q is missing from %s — every member of %q must have it%s (§selects-over-groups)",
			name, tableList(missing), e.sel.Target.Name(), hint)
		return nil, false
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
		return nil, false
	}
	if len(classes) > 1 {
		if classes[classEqOnly] {
			class = classEqOnly
		}
	}
	if len(types) > 0 && class == classForbidden {
		e.errorf(id.Pos(), "column %q (%s) cannot appear in a predicate (§predicates)", name, forbiddenType)
		return nil, false
	}
	if unknown {
		e.l.unknownAdd(written)
		return nil, true
	}
	var goType string
	for t := range types {
		goType = t
	}
	b = &colBinding{name: sqlName, goType: goType, storageGo: storageGo, class: class, storage: storage, nullable: nullable || leftJoined}
	e.cols[written] = b
	return b, false
}

func (e *selectEnv) paramsDrop(ops ...ast.Operand) {
	for _, op := range ops {
		if p, ok := op.(*ast.Param); ok {
			if e.dropped == nil {
				e.dropped = map[string]bool{}
			}
			e.dropped[p.Name.Name()] = true
		}
	}
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
		e.errorf(p.Name.Pos(), "parameter name %q does not work as a Go parameter: it is a Go keyword, not an ASCII identifier, the blank identifier _, which no expression reads, a name the generated signatures use (ctx, q, rt, w, r, opts, volt, cs, c), nil, append, any or true, which the generated functions use, or volt followed by an upper-case letter and no underscore, the form of the generated locals (§predicates)", name)
		return name
	}
	sp := SelectParam{GoName: name, GoType: goType}
	e.params[name] = &sp
	e.paramList = append(e.paramList, sp)
	return name
}

func goParamOK(name string) bool {
	if name == "" || name == "_" || goKeywords[name] || naming.RouteSignatureName(name) || naming.BodyBuiltin(name) || selectMethodNames[name] || naming.LocalName(name) {
		return false
	}
	return goIdentOK(name)
}

var selectMethodNames = map[string]bool{"q": true, "rt": true}

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
		b, _ := e.columnBind(x.Col)
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
			bb, unknown := e.columnBind(cr)
			if bb != nil {
				b = bb
			}
			if unknown {
				e.paramsDrop(x.X, x.Y)
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
	col, _ := e.columnBind(left.(*ast.ColRef))
	if col == nil {
		return rt.GenPred{}
	}
	var p rt.GenPred
	var err error
	switch o := right.(type) {
	case *ast.ColRef:
		other, _ := e.columnBind(o)
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
		if lit == classTime {
			e.errorf(l.Pos(), "column %q (%s) takes no literal: there is no date/time literal and a string is text; compare it with a parameter (§predicates)", b.name, b.goType)
			return "", false
		}
		if lit != classText {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the string %q (§predicates)", b.name, b.goType, l.Tok.Val)
			return "", false
		}
		return fmt.Sprintf("%q", l.Tok.Val), true
	default:
		if lit != classBool {
			e.errorf(l.Pos(), "column %q (%s) is not boolean (§predicates)", b.name, b.goType)
			return "", false
		}
		if l.Tok.Val == "true" {
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
	b, unknown := e.columnBind(x.Col)
	if unknown && x.List != nil {
		e.paramsDrop(x.List)
	}
	if b == nil {
		return rt.GenPred{}
	}
	if x.List != nil {
		if b.class == classEqOnly && b.storage == classForbidden {
			e.errorf(x.Col.Pos(), "column %q has declared type %s stored as []byte; it cannot take a list parameter: JSON carries no blob, and its text never equals the stored bytes (§predicates)", b.name, b.goType)
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
	b, unknown := e.columnBind(x.Col)
	if unknown {
		e.paramsDrop(x.Pattern)
	}
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

// WithFn is the select's nesting as the generator takes it, nil when
// none (§selects-over-groups).
func (si *SelectInfo) WithFn() *naming.SelectWith {
	w := si.With
	if w == nil {
		return nil
	}
	return &naming.SelectWith{Name: w.Name, TableKey: w.Table.Key, ParentKey: w.ParentKey, ChildKey: w.ChildKey, Field: w.Field, RowType: w.RowType}
}

// JoinFns lists the select's joins as the generator takes them, the
// joined table's column first in every pair (§selects-over-groups).
func (si *SelectInfo) JoinFns() []naming.SelectJoin {
	var out []naming.SelectJoin
	for _, j := range si.Joins {
		on := make([][2]string, len(j.On))
		for k, pair := range j.On {
			on[k] = [2]string{pair[1], pair[0]}
		}
		out = append(out, naming.SelectJoin{Name: j.Name, TableKey: j.Table.Key, Left: j.Left, On: on})
	}
	return out
}

// SelectFns lowers the package's checked selects into per-member
// generator instructions (§selects-over-groups); an unresolved select
// (§columns) is never lowered.
func (p *Package) SelectFns() []naming.SelectFn {
	var out []naming.SelectFn
	for _, si := range p.Selects {
		if si.Unresolved {
			continue
		}
		for _, m := range si.Members {
			fn := naming.SelectFn{
				TableKey:     m.Key,
				MethodSuffix: si.MethodSuffix,
				Where:        si.Where,
				Order:        si.Order,
				Limit:        si.Limit,
				LimitParam:   si.LimitParam,
				Items:        si.Items,
				Excluded:     si.Excluded,
				SharedType:   si.Shared,
				Joins:        si.JoinFns(),
				Group:        si.Group,
				Lookup:       si.Lookup,
				With:         si.WithFn(),
			}
			for _, prm := range si.Params {
				fn.Params = append(fn.Params, naming.SelectParam(prm))
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
