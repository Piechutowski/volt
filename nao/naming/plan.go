package naming

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
)

// Plan is a checked package's naming plan: every Go and SQL name of
// every table and column, decided once, plus
// the generated CRUD method table indexed by method name. A Plan is
// immutable after PlanBuild and safe to share between goroutines.
type Plan struct {
	memo *PlanMemo
	f    *ast.File
	info *check.Info
	p    *plan
	err  error

	byKey   map[string]*Table
	methods map[string]crudRef
	names   []string
	base    *nameBase
}

type crudRef struct {
	tableKey string
	method   CRUDMethod
}

// PlanBuild plans one checked file (a package's merged declarations).
// A plan that cannot be built (an unmappable column type, a Go name
// collision) still answers: Err reports the reason and every lookup
// fails with it.
func PlanBuild(f *ast.File, info *check.Info) *Plan {
	return PlanBuildMemo(f, info, nil)
}

// PlanMemo remembers a package's table models across builds of its
// plan: a table that is the checked object it was is the same model.
// The zero value is ready; a PlanMemo belongs to one package.
type PlanMemo struct {
	prev, next map[*check.TableInfo]*memoModel
	base       *nameBase
	// Hits and Misses count the models reused and built by the last
	// PlanBuildMemo.
	Hits, Misses int
}

type memoModel struct {
	tm      *Table
	imports map[string]bool
}

// PlanBuildMemo is PlanBuild with a memo of the package's earlier
// plans; a nil memo builds everything.
func PlanBuildMemo(f *ast.File, info *check.Info, memo *PlanMemo) *Plan {
	pl := &Plan{f: f, info: info, memo: memo}
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
	}
	p, err := planBuild(f, info, memo)
	if memo != nil {
		memo.prev, memo.next = memo.next, nil
	}
	if err != nil {
		pl.err = err
		return pl
	}
	pl.p = p
	if memo != nil {
		if memo.base == nil {
			memo.base = newNameBase()
		}
		pl.base = memo.base
	} else {
		pl.base = newNameBase()
	}
	pl.base.update(p.tables)
	pl.byKey = make(map[string]*Table, len(p.tables))
	pl.methods = make(map[string]crudRef, 5*len(p.tables))
	for _, t := range p.tables {
		pl.byKey[t.Info.Key] = t
		for _, m := range crudMethodsOf(t) {
			pl.names = append(pl.names, m.Name)
			if _, dup := pl.methods[m.Name]; !dup {
				pl.methods[m.Name] = crudRef{tableKey: t.Info.Key, method: m}
			}
		}
	}
	return pl
}

// Err reports why the plan could not be built, or nil.
func (pl *Plan) Err() error { return pl.err }

// File is the checked file the plan was built from, a package's merged
// declarations (spec §projects-and-packages).
func (pl *Plan) File() *ast.File { return pl.f }

// Info is the schema facts the plan was built from (spec §static-semantics).
func (pl *Plan) Info() *check.Info { return pl.info }

// Tables lists every table's plan in declaration order, in a slice the
// caller owns; nil when the plan could not be built (spec §mapping-to-go).
func (pl *Plan) Tables() []*Table {
	if pl.p == nil {
		return nil
	}
	return append([]*Table(nil), pl.p.tables...)
}

// Table is the plan of the table with the given key, or the reason there
// is none: the plan's own error, or no such table (spec §mapping-to-go).
func (pl *Plan) Table(key string) (*Table, error) { return pl.table(key) }

// NameCollisionErr is the first package-scope name collision the generated
// files would produce, as an error; nil when there is none (spec
// §changesets).
func (pl *Plan) NameCollisionErr() error {
	cs := pl.DynNameCollisions()
	if len(cs) == 0 {
		return nil
	}
	c := cs[0]
	return fmt.Errorf("%s and %s both need the Go name %s; rename one (e.g. with [model:])", c.First, c.Second, c.Name)
}

