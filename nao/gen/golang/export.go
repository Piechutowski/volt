// Exported naming and typing helpers for the Volt language layer (lang
// package): the router generator must agree byte-for-byte with the model
// generator on names and Go types, so there is exactly one implementation.
package golang

import "github.com/Piechutowski/volt/lang/ast"

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
// member table, with WHERE/ORDER fragments rendered by the checker.
type SelectFn struct {
	TableKey     string // canonical schema.name key into check.Info
	MethodSuffix string
	WhereSQL     string
	OrderSQL     string
	Params       []SelectParam
}
