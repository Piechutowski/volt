// The shared naming plan (decision D74): one immutable Plan per checked
// package, built once after the schema pass and read by the Volt
// checker, the generators, vet and the language server. Every question
// about a generated name — what a table's CRUD methods are called, what
// fields a model carries, which identifiers a package mints — is
// answered from here instead of being re-derived from the AST on every
// call, which is what made checking cubic in tables per package.
package golang

import (
	"fmt"
	"sort"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
)

// Plan is a checked package's naming plan: every Go and SQL name of
// every table and column, decided once (D09, D10, D13, D15-D17), plus
// the generated CRUD method table indexed by method name. A Plan is
// immutable after PlanBuild and safe to share between goroutines.
type Plan struct {
	memo *PlanMemo // nil outside the editor session
	f    *ast.File
	info *check.Info
	p    *plan // nil when err != nil
	err  error

	byKey   map[string]*tableModel
	methods map[string]crudRef // generated CRUD method name -> owner; the first table declared wins
	names   []string           // every CRUD method name in declaration order, for did-you-mean hints
	base    *nameBase          // the models' names (§V11.7), the memo's when there is one
}

// crudRef locates one generated CRUD method.
type crudRef struct {
	tableKey string
	method   CRUDMethod
}

// PlanBuild plans one checked file (a package's merged declarations).
// A plan that cannot be built — an unmappable column type, a Go name
// collision — still answers: Err reports the reason and every lookup
// fails with it, so callers report the generation error where they
// always did.
func PlanBuild(f *ast.File, info *check.Info) *Plan {
	return PlanBuildMemo(f, info, nil)
}

// PlanMemo remembers a package's table models across builds of its
// plan: a table that is the checked object it was, under the same
// enum types, is the same model (D84). The zero value is ready; a
// PlanMemo belongs to one package.
type PlanMemo struct {
	prev, next map[*check.TableInfo]*memoModel
	base       *nameBase // the models' names, updated for the models that changed (D99)
	// Hits and Misses count the models reused and built by the last
	// PlanBuildMemo.
	Hits, Misses int
}

type memoModel struct {
	tm      *tableModel
	imports map[string]bool
	enums   []enumType // the enum types the model was built with, sorted by key (D91)
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
// memo downstream can key on it (D84). nil when the plan has no such
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
// takes a params struct only when a column is caller-supplied (D16).
func (pl *Plan) CRUDMethods(tableKey string) (model string, methods []CRUDMethod, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return "", nil, err
	}
	return t.model, crudMethodsOf(t), nil
}

// CRUDMethod finds the generated CRUD method with the given name; when
// two tables would mint it, the table declared first owns it, as the
// query generator's declaration-order emission does.
func (pl *Plan) CRUDMethod(name string) (tableKey string, m CRUDMethod, ok bool) {
	ref, ok := pl.methods[name]
	return ref.tableKey, ref.method, ok
}

// CRUDMethodNames lists every generated CRUD method name in declaration
// order: the candidates a did-you-mean hint is drawn from.
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
	create := CRUDMethod{Name: m + "Create", Op: "create", Result: m}
	if len(t.createFields()) > 0 {
		create.Body = m + "CreateParams"
	}
	methods = append(methods, create)
	if len(t.pk) > 0 {
		if len(t.nonPK()) > 0 {
			methods = append(methods, CRUDMethod{Name: m + "Update", Op: "update", Key: key, Body: m + "UpdateParams", Result: m})
		}
		methods = append(methods, CRUDMethod{Name: m + "Delete", Op: "delete", Key: key})
	}
	return methods
}

// ParamsValidators reports whether the table's CreateParams and
// UpdateParams structs get a Validate method: each does when the struct
// exists and carries every column of at least one check. The same rule
// decides what the validator generator emits and what a generated
// handler calls, so the two cannot disagree.
func (pl *Plan) ParamsValidators(tableKey string, checks []CheckSpec) (create, update bool, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return false, false, err
	}
	c, u := paramsChecks(t, checks)
	return len(c) > 0, len(u) > 0, nil
}