// SelectRowName is the row type SelectRowType names, without its
// fields: the model, the shared type, or the model with the method
// suffix when columns are excluded.
func (pl *Plan) SelectRowName(fn SelectFn) (string, error) {
	t, err := pl.table(fn.TableKey)
	if err != nil {
		return "", err
	}
	switch {
	case fn.SharedType != "":
		return fn.SharedType, nil
	case len(fn.Excluded) > 0:
		return t.SelectMethod(fn), nil
	}
	return t.Model, nil
}

// ModelRef is the identity of a table's model: the same value across
// plans exactly when the model was reused from the plan's memo, so a
// memo downstream can key on it. nil when the plan has no such
// table.
func (pl *Plan) ModelRef(tableKey string) any {
	t, err := pl.table(tableKey)
	if err != nil {
		return nil
	}
	return t
}

func (pl *Plan) table(key string) (*Table, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	t := pl.byKey[key]
	if t == nil {
		return nil, fmt.Errorf("no table %q", key)
	}
	return t, nil
}

// CRUDMethods lists the default CRUD methods the query generator emits
// for one table, with the same existence rules it applies: Get, Update
// and Delete need a primary key, Update needs a non-key column, Create
// and Update take a changeset (spec §changesets).
func (pl *Plan) CRUDMethods(tableKey string) (model string, methods []CRUDMethod, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return "", nil, err
	}
	return t.Model, crudMethodsOf(t), nil
}

// CRUDMethod finds the generated CRUD method with the given name; when
// two tables would mint it, the table declared first owns it.
func (pl *Plan) CRUDMethod(name string) (tableKey string, m CRUDMethod, ok bool) {
	ref, ok := pl.methods[name]
	return ref.tableKey, ref.method, ok
}

// CRUDMethodNames lists every generated CRUD method name in declaration
// order, in a slice the caller owns.
func (pl *Plan) CRUDMethodNames() []string { return append([]string(nil), pl.names...) }

// QueryName reports whether name is a method of the generated Queries
// handle, or the exported stem of a statement constant the query files
// declare, and what mints it (spec §selects-over-groups,
// §generated-query-surface): a select's method and constant must take
// none of them.
func (pl *Plan) QueryName(name string) (string, bool) {
	switch name {
	case "WithCache", "WithTx", "Tx":
		return "the Queries method " + name, true
	}
	if pl.err != nil {
		return "", false
	}
	if ref, ok := pl.methods[name]; ok {
		return fmt.Sprintf("the CRUD method %s of table %q", name, ref.tableKey), true
	}
	for _, t := range pl.p.tables {
		if len(t.Fields) == 0 || !strings.HasPrefix(name, t.Model) {
			continue
		}
		tbl, n := t.Info.Decl.Name.Name(), &t.Names
		switch name {
		case n.GetMany:
			if len(t.PK) == 1 {
				return fmt.Sprintf("the method %s of table %q", name, tbl), true
			}
		case n.Query, n.Count, n.Exists, n.DeleteWhere, n.UpdateWhere:
			return fmt.Sprintf("the dynamic-layer method %s of table %q", name, tbl), true
		}
		if c := "volt" + name + "SQL"; c == n.TableConst || c == n.ColumnsConst {
			return fmt.Sprintf("the statement constant %s of table %q", c, tbl), true
		}
		for _, f := range t.UniqueFields() {
			if name == t.GetBy(f) {
				return fmt.Sprintf("the method %s of table %q", name, tbl), true
			}
		}
	}
	return "", false
}

func crudMethodsOf(t *Table) []CRUDMethod { return t.crud }

func crudMethodsBuild(t *Table) []CRUDMethod {
	var methods []CRUDMethod
	var key []SelectParam
	for _, fp := range t.PK {
		key = append(key, SelectParam{GoName: fp.Arg, GoType: fp.BaseType})
	}
	m, n := t.Model, &t.Names
	if len(t.PK) > 0 {
		methods = append(methods, CRUDMethod{Name: n.Get, Op: "get", Key: key, Result: m})
	}
	methods = append(methods, CRUDMethod{Name: n.List, Op: "list", Result: m, Many: true})
	methods = append(methods, CRUDMethod{Name: n.Create, Op: "create", Body: n.Changeset, Params: n.Params,
		New: n.New, Allow: t.handles(t.Settable()), Result: m})
	if len(t.PK) > 0 {
		if len(t.NonPK()) > 0 {
			methods = append(methods, CRUDMethod{Name: n.Update, Op: "update", Key: key, Body: n.Changeset, Params: n.Params,
				Get: n.Get, Allow: t.handles(t.NonPK()), Result: m})
		}
		methods = append(methods, CRUDMethod{Name: n.Delete, Op: "delete", Key: key})
	}
	return methods
}

