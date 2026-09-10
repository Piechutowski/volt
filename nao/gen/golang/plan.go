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

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
)

// Plan is a checked package's naming plan: every Go and SQL name of
// every table and column, decided once (D09, D10, D13, D15-D17), plus
// the generated CRUD method table indexed by method name. A Plan is
// immutable after PlanBuild and safe to share between goroutines.
type Plan struct {
	f    *ast.File
	info *check.Info
	p    *plan // nil when err != nil
	err  error

	byKey   map[string]*tableModel
	methods map[string]crudRef // generated CRUD method name -> owner; the first table declared wins
	names   []string           // every CRUD method name in declaration order, for did-you-mean hints
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
	pl := &Plan{f: f, info: info}
	p, err := planBuild(f, info)
	if err != nil {
		pl.err = err
		return pl
	}
	pl.p = p
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

func crudMethodsOf(t *tableModel) []CRUDMethod {
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
	fields = make([]FieldSig, 0, len(t.fields))
	for _, fp := range t.fields {
		fields = append(fields, fieldSigOf(fp))
	}
	return t.model, fields, nil
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
	return dynNamesCheck(pl.p, pl.info)
}

/* ===== the §V11.7 name scope ===== */

// Names is the scope of package-level Go identifiers a package's
// generated files mint (§V11.7): models, params structs, dynamic column
// handles and option wrappers, enum types, the Queries handle and its
// constructor — the scope a minted row type must not collide with. A
// description is rendered on lookup, since nearly every name is never
// asked about; Add records a name the checker mints itself.
type Names struct {
	origins map[string]nameOrigin
}

type nameKind uint8

const (
	nameLiteral      nameKind = iota // desc holds the description verbatim
	nameModel                        // the model of table
	nameCreateParams                 // the create params of table
	nameUpdateParams                 // the update params of table
	nameDynFunc                      // a dynamic-layer function, named by the key itself
	nameHandle                       // a dynamic column handle: table + field
	nameEnum                         // an enum type
)

type nameOrigin struct {
	kind  nameKind
	desc  string
	table *tableModel
	field *fieldPlan
	enum  *ast.Enum
}

// Names returns a fresh name scope for the package: the two fixed names
// alone when the plan could not be built, so the checker's collision
// rule still holds for what is certain.
func (pl *Plan) Names() *Names {
	n := &Names{origins: map[string]nameOrigin{}}
	n.Add("Queries", "the generated Queries handle")
	n.Add("New", "the generated constructor")
	if pl.err != nil {
		return n
	}
	size := 2
	for _, t := range pl.p.tables {
		size += 3 + len(dynWrapperSuffixes) + len(t.fields)
	}
	n.origins = make(map[string]nameOrigin, size)
	n.Add("Queries", "the generated Queries handle")
	n.Add("New", "the generated constructor")
	for _, t := range pl.p.tables {
		n.origins[t.model] = nameOrigin{kind: nameModel, table: t}
		n.origins[t.model+"CreateParams"] = nameOrigin{kind: nameCreateParams, table: t}
		n.origins[t.model+"UpdateParams"] = nameOrigin{kind: nameUpdateParams, table: t}
		for _, suffix := range dynWrapperSuffixes {
			n.origins[t.model+suffix] = nameOrigin{kind: nameDynFunc}
		}
		for _, fp := range t.fields {
			n.origins[t.model+fp.goField] = nameOrigin{kind: nameHandle, table: t, field: fp}
		}
	}
	for _, d := range pl.f.Decls {
		if e, ok := d.(*ast.Enum); ok {
			if typ, err := enumTypeName(e.Name.Schema(), e.Name.Base()); err == nil {
				n.origins[typ] = nameOrigin{kind: nameEnum, enum: e}
			}
		}
	}
	return n
}

// Add records a minted name with its description; a later Add of the
// same name replaces the earlier one.
func (n *Names) Add(name, desc string) {
	n.origins[name] = nameOrigin{kind: nameLiteral, desc: desc}
}

// Lookup reports whether name is taken and, if so, by what.
func (n *Names) Lookup(name string) (desc string, ok bool) {
	o, ok := n.origins[name]
	if !ok {
		return "", false
	}
	switch o.kind {
	case nameModel:
		return fmt.Sprintf("the model of table %q", o.table.ti.Decl.Name.Base()), true
	case nameCreateParams:
		return fmt.Sprintf("the create params of table %q", o.table.ti.Decl.Name.Base()), true
	case nameUpdateParams:
		return fmt.Sprintf("the update params of table %q", o.table.ti.Decl.Name.Base()), true
	case nameDynFunc:
		return "the dynamic-layer function " + name, true
	case nameHandle:
		return fmt.Sprintf("the dynamic column handle for %s.%s", o.table.ti.Decl.Name.Base(), o.field.colName), true
	case nameEnum:
		return fmt.Sprintf("the enum %q", o.enum.Name.String()), true
	}
	return o.desc, true
}
