// Groups, predicates and selects (docs/spec.md §V9-§V11): resolution,
// the column-agreement rule, the typed closed expression language, and
// the lowering to SQL fragments the generator emits verbatim.
package lang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
)

// GroupInfo is one resolved Group (§V9): its declaration and member
// tables in first-addition order.
type GroupInfo struct {
	Decl    *ast.Group
	Members []*check.TableInfo
}

// SelectParam is one parameter of a generated select method, in first
// appearance order (§V11.6).
type SelectParam struct {
	SQLName string // as written after ':'
	GoName  string // Go parameter name (identical today; validated)
	GoType  string // e.g. "string", "int64", "time.Time"
}

// SelectInfo is one checked Select (§V11), lowered far enough that the
// generator only substitutes the table name. The agreement rule
// (§V11.4) is what makes one WHERE/ORDER rendering serve every member.
type SelectInfo struct {
	Decl         *ast.Select
	MethodSuffix string // GoName of the select name, e.g. "Rows"
	Members      []*check.TableInfo
	WhereSQL     string // "" when no where clause
	OrderSQL     string // "" when no order setting
	Params       []SelectParam
}

/* ===== collection ===== */

// dataQueries checks a package's groups, predicates and selects. Runs
// after the schema pass (tables resolved) per package.
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
				c.errorf(d.Name.Pos(), "V9", "group %q redeclared (§V9.1)", name)
				continue
			}
			groupDecls[name] = d
			groupOrder = append(groupOrder, name)
		case *ast.Pred:
			name := d.Name.Name()
			if _, dup := pkg.Preds[name]; dup {
				c.errorf(d.Name.Pos(), "V10", "predicate %q redeclared (§V10)", name)
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

	seen := map[string]*ast.Select{} // tableKey+method -> declaring select
	for _, sel := range selects {
		if si := c.selectCheck(sel, info); si != nil {
			for _, m := range si.Members {
				key := m.Key + "." + si.MethodSuffix
				if prev, dup := seen[key]; dup {
					c.errorf(sel.Name.Pos(), "V11",
						"select %q collides with select %q on table %q — both generate %s%s (§V11.1)",
						sel.Name.Name(), prev.Name.Name(), m.Decl.Name.Base(),
						modelOrBase(m), si.MethodSuffix)
					break
				}
				seen[key] = sel
			}
			pkg.Selects = append(pkg.Selects, si)
		}
	}
}

func modelOrBase(t *check.TableInfo) string {
	if m, err := golang.ModelName(t.Decl); err == nil {
		return m
	}
	return t.Decl.Name.Base()
}

/* ===== groups (§V9) ===== */