// ModelFields reports the exact struct fields the model generator emits
// for the named table (spec §selects-over-groups, §mapping-to-go).
func (pl *Plan) ModelFields(tableKey string) (model string, fields []FieldSig, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return "", nil, err
	}
	return t.Model, t.sigs, nil
}

func fieldSigsBuild(t *Table) []FieldSig {
	sigs := make([]FieldSig, 0, len(t.Fields))
	for _, fp := range t.Fields {
		sigs = append(sigs, fieldSigOf(fp))
	}
	return sigs
}

func fieldSigOf(fp *Field) FieldSig {
	return FieldSig{
		Name: fp.GoField, Col: fp.ColName, Type: fp.GoType,
		Tag: fp.Tag, Doc: SettingNote(fp.Col.Settings),
		Nullable: fp.Nullable,
	}
}

// SelectRowType names the row type one instantiation returns (§selects-over-groups)
// and reports its fields.
func (pl *Plan) SelectRowType(fn SelectFn) (string, []FieldSig, error) {
	t, err := pl.table(fn.TableKey)
	if err != nil {
		return "", nil, err
	}
	cols, err := t.SelectColumns(fn)
	if err != nil {
		return "", nil, err
	}
	row, err := pl.SelectRowName(fn)
	if err != nil {
		return "", nil, err
	}
	sigs := make([]FieldSig, 0, len(cols))
	for _, fp := range cols {
		sigs = append(sigs, fieldSigOf(fp))
	}
	return row, sigs, nil
}

// SelectStatement renders the SQL one select instantiation runs through
// the runtime (spec §predicates), with the Go expressions the
// method binds in placeholder order.
func (pl *Plan) SelectStatement(fn SelectFn) (string, []string, error) {
	t, err := pl.table(fn.TableKey)
	if err != nil {
		return "", nil, err
	}
	return t.SelectSQL(fn)
}

// DynNameCollisions reports the package-scope Go name collisions the
// generated files would produce, in declaration order; a plan that could
// not be built reports nothing.
func (pl *Plan) DynNameCollisions() []NameCollision {
	if pl.err != nil {
		return nil
	}
	seen := map[string]dynOrigin{"Queries": {kind: dynQueries}}
	var out []NameCollision
	for _, td := range pl.info.Types {
		if td.Path == "" {
			out = dynArrive(out, seen, td.Key, dynOrigin{kind: dynType, td: td, pos: td.Decl.Name.Pos()})
		}
	}
	type arrival struct {
		first, o  dynOrigin
		pos, rank int
	}
	var arrivals []arrival
	for name, first := range seen {
		for _, o := range pl.base.origins[name].all() {
			if o.dyn {
				arrivals = append(arrivals, arrival{first, dynOriginOf(o), pl.base.pos[o.table], o.rank})
			}
		}
	}
	for name := range pl.base.dups {
		if _, ok := seen[name]; ok {
			continue
		}
		var origins []*nameOrigin
		for _, o := range pl.base.origins[name].all() {
			if o.dyn {
				origins = append(origins, o)
			}
		}
		sort.Slice(origins, func(i, j int) bool {
			a, b := origins[i], origins[j]
			if pl.base.pos[a.table] != pl.base.pos[b.table] {
				return pl.base.pos[a.table] < pl.base.pos[b.table]
			}
			return a.rank < b.rank
		})
		first := dynOriginOf(origins[0])
		for _, o := range origins[1:] {
			arrivals = append(arrivals, arrival{first, dynOriginOf(o), pl.base.pos[o.table], o.rank})
		}
	}
	sort.Slice(arrivals, func(i, j int) bool {
		a, b := arrivals[i], arrivals[j]
		if a.pos != b.pos {
			return a.pos < b.pos
		}
		return a.rank < b.rank
	})
	for _, a := range arrivals {
		out = append(out, dynPair(a.o.name(), a.first, a.o))
	}
	return out
}

