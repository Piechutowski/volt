// The shared naming plan: one immutable Plan per checked
// package, built once after the schema pass and read by the Volt
// checker, the generators, vet and the language server.
package golang

import (
	"fmt"
	"sort"

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

	byKey   map[string]*tableModel
	methods map[string]crudRef
	names   []string
	base    *nameBase
}

type crudRef struct {
	tableKey string
	method   CRUDMethod
}

// PlanBuild plans one checked file (a package's merged declarations).
// A plan that cannot be built — an unmappable column type, a Go name
// collision — still answers: Err reports the reason and every lookup
// fails with it.
func PlanBuild(f *ast.File, info *check.Info) *Plan {
	return PlanBuildMemo(f, info, nil)
}

// PlanMemo remembers a package's table models across builds of its
// plan: a table that is the checked object it was, under the same
// enum types, is the same model. The zero value is ready; a
// PlanMemo belongs to one package.
type PlanMemo struct {
	prev, next map[*check.TableInfo]*memoModel
	base       *nameBase
	// Hits and Misses count the models reused and built by the last
	// PlanBuildMemo.
	Hits, Misses int
}

type memoModel struct {
	tm      *tableModel
	imports map[string]bool
	enums   []enumType
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
	pl.byKey = make(map[string]*tableModel, len(p.tables))
	pl.methods = make(map[string]crudRef, 5*len(p.tables))
	for _, t := range p.tables {
		pl.byKey[t.ti.Key] = t
		for _, m := range crudMethodsOf(t) {
			pl.names = append(pl.names, m.Name)
			if _, dup := pl.methods[m.Name]; !dup {
				pl.methods[m.Name] = crudRef{tableKey: t.ti.Key, method: m}
			}
		}
	}
	return pl
}

// Err reports why the plan could not be built, or nil.
func (pl *Plan) Err() error { return pl.err }

// Models renders the models file (nao_models.go) for the planned package.
func (pl *Plan) Models(opts Options) ([]byte, error) {
	return modelsGenerate(pl.f, pl.info, pl.p, pl.err, opts)
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
		return t.model + fn.MethodSuffix, nil
	}
	return t.model, nil
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

func (pl *Plan) table(key string) (*tableModel, error) {
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
	return t.model, crudMethodsOf(t), nil
}

// CRUDMethod finds the generated CRUD method with the given name; when
// two tables would mint it, the table declared first owns it.
func (pl *Plan) CRUDMethod(name string) (tableKey string, m CRUDMethod, ok bool) {
	ref, ok := pl.methods[name]
	return ref.tableKey, ref.method, ok
}

// CRUDMethodNames lists every generated CRUD method name in declaration
// order.
func (pl *Plan) CRUDMethodNames() []string { return pl.names }

func crudMethodsOf(t *tableModel) []CRUDMethod { return t.crud }

func crudMethodsBuild(t *tableModel) []CRUDMethod {
	var methods []CRUDMethod
	var key []SelectParam
	for _, fp := range t.pk {
		key = append(key, SelectParam{SQLName: fp.param, GoName: fp.arg, GoType: fp.baseType})
	}
	m := t.model
	if len(t.pk) > 0 {
		methods = append(methods, CRUDMethod{Name: m + "Get", Op: "get", Key: key, Result: m})
	}
	methods = append(methods, CRUDMethod{Name: m + "List", Op: "list", Result: m, Many: true})
	methods = append(methods, CRUDMethod{Name: m + "Create", Op: "create", Body: m + "Changeset", Params: m + "Params",
		New: m + "New", Allow: t.handles(t.settable()), Result: m})
	if len(t.pk) > 0 {
		if len(t.nonPK()) > 0 {
			methods = append(methods, CRUDMethod{Name: m + "Update", Op: "update", Key: key, Body: m + "Changeset", Params: m + "Params",
				Get: m + "Get", Allow: t.handles(t.nonPK()), Result: m})
		}
		methods = append(methods, CRUDMethod{Name: m + "Delete", Op: "delete", Key: key})
	}
	return methods
}

// ModelFields reports the exact struct fields the model generator emits
// for the table with the given canonical key (spec §selects-over-groups, §mapping-to-go).
func (pl *Plan) ModelFields(tableKey string) (model string, fields []FieldSig, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return "", nil, err
	}
	return t.model, t.sigs, nil
}

func fieldSigsBuild(t *tableModel) []FieldSig {
	sigs := make([]FieldSig, 0, len(t.fields))
	for _, fp := range t.fields {
		sigs = append(sigs, fieldSigOf(fp))
	}
	return sigs
}

func fieldSigOf(fp *fieldPlan) FieldSig {
	return FieldSig{
		Name: fp.goField, Col: fp.colName, Type: fp.goType,
		Tag: fp.tag, Doc: settingNote(fp.col.Settings),
		Nullable: fp.nullable,
	}
}

// SelectRowType names the row type one instantiation returns (§selects-over-groups)
// and reports its fields.
func (pl *Plan) SelectRowType(fn SelectFn) (string, []FieldSig, error) {
	t, err := pl.table(fn.TableKey)
	if err != nil {
		return "", nil, err
	}
	cols, err := selectColumns(t, fn)
	if err != nil {
		return "", nil, err
	}
	row := t.model
	switch {
	case fn.SharedType != "":
		row = fn.SharedType
	case len(fn.Excluded) > 0:
		row = t.model + fn.MethodSuffix
	}
	sigs := make([]FieldSig, 0, len(cols))
	for _, fp := range cols {
		sigs = append(sigs, fieldSigOf(fp))
	}
	return row, sigs, nil
}

