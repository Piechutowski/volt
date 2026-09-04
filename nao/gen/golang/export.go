// Exported naming and typing helpers for the Volt language layer (lang
// package): the router generator must agree byte-for-byte with the model
// generator on names and Go types, so there is exactly one implementation.
package golang

import (
	"fmt"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
)

// ModelName derives the Go model type name for a table (decision D10):
// the [model:] override when present, else the singularized table name.
func ModelName(t *ast.Table) (string, error) { return modelName(t) }

// GoName converts a DBML name to an exported Go identifier.
func GoName(dbmlName string) (string, error) { return goName(dbmlName) }

// GoTypeName maps a DBML column type (arguments stripped, lowercased) to
// its Go type name, reporting whether the type is known.
func GoTypeName(dbmlType string) (string, bool) {
	t, ok := typeMap[dbmlType]
	if !ok {
		return "", false
	}
	return t.name, true
}

// SelectParam is one Go parameter of a generated select method
// (spec §V11.6), in declaration order.
type SelectParam struct {
	SQLName string // name bound as :name in the statement
	GoName  string
	GoType  string // e.g. "string", "int64", "time.Time"
}

// SelectFn is one select instantiation: a method on Queries for one
// member table, with WHERE/ORDER fragments rendered by the checker and
// the §V11.7 projection already validated.
type SelectFn struct {
	TableKey     string // canonical schema.name key into check.Info
	MethodSuffix string
	WhereSQL     string
	OrderSQL     string
	Params       []SelectParam
	Cols         []string // explicit projection, declared order; nil = all
	Excluded     []string // star-form exclusions; nil = none
	SharedType   string   // explicit list: the one shared row type name
}

// FieldSig is one generated struct field — hover- and derivative-grade
// truth, from the same plan the generator runs.
type FieldSig struct {
	Name     string // Go field name, e.g. "EditorID"
	Col      string // source column name, e.g. "editor_id"
	Type     string // full Go type, e.g. "rt.Null[int64]"
	Tag      string // assembled struct tag (Appendix A.5)
	Doc      string // column note, "" when absent
	Nullable bool   // NULL-admitting column (Appendix A.2)
}

// CheckFn is one table's validator surface (spec §V12): the checks a
// generated Validate method evaluates.
type CheckFn struct {
	TableKey string // canonical schema.name key into check.Info
	Checks   []CheckSpec
}

// CheckSpec is one lowered check, rendered by the Volt checker.
type CheckSpec struct {
	Name string // the check's [name:] setting, "" when absent
	Src  string // human-readable form, reported by rt.CheckError
	Cond string // typed form: Go condition over v.<Field>; "" otherwise
	Call string // Go-reference call, e.g. "EmailValid(v.Email)"; "" otherwise
}

// EnumTypeName is the Go type name an enum declaration generates
// (Appendix A.3), for tooling and row-type collision checks.
func EnumTypeName(schema, base string) (string, error) { return enumTypeName(schema, base) }

// ModelFields reports the exact struct fields the model generator emits
// for the table with the given canonical key — hover-grade truth, from
// the same plan the generator runs (spec §V11.6, Appendix A).
func ModelFields(f *ast.File, info *check.Info, tableKey string) (model string, fields []FieldSig, err error) {
	p, err := planBuild(f, info)
	if err != nil {
		return "", nil, err
	}
	for _, t := range p.tables {
		if t.ti.Key != tableKey {
			continue
		}
		for _, fp := range t.fields {
			fields = append(fields, FieldSig{
				Name: fp.goField, Col: fp.colName, Type: fp.goType,
				Tag: fp.tag, Doc: settingNote(fp.col.Settings),
				Nullable: fp.nullable,
			})
		}
		return t.model, fields, nil
	}
	return "", nil, fmt.Errorf("no table %q", tableKey)
}

// PackageNames lists every package-level Go identifier the model, query
// and dynamic generators emit for a schema, each with a description —
// the scope a minted row type (§V11.7) must not collide with.
func PackageNames(f *ast.File, info *check.Info) (map[string]string, error) {
	p, err := planBuild(f, info)
	if err != nil {
		return nil, err
	}
	names := map[string]string{"Queries": "the generated Queries handle", "New": "the generated constructor"}
	for _, t := range p.tables {
		tbl := t.ti.Decl.Name.Base()
		names[t.model] = fmt.Sprintf("the model of table %q", tbl)
		names[t.model+"CreateParams"] = fmt.Sprintf("the create params of table %q", tbl)
		names[t.model+"UpdateParams"] = fmt.Sprintf("the update params of table %q", tbl)
		for _, suffix := range []string{"Limit", "Offset", "Distinct", "OrderBy", "After", "Set"} {
			names[t.model+suffix] = fmt.Sprintf("the dynamic-layer function %s%s", t.model, suffix)
		}
		for _, fp := range t.fields {
			names[t.model+fp.goField] = fmt.Sprintf("the dynamic column handle for %s.%s", tbl, fp.colName)
		}
	}
	for _, d := range f.Decls {
		if e, ok := d.(*ast.Enum); ok {
			if n, err := enumTypeName(e.Name.Schema(), e.Name.Base()); err == nil {
				names[n] = fmt.Sprintf("the enum %q", e.Name.String())
			}
		}
	}
	return names, nil
}

// CRUDMethod is one generated default-CRUD method of a table (CRUD-1 to
// CRUD-7), as a query route (spec §V4.8) sees it: the name to call, the
// identity parameters in key order, the params struct it decodes, and
// what comes back.
type CRUDMethod struct {
	Name   string        // method name, e.g. "UserGet"
	Op     string        // get | list | create | update | delete
	Key    []SelectParam // identity parameters (get, update, delete); nil otherwise
	Body   string        // params struct type (create, update); "" when the method takes none
	Result string        // row type; "" for delete
	Many   bool          // slice result (list)
}

// CRUDMethods lists the default CRUD methods the query generator emits
// for one table, with the same existence rules it applies: Get, Update
// and Delete need a primary key, Update needs a non-key column, Create
// takes a params struct only when a column is caller-supplied (D16).
func CRUDMethods(f *ast.File, info *check.Info, tableKey string) (model string, methods []CRUDMethod, err error) {
	p, err := planBuild(f, info)
	if err != nil {
		return "", nil, err
	}
	for _, t := range p.tables {
		if t.ti.Key != tableKey {
			continue
		}
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
		return t.model, methods, nil
	}
	return "", nil, fmt.Errorf("no table %q", tableKey)
}
