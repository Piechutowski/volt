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
	Name string // Go field name, e.g. "EditorID"
	Col  string // source column name, e.g. "editor_id"
	Type string // full Go type, e.g. "rt.Null[int64]"
	Tag  string // assembled struct tag (Appendix A.5)
	Doc  string // column note, "" when absent
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
			})
		}
		return t.model, fields, nil
	}
	return "", nil, fmt.Errorf("no table %q", tableKey)
}