// SelectStatement renders the SQL one select instantiation runs.
func (pl *Plan) SelectStatement(fn SelectFn) (string, error) {
	t, err := pl.table(fn.TableKey)
	if err != nil {
		return "", err
	}
	return SelectSQL(t, fn), nil
}

// DynNameCollisions reports the package-scope Go name collisions the
// generated files would produce; see the package-level function.
func (pl *Plan) DynNameCollisions() []NameCollision {
	if pl.err != nil {
		return nil
	}
	seen := map[string]dynOrigin{"Queries": {kind: dynQueries}}
	var out []NameCollision
	for _, e := range pl.info.Enums {
		typeName, err := enumTypeName(e.Decl.Name.Schema(), e.Decl.Name.Base())
		if err != nil {
			continue
		}
		out = dynArrive(out, seen, typeName, dynOrigin{kind: dynEnum, e: e, pos: e.Decl.Pos()})
		for _, v := range e.Decl.Values {
			constName, err := goName(v.Name.Name())
			if err != nil {
				continue
			}
			out = dynArrive(out, seen, typeName+constName, dynOrigin{kind: dynEnumValue, e: e, v: v, pos: v.Pos()})
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
	return NameCollision{Name: name, First: first.describe(), Second: second.describe(), Pos: second.pos}
}

func dynOriginOf(o *nameOrigin) dynOrigin {
	d := dynOrigin{tm: o.table, pos: o.table.ti.Decl.Pos()}
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
		d.kind, d.f, d.pos = dynHandle, o.field, o.field.col.Pos()
	case nameDynFunc:
		d.kind, d.sfx = dynWrapper, o.name[len(o.table.model):]
	}
	return d
}

// Names is the scope of package-level Go identifiers a package's
// generated files mint (§selects-over-groups): models, changesets with
// their column and params aliases and constructors (§changesets), column
// handles and option wrappers, enum types, the Queries handle and its
// constructor. It is read in layers, last writer first.
type Names struct {
	base  *nameBase
	enums map[string]*ast.Enum
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
	table *tableModel
	field *fieldPlan
	dyn   bool
	rank  int
}

func modelNamesBuild(t *tableModel) []nameOrigin {
	names := make([]nameOrigin, 0, 5+len(dynWrapperSuffixes)+len(t.fields))
	add := func(name string, kind nameKind, field *fieldPlan, dyn bool, rank int) {
		names = append(names, nameOrigin{name: name, kind: kind, idx: len(names), table: t, field: field, dyn: dyn, rank: rank})
	}
	queryable := len(t.fields) > 0
	add(t.model, nameModel, nil, true, 0)
	add(t.model+"Changeset", nameChangeset, nil, queryable, 1)
	add(t.model+"Column", nameColumn, nil, queryable, 2)
	add(t.model+"Params", nameParams, nil, queryable, 3)
	add(t.model+"New", nameNew, nil, queryable, 4)
	for i, suffix := range dynWrapperSuffixes {
		add(t.model+suffix, nameDynFunc, nil, queryable, 5+len(t.fields)+i)
	}
	for i, fp := range t.fields {
		add(t.model+fp.goField, nameHandle, fp, queryable, 5+i)
	}
	return names
}

func (o *nameOrigin) describe() string {
	switch o.kind {
	case nameModel:
		return fmt.Sprintf("the model of table %q", o.table.ti.Decl.Name.Base())
	case nameChangeset:
		return fmt.Sprintf("the changeset of table %q", o.table.ti.Decl.Name.Base())
	case nameColumn:
		return fmt.Sprintf("the column handle type of table %q", o.table.ti.Decl.Name.Base())
	case nameParams:
		return fmt.Sprintf("the write body type of table %q", o.table.ti.Decl.Name.Base())
	case nameNew:
		return fmt.Sprintf("the changeset constructor of table %q", o.table.ti.Decl.Name.Base())
	case nameHandle:
		return fmt.Sprintf("the column handle for %s.%s", o.table.ti.Decl.Name.Base(), o.field.colName)
	}
	return "the dynamic-layer function " + o.name
}

type nameBase struct {
	origins map[string]nameSlot
	models  map[*tableModel]bool
	pos     map[*tableModel]int
	dups    map[string]bool
}

type nameSlot struct {
	first *nameOrigin
	more  []*nameOrigin
}

func newNameBase() *nameBase {
	return &nameBase{origins: map[string]nameSlot{}, models: map[*tableModel]bool{}, dups: map[string]bool{}}
}

func (b *nameBase) update(tables []*tableModel) {
	current := make(map[*tableModel]bool, len(tables))
	b.pos = make(map[*tableModel]int, len(tables))
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

func (b *nameBase) add(t *tableModel) {
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

func (b *nameBase) remove(t *tableModel) {
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
	n := &Names{base: pl.base, enums: map[string]*ast.Enum{}, added: map[string]string{}}
	if pl.err != nil {
		return n
	}
	for _, d := range pl.f.Decls {
		if e, ok := d.(*ast.Enum); ok {
			if typ, err := enumTypeName(e.Name.Schema(), e.Name.Base()); err == nil {
				n.enums[typ] = e
			}
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
	if e, ok := n.enums[name]; ok {
		return fmt.Sprintf("the enum %q", e.Name.String()), true
	}
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