// groupResolve computes a group's member set, resolving group
// references depth-first with cycle detection (§V9.4).
func (c *checker) groupResolve(name string, decls map[string]*ast.Group, info *check.Info, path map[string]bool) *GroupInfo {
	if g, done := c.pkg.Groups[name]; done {
		return g
	}
	d := decls[name]
	if path[name] {
		c.errorf(d.Name.Pos(), "V9", "group %q is part of a reference cycle (§V9.4)", name)
		c.pkg.Groups[name] = &GroupInfo{Decl: d}
		return c.pkg.Groups[name]
	}
	path[name] = true
	defer delete(path, name)

	g := &GroupInfo{Decl: d}
	index := map[string]int{} // table key -> position in Members, -1 = removed
	add := func(pos token.Position, ti *check.TableInfo) {
		if i, present := index[ti.Key]; present && i >= 0 {
			c.errorf(pos, "V9", "table %q is already a member (§V9.3)", ti.Decl.Name.Base())
			return
		}
		index[ti.Key] = len(g.Members)
		g.Members = append(g.Members, ti)
	}
	remove := func(pos token.Position, ti *check.TableInfo) {
		i, present := index[ti.Key]
		if !present || i < 0 {
			c.errorf(pos, "V9", "table %q is not a member; nothing to remove (§V9.3)", ti.Decl.Name.Base())
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

	for _, term := range d.Terms {
		want := term.Name.Name()
		if ti := tableByBase(info, want); ti != nil {
			if term.Neg {
				remove(term.Name.Pos(), ti)
			} else {
				add(term.Name.Pos(), ti)
			}
			continue
		}
		if _, isGroup := decls[want]; isGroup {
			sg := c.groupResolve(want, decls, info, path)
			for _, ti := range sg.Members {
				if term.Neg {
					remove(term.Name.Pos(), ti)
				} else {
					add(term.Name.Pos(), ti)
				}
			}
			continue
		}
		c.nameMiss(term.Name, info, "V9", "group term")
	}
	if len(g.Members) == 0 {
		c.errorf(d.Name.Pos(), "V9", "group %q has no members (§V9.4)", name)
	}
	c.pkg.Groups[name] = g
	return g
}

// tableByBase finds a table by exact base name.
func tableByBase(info *check.Info, want string) *check.TableInfo {
	for _, ti := range info.Tables {
		if ti.Decl.Name.Base() == want {
			return ti
		}
	}
	return nil
}

// nameMiss reports an unresolved table-or-group name with the §V5.4
// did-you-mean aids.
func (c *checker) nameMiss(id *ast.Ident, info *check.Info, section, what string) {
	want := id.Name()
	for _, ti := range info.Tables {
		if strings.EqualFold(ti.Decl.Name.Base(), want) {
			c.errorf(id.Pos(), section, "unknown %s %q; did you mean %q? names are case-sensitive (§V9.2)", what, want, ti.Decl.Name.Base())
			return
		}
	}
	for _, ti := range info.Tables {
		if m, err := golang.ModelName(ti.Decl); err == nil && m == want {
			c.errorf(id.Pos(), section, "unknown %s %q; %q is the model of table %q — name the table (§V9.2)", what, want, want, ti.Decl.Name.Base())
			return
		}
	}
	c.errorf(id.Pos(), section, "unknown %s %q: no such table or group in package %q (§V9.2)", what, want, c.pkg.Path)
}

/* ===== predicates (§V10) ===== */

// predRefCycles rejects cyclic Pred references (§V10.2).
func (c *checker) predRefCycles(pkg *Package) {
	state := map[string]int{} // 0 unseen, 1 visiting, 2 done
	var visit func(name string, at token.Position)
	visit = func(name string, at token.Position) {
		switch state[name] {
		case 1:
			c.errorf(at, "V10", "predicate %q is part of a reference cycle (§V10.2)", name)
			return
		case 2:
			return
		}
		state[name] = 1
		if d := pkg.Preds[name]; d != nil {
			predWalk(d.X, func(r *ast.PredRef) {
				if pkg.Preds[r.Name.Name()] == nil {
					c.errorf(r.Name.Pos(), "V10", "unknown predicate %q (§V10.2)", r.Name.Name())
					return
				}
				visit(r.Name.Name(), r.Name.Pos())
			})
		}
		state[name] = 2
	}
	for name, d := range pkg.Preds {
		visit(name, d.Name.Pos())
	}
}

// predWalk visits every PredRef in an expression.
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

/* ===== selects (§V11) ===== */

// typeClass buckets a declared column type for operator rules (§V10.3,
// §V10.4). Agreement is judged on the Go type; orderability on the
// declared class, so decimal (string-mapped, exact) is eq-only.
type typeClass int

const (
	classForbidden typeClass = iota // blob, json: no predicate use
	classNumeric
	classText
	classTime
	classBool
	classEqOnly // decimal/numeric/money: = != in only
)

func classOf(declType string) typeClass {
	base := strings.ToLower(declType)
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	switch base {
	case "decimal", "numeric", "money":
		return classEqOnly
	case "bool", "boolean":
		return classBool
	case "date", "datetime", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone":
		return classTime
	}
	goType, ok := golang.GoTypeName(base)
	if !ok {
		return classForbidden
	}
	switch {
	case strings.HasPrefix(goType, "int"), strings.HasPrefix(goType, "uint"), strings.HasPrefix(goType, "float"):
		return classNumeric
	case goType == "string":
		return classText
	case goType == "time.Time":
		return classTime
	case goType == "bool":
		return classBool
	}
	return classForbidden // []byte, json.RawMessage, enums handled below
}

// colBinding is one column's agreed meaning across the select's members.
type colBinding struct {
	name   string
	goType string
	class  typeClass
}

// selectCheck resolves and types one Select (§V11) and lowers it to a
// SelectInfo, or reports why not.
func (c *checker) selectCheck(sel *ast.Select, info *check.Info) *SelectInfo {
	si := &SelectInfo{Decl: sel}

	// §V11.1: method suffix.
	suffix, err := golang.GoName(sel.Name.Name())
	if err != nil {
		c.errorf(sel.Name.Pos(), "V11", "select name %q does not map to a Go identifier: %v (§V11.1)", sel.Name.Name(), err)
		return nil
	}
	si.MethodSuffix = suffix
	if crudMethodSuffixes[suffix] {
		c.errorf(sel.Name.Pos(), "V11", "select name %q collides with the generated CRUD surface (§V11.1)", suffix)
		return nil
	}

	// §V11.2: target.
	want := sel.Target.Name()
	if g, isGroup := c.pkg.Groups[want]; isGroup {
		si.Members = g.Members
	} else if ti := tableByBase(info, want); ti != nil {
		si.Members = []*check.TableInfo{ti}
	} else {
		c.nameMiss(sel.Target, info, "V11", "select target")
		return nil
	}
	if len(si.Members) == 0 {
		return nil // the group already errored
	}

	env := &selectEnv{
		c:       c,
		sel:     sel,
		members: si.Members,
		cols:    map[string]*colBinding{},
		params:  map[string]*SelectParam{},
	}

	whereSQL := ""
	if sel.Where != nil {
		whereSQL = env.exprCheck(sel.Where, map[string]bool{})
	}

	orderSQL := c.selectSettings(sel, env)
	if env.failed {
		return nil
	}
	si.WhereSQL = whereSQL
	si.OrderSQL = orderSQL
	si.Params = env.paramList
	return si
}

// crudMethodSuffixes are the fixed per-model method suffixes the CRUD
// and dynamic generators mint (Appendix A); selects must not collide.
var crudMethodSuffixes = map[string]bool{
	"Get": true, "GetMany": true, "List": true, "Create": true,
	"Update": true, "Delete": true, "Count": true, "Exists": true,
	"Query": true, "DeleteWhere": true, "UpdateWhere": true,
	"Limit": true, "Offset": true, "OrderBy": true,
}

// selectSettings checks the settings list ([order: ...] only, §V11.5)
// and renders ORDER BY.
func (c *checker) selectSettings(sel *ast.Select, env *selectEnv) string {
	if sel.Settings == nil {
		return ""
	}
	var parts []string
	for _, s := range sel.Settings.Settings {
		switch strings.ToLower(s.Name) {
		case "order":
			list, ok := s.Value.(*ast.IdentList)
			if !ok {
				c.errorf(s.Pos(), "V11", "order: takes a parenthesized column list (§V11.5)")
				continue
			}
			for i, id := range list.Names {
				b := env.columnBind(id)
				if b == nil {
					continue
				}
				if b.class != classNumeric && b.class != classTime && b.class != classText {
					c.errorf(id.Pos(), "V11", "column %q (%s) is not orderable (§V11.5)", b.name, b.goType)
					continue
				}
				// Rendered with an explicit direction either way, so the
				// emitted SQL answers "and id?" by itself (§V11.5).
				dir := " ASC"
				if mod := list.Mods[i]; mod != nil {
					switch strings.ToLower(mod.Name()) {
					case "asc":
					case "desc":
						dir = " DESC"
					default:
						c.errorf(mod.Pos(), "V11", "order direction is asc or desc, not %q (§V11.5)", mod.Name())
						continue
					}
				}
				parts = append(parts, sqlite.Ident(b.name)+dir)
			}
		default:
			c.errorf(s.Pos(), "V11", "unknown select setting %q; selects take [order: (...)] (§V11.5)", s.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// selectEnv threads the agreement rule and param typing through one
// select's expression checking, and renders SQL as it goes.
type selectEnv struct {
	c         *checker
	sel       *ast.Select
	members   []*check.TableInfo
	cols      map[string]*colBinding
	params    map[string]*SelectParam
	paramList []SelectParam
	failed    bool
}

func (e *selectEnv) errorf(pos token.Position, format string, args ...any) {
	e.failed = true
	e.c.errorf(pos, "V11", format, args...)
}

// columnBind applies the agreement rule (§V11.4): the column must exist
// in every member with one Go type.
func (e *selectEnv) columnBind(id *ast.Ident) *colBinding {
	name := id.Name()
	if b, done := e.cols[name]; done {
		return b
	}
	var missing []string
	types := map[string][]string{} // goType -> member tables
	classes := map[typeClass]bool{}
	var class typeClass
	for _, m := range e.members {
		cd := m.Column(name)
		if cd == nil {
			missing = append(missing, m.Decl.Name.Base())
			continue
		}
		declType := cd.Col.Type.String()
		goType, ok := golang.GoTypeName(normalizeType(declType))
		if !ok {
			// Enum-typed or unmapped: not usable in predicates (v1).
			e.errorf(id.Pos(), "column %q of table %q has type %q, which predicates cannot use (§V10.3)", name, m.Decl.Name.Base(), declType)
			return nil
		}
		types[goType] = append(types[goType], m.Decl.Name.Base())
		class = classOf(declType)
		classes[class] = true
	}
	if len(missing) > 0 {
		e.errorf(id.Pos(), "column %q is missing from %s — every member of %q must have it (§V11.4)",
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
		e.errorf(id.Pos(), "column %q disagrees on type across %q: %s — the predicate cannot be checked for every member (§V11.4)",
			name, e.sel.Target.Name(), strings.Join(parts, "; "))
		return nil
	}
	if len(classes) > 1 {
		// Same Go type, different class (e.g. text vs decimal): take the
		// stricter class so operators stay sound.
		if classes[classEqOnly] {
			class = classEqOnly
		}
	}
	if class == classForbidden {
		e.errorf(id.Pos(), "column %q (blob/json) cannot appear in a predicate (§V10.3)", name)
		return nil
	}
	var goType string
	for t := range types {
		goType = t
	}
	b := &colBinding{name: name, goType: goType, class: class}
	e.cols[name] = b
	return b
}

func normalizeType(t string) string {
	base := strings.ToLower(t)
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	return base
}

func tableList(names []string) string {
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ", "), len(names)-3)
	}
	return strings.Join(names, ", ")
}

// paramBind records a parameter's type; one name, one type (§V10.3).
func (e *selectEnv) paramBind(p *ast.Param, goType string) string {
	name := p.Name.Name()
	if prev, seen := e.params[name]; seen {
		if prev.GoType != goType {
			e.errorf(p.Pos(), "parameter :%s used as %s here but %s earlier — one name, one type (§V10.3)", name, goType, prev.GoType)
		}
		return ":" + name
	}
	if !goParamOK(name) {
		e.errorf(p.Name.Pos(), "parameter name %q does not work as a Go parameter (§V10)", name)
		return ":" + name
	}
	sp := SelectParam{SQLName: name, GoName: name, GoType: goType}
	e.params[name] = &sp
	e.paramList = append(e.paramList, sp)
	return ":" + name
}

func goParamOK(name string) bool {
	if name == "" || goKeywords[name] || reservedParamNames[name] || name == "ctx" || name == "q" {
		return false
	}
	return goIdentOK(name)
}

// exprCheck types an expression and renders its SQL (§V10.3). preds
// tracks in-flight Pred expansions for cycle safety at use sites.
func (e *selectEnv) exprCheck(x ast.PredExpr, preds map[string]bool) string {
	switch x := x.(type) {
	case *ast.PredBinary:
		op := strings.ToUpper(x.Op)
		return e.exprCheck(x.X, preds) + " " + op + " " + e.exprCheck(x.Y, preds)
	case *ast.PredNot:
		return "NOT " + e.exprCheck(x.X, preds)
	case *ast.PredParen:
		return "(" + e.exprCheck(x.X, preds) + ")"
	case *ast.PredRef:
		name := x.Name.Name()
		d := e.c.pkg.Preds[name]
		if d == nil {
			e.errorf(x.Name.Pos(), "unknown predicate %q (§V10.2)", name)
			return "1"
		}
		if preds[name] {
			return "1" // cycle already reported by predRefCycles
		}
		preds[name] = true
		sql := "(" + e.exprCheck(d.X, preds) + ")"
		delete(preds, name)
		return sql
	case *ast.PredCompare:
		return e.compareCheck(x)
	case *ast.PredIn:
		return e.inCheck(x)
	case *ast.PredLike:
		return e.likeCheck(x)
	case *ast.PredNull:
		b := e.columnBind(x.Col)
		if b == nil {
			return "1"
		}
		if x.Not {
			return sqlite.Ident(b.name) + " IS NOT NULL"
		}
		return sqlite.Ident(b.name) + " IS NULL"
	}
	return "1"
}

var compareSQL = map[token.Kind]string{
	token.EQ: "=", token.NEQ: "!=", token.LT: "<",
	token.LE: "<=", token.GT: ">", token.GE: ">=",
}

func (e *selectEnv) compareCheck(x *ast.PredCompare) string {
	ordering := x.Op != token.EQ && x.Op != token.NEQ

	// Resolve the column side first: it anchors the type.
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
		e.errorf(x.Pos(), "a comparison needs a column on one side (§V10)")
		return "1"
	}
	if b == nil {
		return "1"
	}
	if ordering {
		switch b.class {
		case classNumeric, classTime:
		case classEqOnly:
			e.errorf(x.Pos(), "column %q is decimal-exact (%s); it supports = and != only (§V10.3-§V10.4)", b.name, b.goType)
			return "1"
		default:
			e.errorf(x.Pos(), "column %q (%s) is not orderable; < <= > >= need numeric or date/time operands (§V10.3)", b.name, b.goType)
			return "1"
		}
	}

	render := func(o ast.Operand) string {
		switch o := o.(type) {
		case *ast.ColRef:
			bb := e.columnBind(o.Name)
			if bb == nil {
				return "1"
			}
			if bb.goType != b.goType {
				e.errorf(o.Pos(), "cannot compare column %q (%s) with column %q (%s) (§V10.3)", b.name, b.goType, bb.name, bb.goType)
			}
			return sqlite.Ident(bb.name)
		case *ast.Param:
			return e.paramBind(o, b.goType)
		case *ast.Lit:
			return e.litRender(o, b)
		}
		return "1"
	}
	return render(x.X) + " " + compareSQL[x.Op] + " " + render(x.Y)
}

// litRender types a literal against a column binding and renders it.
func (e *selectEnv) litRender(l *ast.Lit, b *colBinding) string {
	switch l.Tok.Kind {
	case token.NUMBER:
		if b.class != classNumeric {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the number %s (§V10.3)", b.name, b.goType, l.Tok.Val)
			return "1"
		}
		return l.Tok.Val
	case token.STRING:
		if b.class != classText && b.class != classEqOnly && b.class != classTime {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the string %q (§V10.3)", b.name, b.goType, l.Tok.Val)
			return "1"
		}
		return "'" + strings.ReplaceAll(l.Tok.Val, "'", "''") + "'"
	default: // true/false idents
		if b.class != classBool {
			e.errorf(l.Pos(), "column %q (%s) is not boolean (§V10.3)", b.name, b.goType)
			return "1"
		}
		if strings.EqualFold(l.Tok.Val, "true") {
			return "1"
		}
		return "0"
	}
}

func (e *selectEnv) inCheck(x *ast.PredIn) string {
	b := e.columnBind(x.Col)
	if b == nil {
		return "1"
	}
	var items []string
	for _, it := range x.Items {
		items = append(items, e.litRender(it, b))
	}
	return sqlite.Ident(b.name) + " IN (" + strings.Join(items, ", ") + ")"
}

func (e *selectEnv) likeCheck(x *ast.PredLike) string {
	b := e.columnBind(x.Col)
	if b == nil {
		return "1"
	}
	if b.class != classText {
		e.errorf(x.Col.Pos(), "like needs a text column; %q is %s (§V10.3)", b.name, b.goType)
		return "1"
	}
	switch p := x.Pattern.(type) {
	case *ast.Lit:
		return sqlite.Ident(b.name) + " LIKE '" + strings.ReplaceAll(p.Tok.Val, "'", "''") + "'"
	case *ast.Param:
		return sqlite.Ident(b.name) + " LIKE " + e.paramBind(p, "string")
	}
	return "1"
}

/* ===== lowering for the generator ===== */

// SelectFns lowers the package's checked selects into per-member
// generator instructions (§V11.6).
func (p *Package) SelectFns() []golang.SelectFn {
	var out []golang.SelectFn
	for _, si := range p.Selects {
		for _, m := range si.Members {
			fn := golang.SelectFn{
				TableKey:     m.Key,
				MethodSuffix: si.MethodSuffix,
				WhereSQL:     si.WhereSQL,
				OrderSQL:     si.OrderSQL,
			}
			for _, prm := range si.Params {
				fn.Params = append(fn.Params, golang.SelectParam(prm))
			}
			out = append(out, fn)
		}
	}
	return out
}
