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