func dynArrive(out []NameCollision, seen map[string]dynOrigin, name string, o dynOrigin) []NameCollision {
	prev, dup := seen[name]
	if !dup {
		seen[name] = o
		return out
	}
	return append(out, dynPair(name, prev, o))
}

func dynPair(name string, first, second dynOrigin) NameCollision {
	if second.pos.Line() < first.pos.Line() || (second.pos.Line() == first.pos.Line() && second.pos.Column() < first.pos.Column()) {
		first, second = second, first
	}
	nc := NameCollision{Name: name, First: first.describe(), Second: second.describe(), Pos: second.pos}
	switch {
	case first.kind == dynType:
		nc.Type, nc.With = first.td, second.describe()
	case second.kind == dynType:
		nc.Type, nc.With = second.td, first.describe()
	}
	return nc
}

func dynOriginOf(o *nameOrigin) dynOrigin {
	d := dynOrigin{tm: o.table, pos: o.table.Info.Decl.Pos()}
	switch o.kind {
	case nameModel:
		d.kind = dynModel
	case nameChangeset:
		d.kind = dynChangeset
	case nameColumn:
		d.kind = dynColumn
	case nameParams:
		d.kind = dynParams
	case nameNew:
		d.kind = dynNew
	case nameHandle:
		d.kind, d.f, d.pos = dynHandle, o.field, o.field.Col.Pos()
	case nameDynFunc:
		d.kind, d.sfx = dynWrapper, o.name[len(o.table.Model):]
	}
	return d
}

// Names is the scope of package-level Go identifiers a package's
// generated files mint (§selects-over-groups): models, changesets with
// their column and params aliases and constructors (§changesets), column
// handles and option wrappers, the Queries handle and its constructor,
// beside the package's bare declared types, which its own Go files
// declare (§types). It is read in layers, last writer first.
type Names struct {
	base  *nameBase
	types map[string]*check.TypeInfo
	added map[string]string
}

type nameKind uint8

const (
	nameModel nameKind = iota
	nameChangeset
	nameColumn
	nameParams
	nameNew
	nameDynFunc
	nameHandle
)

type nameOrigin struct {
	name  string
	kind  nameKind
	idx   int
	table *Table
	field *Field
	dyn   bool
	rank  int
}

func modelNamesBuild(t *Table) []nameOrigin {
	wrappers := t.Names.wrappers()
	names := make([]nameOrigin, 0, 5+len(wrappers)+len(t.Fields))
	add := func(name string, kind nameKind, field *Field, dyn bool, rank int) {
		names = append(names, nameOrigin{name: name, kind: kind, idx: len(names), table: t, field: field, dyn: dyn, rank: rank})
	}
	queryable := len(t.Fields) > 0
	add(t.Model, nameModel, nil, true, 0)
	add(t.Names.Changeset, nameChangeset, nil, queryable, 1)
	add(t.Names.Column, nameColumn, nil, queryable, 2)
	add(t.Names.Params, nameParams, nil, queryable, 3)
	add(t.Names.New, nameNew, nil, queryable, 4)
	for i, w := range wrappers {
		add(w, nameDynFunc, nil, queryable, 5+len(t.Fields)+i)
	}
	for i, fp := range t.Fields {
		add(t.Handle(fp), nameHandle, fp, queryable, 5+i)
	}
	return names
}

func (o *nameOrigin) describe() string {
	switch o.kind {
	case nameModel:
		return fmt.Sprintf("the model of table %q", o.table.Info.Decl.Name.Name())
	case nameChangeset:
		return fmt.Sprintf("the changeset of table %q", o.table.Info.Decl.Name.Name())
	case nameColumn:
		return fmt.Sprintf("the column handle type of table %q", o.table.Info.Decl.Name.Name())
	case nameParams:
		return fmt.Sprintf("the write body type of table %q", o.table.Info.Decl.Name.Name())
	case nameNew:
		return fmt.Sprintf("the changeset constructor of table %q", o.table.Info.Decl.Name.Name())
	case nameHandle:
		return fmt.Sprintf("the column handle for %s.%s", o.table.Info.Decl.Name.Name(), o.field.ColName)
	}
	return "the dynamic-layer function " + o.name
}

