// Exported naming and typing helpers for the Volt language layer (lang
// package): the router generator must agree byte-for-byte with the model
// generator on names and Go types, so there is exactly one implementation.
package golang

import (
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
	Name string   // the check's [name:] setting, "" when absent
	Src  string   // human-readable form, reported by rt.CheckError
	Cond string   // typed form: Go condition over v.<Field>; "" otherwise
	Call string   // Go-reference call, e.g. "EmailValid(v.Email)"; "" otherwise
	Cols []string // the columns the check reads, so a params struct knows whether it can evaluate it (§V12.6)
}

// ParamsValidators reports whether the table's CreateParams and
// UpdateParams structs get a Validate method: each does when the struct
// exists and carries every column of at least one check. The same rule
// decides what the validator generator emits and what a generated
// handler calls, so the two cannot disagree.
//
// One-shot convenience over Plan.ParamsValidators: it plans the whole
// file. Callers with a Package ask its Plan instead (D74).
func ParamsValidators(f *ast.File, info *check.Info, tableKey string, checks []CheckSpec) (create, update bool, err error) {
	return PlanBuild(f, info).ParamsValidators(tableKey, checks)
}

// paramsChecks splits the checks a table's params structs can evaluate:
// those whose columns all appear among the struct's fields. A check
// reading a defaulted or auto-increment column (absent from
// CreateParams, D16) or a key column (absent from UpdateParams) stays
// with the row's Validate and the DDL.
func paramsChecks(t *tableModel, checks []CheckSpec) (create, update []CheckSpec) {
	covered := func(fields []*fieldPlan, ck CheckSpec) bool {
		have := map[string]bool{}
		for _, f := range fields {
			have[f.colName] = true
		}
		for _, col := range ck.Cols {
			if !have[col] {
				return false
			}
		}
		return len(ck.Cols) > 0
	}
	cf := t.createFields()
	uf := t.nonPK()
	for _, ck := range checks {
		if len(cf) > 0 && covered(cf, ck) {
			create = append(create, ck)
		}
		if len(t.pk) > 0 && len(uf) > 0 && covered(uf, ck) {
			update = append(update, ck)
		}
	}
	return create, update
}

// EnumTypeName is the Go type name an enum declaration generates
// (Appendix A.3), for tooling and row-type collision checks.
func EnumTypeName(schema, base string) (string, error) { return enumTypeName(schema, base) }

// ModelFields reports the exact struct fields the model generator emits
// for the table with the given canonical key — hover-grade truth, from
// the same plan the generator runs (spec §V11.6, Appendix A).
//
// One-shot convenience over Plan.ModelFields: it plans the whole file.
func ModelFields(f *ast.File, info *check.Info, tableKey string) (model string, fields []FieldSig, err error) {
	return PlanBuild(f, info).ModelFields(tableKey)
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
//
// One-shot convenience over Plan.CRUDMethods: it plans the whole file.
func CRUDMethods(f *ast.File, info *check.Info, tableKey string) (model string, methods []CRUDMethod, err error) {
	return PlanBuild(f, info).CRUDMethods(tableKey)
}
