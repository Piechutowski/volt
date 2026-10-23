// Shape rendering for migrations (spec §migrations): a create table,
// drop table or rebuild table body and an added column lower by the
// same mapping as a schema Table (§mapping-to-sqlite-ddl), with plain
// names and the inline ref: settings as the foreign keys.
package sqlite

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
)

// TypeSQL is the SQLite column type of a built-in column type (spec
// §mapping-to-sqlite-ddl), false for any other name.
func TypeSQL(builtin string) (string, bool) {
	t, ok := sqliteType[builtin]
	return t, ok
}

// TypeVolt is the built-in column type a SQLite column type reads back
// as (spec §mapping-to-sqlite-ddl): INTEGER is int, since a bool is
// stored as one. An unmapped type reads back as "".
func TypeVolt(sqlType string) string {
	switch strings.ToUpper(sqlType) {
	case "INTEGER":
		return "int"
	case "REAL":
		return "float64"
	case "TEXT":
		return "string"
	case "TIMESTAMP":
		return "time.Time"
	case "BLOB":
		return "[]byte"
	}
	return ""
}

// DefaultSQL renders a default: value as SQLite takes it (spec
// §mapping-to-sqlite-ddl): a backtick expression parenthesized, a
// string quoted, a boolean as 1 or 0.
func DefaultSQL(v ast.Node) (string, error) { return defaultSQL(v) }

// TableDDL renders the CREATE TABLE statement of a shape body, without
// the trailing semicolon (spec §migrations): the
// columns, the primary key of its pk columns in order, and a foreign
// key per inline ref: with its delete: and update: actions
// (§relationships-ref), as the schema's Table decides them.
func TableDDL(table string, body []ast.TableItem) (string, error) {
	var pkCols []string
	for _, item := range body {
		if col, ok := item.(*ast.Column); ok && col.Settings.Get("pk") != nil {
			pkCols = append(pkCols, col.Name.Name())
		}
	}
	var lines, constraints []string
	for _, item := range body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		name := col.Name.Name()
		sqlType, ok := TypeSQL(col.Type.String())
		if !ok {
			return "", fmt.Errorf("table %s: column %q: type %q is not a built-in (§migrations)", table, name, col.Type.String())
		}
		line, err := columnLine(col, sqlType, pkCols)
		if err != nil {
			return "", fmt.Errorf("table %s: %w", table, err)
		}
		lines = append(lines, line)
		if ep := refEndpoint(col); ep != nil {
			constraints = append(constraints, "  FOREIGN KEY ("+identQuote(name)+") REFERENCES "+referenceSQL(col, ep))
		}
	}
	if len(pkCols) > 1 {
		lines = append(lines, "  PRIMARY KEY ("+quotedJoin(pkCols)+")")
	}
	lines = append(lines, constraints...)
	return "CREATE TABLE " + identQuote(table) + " (\n" + strings.Join(lines, ",\n") + "\n)", nil
}

// IndexesDDL renders the CREATE INDEX statements of a shape body,
// without trailing semicolons, named for table (spec §migrations):
// the index of every column that carries unique or
// index, then the indexes of its Indexes block, as the schema's Table
// lowers them.
func IndexesDDL(table string, body []ast.TableItem) ([]string, error) {
	var cols []*ast.Column
	var indexes []*ast.Index
	for _, item := range body {
		switch item := item.(type) {
		case *ast.Column:
			cols = append(cols, item)
		case *ast.IndexesBlock:
			indexes = append(indexes, item.Indexes...)
		}
	}
	return indexStatements(table, cols, indexes, map[string]string{})
}

// ColumnDDL renders the definition of a column added by ALTER TABLE
// ADD COLUMN (spec §migrations): its type, NOT NULL, DEFAULT
// and REFERENCES with its delete: and update: actions. A unique or
// index setting is the column's CREATE INDEX, which the caller writes
// after the ADD COLUMN, since SQLite takes no UNIQUE here.
func ColumnDDL(col *ast.Column) (string, error) {
	name := col.Name.Name()
	sqlType, ok := TypeSQL(col.Type.String())
	if !ok {
		return "", fmt.Errorf("column %q: type %q is not a built-in (§migrations)", name, col.Type.String())
	}
	parts := []string{identQuote(name), sqlType}
	if !isNullable(col) {
		parts = append(parts, "NOT NULL")
	}
	if s := col.Settings.Get("default"); s != nil {
		d, err := defaultSQL(s.Value)
		if err != nil {
			return "", fmt.Errorf("column %q: %w", name, err)
		}
		parts = append(parts, "DEFAULT "+d)
	}
	if ep := refEndpoint(col); ep != nil {
		parts = append(parts, "REFERENCES "+referenceSQL(col, ep))
	}
	return strings.Join(parts, " "), nil
}

// refEndpoint is the endpoint of a column's ref: setting, nil without
// one (spec §relationships-ref).
func refEndpoint(col *ast.Column) *ast.RefEndpoint {
	if s := col.Settings.Get("ref"); s != nil {
		ep, _ := s.Value.(*ast.RefEndpoint)
		return ep
	}
	return nil
}

// referenceSQL renders the referenced table and columns of a ref: with
// the ON DELETE and ON UPDATE clauses of the column's delete: and
// update: settings (spec §migrations, §relationships-ref).
func referenceSQL(col *ast.Column, ep *ast.RefEndpoint) string {
	out := identQuote(ep.Table.Name()) + " (" + quotedJoin(identNames(ep.Columns)) + ")"
	if a := actionSQL(col.Settings, "delete"); a != "" {
		out += " ON DELETE " + a
	}
	if a := actionSQL(col.Settings, "update"); a != "" {
		out += " ON UPDATE " + a
	}
	return out
}

func identNames(ids []*ast.Ident) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Name()
	}
	return out
}