type nameBase struct {
	origins map[string]nameSlot
	models  map[*Table]bool
	pos     map[*Table]int
	dups    map[string]bool
}

type nameSlot struct {
	first *nameOrigin
	more  []*nameOrigin
}

func newNameBase() *nameBase {
	return &nameBase{origins: map[string]nameSlot{}, models: map[*Table]bool{}, dups: map[string]bool{}}
}

func (b *nameBase) update(tables []*Table) {
	current := make(map[*Table]bool, len(tables))
	b.pos = make(map[*Table]int, len(tables))
	for i, t := range tables {
		current[t] = true
		b.pos[t] = i
	}
	for t := range b.models {
		if !current[t] {
			b.remove(t)
		}
	}
	for _, t := range tables {
		if !b.models[t] {
			b.add(t)
		}
	}
}

func (b *nameBase) add(t *Table) {
	b.models[t] = true
	for i := range t.names {
		o := &t.names[i]
		slot := b.origins[o.name]
		if slot.first == nil {
			slot.first = o
		} else {
			slot.more = append(slot.more, o)
		}
		b.origins[o.name] = slot
		b.dupsMark(o.name)
	}
}

func (b *nameBase) remove(t *Table) {
	delete(b.models, t)
	for i := range t.names {
		o := &t.names[i]
		slot := b.origins[o.name]
		var kept []*nameOrigin
		for _, x := range slot.more {
			if x != o {
				kept = append(kept, x)
			}
		}
		if slot.first == o {
			if len(kept) == 0 {
				delete(b.origins, o.name)
				b.dupsMark(o.name)
				continue
			}
			slot.first, kept = kept[0], kept[1:]
		}
		slot.more = kept
		b.origins[o.name] = slot
		b.dupsMark(o.name)
	}
}

func (b *nameBase) dupsMark(name string) {
	n := 0
	for _, o := range b.origins[name].all() {
		if o.dyn {
			n++
		}
	}
	if n >= 2 {
		b.dups[name] = true
	} else {
		delete(b.dups, name)
	}
}

func (s nameSlot) all() []*nameOrigin {
	if s.first == nil {
		return nil
	}
	return append([]*nameOrigin{s.first}, s.more...)
}

func (b *nameBase) lookup(name string) *nameOrigin {
	slot := b.origins[name]
	best := slot.first
	for _, o := range slot.more {
		if b.pos[o.table] > b.pos[best.table] || (b.pos[o.table] == b.pos[best.table] && o.idx > best.idx) {
			best = o
		}
	}
	return best
}

// Names returns the package's name scope for one check: the two fixed
// names alone when the plan could not be built.
func (pl *Plan) Names() *Names {
	n := &Names{base: pl.base, types: map[string]*check.TypeInfo{}, added: map[string]string{}}
	if pl.err != nil {
		return n
	}
	for _, td := range pl.info.Types {
		if td.Path == "" {
			n.types[td.Key] = td
		}
	}
	return n
}

// Add records a minted name with its description; a later Add of the
// same name replaces the earlier one.
func (n *Names) Add(name, desc string) {
	n.added[name] = desc
}

// Lookup reports whether name is taken and, if so, by what.
func (n *Names) Lookup(name string) (desc string, ok bool) {
	if desc, ok := n.added[name]; ok {
		return desc, true
	}
	if _, ok := n.types[name]; ok {
		return fmt.Sprintf("the declared type %s", name), true
	}
	return n.Generated(name)
}

// Generated is Lookup over the names generated code mints alone, the
// declared types and the names added by Add left out.
func (n *Names) Generated(name string) (desc string, ok bool) {
	if n.base != nil {
		if o := n.base.lookup(name); o != nil {
			return o.describe(), true
		}
	}
	switch name {
	case "Queries":
		return "the generated Queries handle", true
	case "New":
		return "the generated constructor", true
	}
	return "", false
}