// ModelFields reports the exact struct fields the model generator emits
// for the table with the given canonical key — hover-grade truth, from
// the same plan the generator runs (spec §V11.6, Appendix A).
func (pl *Plan) ModelFields(tableKey string) (model string, fields []FieldSig, err error) {
	t, err := pl.table(tableKey)
	if err != nil {
		return "", nil, err
	}
	return t.model, t.sigs, nil
}

// fieldSigsBuild is the model's field signatures, built with it (D99).
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

// SelectRowType names the row type one instantiation returns (§V11.7)
// and reports its fields — hover-grade truth for the tooling.
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
	// The report is the one a walk over every minted name in minting
	// order produces — the fixed Queries name, the enums' types and
	// constants, then each model's names in declaration order — pairing
	// a name minted again with the origin that minted it first, at the
	// later of the two by position. It is computed from the collisions
	// alone: the enums' names are few, and the names two or more
	// models mint the base keeps across plans (D99, D104).
	seen := map[string]dynOrigin{"Queries": {kind: dynQueries}}
	var out []NameCollision
	for _, e := range pl.info.Enums {
		typeName, err := enumTypeName(e.Decl.Name.Schema(), e.Decl.Name.Base())
		if err != nil {
			continue // generation reports unusable names itself
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
	// The models' arrivals that collide: with a name the fixed name or
	// an enum minted first, which stays the first origin of every later
	// arrival, or with an earlier model's.
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
	// Each origin mints one name, so (pos, rank) orders the arrivals
	// totally: the walk's order, whatever order the maps gave.
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

// dynArrive records one minted name: the first arrival is remembered,
// a later one is paired with it.
func dynArrive(out []NameCollision, seen map[string]dynOrigin, name string, o dynOrigin) []NameCollision {
	prev, dup := seen[name]
	if !dup {
		seen[name] = o
		return out
	}
	return append(out, dynPair(name, prev, o))
}

// dynPair reports one collision at the later of its two origins.
func dynPair(name string, first, second dynOrigin) NameCollision {
	if second.pos.Line() < first.pos.Line() || (second.pos.Line() == first.pos.Line() && second.pos.Column() < first.pos.Column()) {
		first, second = second, first
	}
	return NameCollision{Name: name, First: first.describe(), Second: second.describe(), Pos: second.pos}
}

// dynOriginOf is a model's minted name as the collision report
// describes it.
func dynOriginOf(o *nameOrigin) dynOrigin {
	d := dynOrigin{tm: o.table, pos: o.table.ti.Decl.Pos()}
	switch o.kind {
	case nameModel:
		d.kind = dynModel
	case nameCreateParams:
		d.kind = dynCreateParams
	case nameUpdateParams:
		d.kind = dynUpdateParams
	case nameHandle:
		d.kind, d.f, d.pos = dynHandle, o.field, o.field.col.Pos()
	case nameDynFunc:
		d.kind, d.sfx = dynWrapper, o.name[len(o.table.model):]
	}
	return d
}

/* ===== the §V11.7 name scope ===== */

// Names is the scope of package-level Go identifiers a package's
// generated files mint (§V11.7): models, params structs, dynamic column
// handles and option wrappers, enum types, the Queries handle and its
// constructor — the scope a minted row type must not collide with. It
// is read in layers, last writer first, as the one map it replaced was
// written (D99): the names the checker adds during a check, the enums'
// and the two fixed names of this plan, and the models' names, which
// the plan memo keeps across plans and updates for the models that
// changed. A description is rendered on lookup, since nearly every
// name is never asked about.
type Names struct {
	base  *nameBase            // the models' names: read here, written by the plan build
	enums map[string]*ast.Enum // enum type name -> declaration
	added map[string]string    // what the checker minted, with its description
}

type nameKind uint8

const (
	nameModel        nameKind = iota // the model of table
	nameCreateParams                 // the create params of table
	nameUpdateParams                 // the update params of table
	nameDynFunc                      // a dynamic-layer function, named by the key itself
	nameHandle                       // a dynamic column handle: table + field
)

// nameOrigin is one name a model mints and what it is: built with the
// model (tableModel.names), in the order the names are minted, so the
// scope's base can hold pointers into the models' own lists.
type nameOrigin struct {
	name  string
	kind  nameKind
	idx   int // position in the model's list: a later name of the same model wins
	table *tableModel
	field *fieldPlan
	// The dynamic layer mints a model's names under conditions of its
	// own (D30): dyn says whether it mints this one, rank where the
	// name falls in the model's minting order, which the collision
	// report follows (DynNameCollisions).
	dyn  bool
	rank int
}

// modelNamesBuild is every package-level name a model mints, in the
// order the generators write them.
func modelNamesBuild(t *tableModel) []nameOrigin {
	names := make([]nameOrigin, 0, 3+len(dynWrapperSuffixes)+len(t.fields))
	add := func(name string, kind nameKind, field *fieldPlan, dyn bool, rank int) {
		names = append(names, nameOrigin{name: name, kind: kind, idx: len(names), table: t, field: field, dyn: dyn, rank: rank})
	}
	// no queryable shape: no queries, no dynamic layer
	queryable := len(t.fields) > 0
	add(t.model, nameModel, nil, true, 0)
	add(t.model+"CreateParams", nameCreateParams, nil, queryable && len(t.createFields()) > 0, 1)
	add(t.model+"UpdateParams", nameUpdateParams, nil, queryable && len(t.pk) > 0 && len(t.nonPK()) > 0, 2)
	for i, suffix := range dynWrapperSuffixes {
		add(t.model+suffix, nameDynFunc, nil, queryable, 3+len(t.fields)+i)
	}
	for i, fp := range t.fields {
		add(t.model+fp.goField, nameHandle, fp, queryable, 3+i)
	}
	return names
}

func (o *nameOrigin) describe() string {
	switch o.kind {
	case nameModel:
		return fmt.Sprintf("the model of table %q", o.table.ti.Decl.Name.Base())
	case nameCreateParams:
		return fmt.Sprintf("the create params of table %q", o.table.ti.Decl.Name.Base())
	case nameUpdateParams:
		return fmt.Sprintf("the update params of table %q", o.table.ti.Decl.Name.Base())
	case nameHandle:
		return fmt.Sprintf("the dynamic column handle for %s.%s", o.table.ti.Decl.Name.Base(), o.field.colName)
	}
	return "the dynamic-layer function " + o.name
}

// nameBase is the models' names of one package, kept across plans:
// each name's origins in the models that mint it, as pointers into the
// models' own lists, and the models present, so a plan with one model
// changed touches that model's names alone (D99).
type nameBase struct {
	origins map[string]nameSlot
	models  map[*tableModel]bool
	pos     map[*tableModel]int // declaration position in the current plan
	dups    map[string]bool     // names two or more models mint for the dynamic layer
}

// nameSlot is a name's origins: the first held by value, since nearly
// every name has one, the rest in a list.
type nameSlot struct {
	first *nameOrigin
	more  []*nameOrigin
}

func newNameBase() *nameBase {
	return &nameBase{origins: map[string]nameSlot{}, models: map[*tableModel]bool{}, dups: map[string]bool{}}
}

// update makes the base the given models': the models gone are
// removed, the models new are added, the rest stand.
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

// dupsMark keeps dups current for one name after its slot changed.
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

// all lists a slot's origins, the first first; nil for a name nobody
// mints.
func (s nameSlot) all() []*nameOrigin {
	if s.first == nil {
		return nil
	}
	return append([]*nameOrigin{s.first}, s.more...)
}

// lookup answers the origin the one-map build would have kept for the
// name: the model declared last, and its later name.
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
// names alone when the plan could not be built, so the checker's
// collision rule still holds for what is certain.
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
