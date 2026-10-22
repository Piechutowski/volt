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
// the trailing semicolon (spec §migrations rules 5, 6 and 9): the
// columns, a composite primary key, and a foreign key per inline ref:
// with its delete: and update: actions (§relationships-ref), as
// the schema's Table decides them.
func TableDDL(table string, body []ast.TableItem) (string, error) {
	pkCols := pkKeyColumns(body)
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
	if len(pkCols) > 0 {
		lines = append(lines, "  PRIMARY KEY ("+quotedJoin(pkCols)+")")
	}
	lines = append(lines, constraints...)
	return "CREATE TABLE " + identQuote(table) + " (\n" + strings.Join(lines, ",\n") + "\n)", nil
}

// IndexesDDL renders the CREATE INDEX statement of every non-pk index
// of a shape body, without trailing semicolons, named for table (spec
// §migrations rules 5, 9 and 10).
func IndexesDDL(table string, body []ast.TableItem) ([]string, error) {
	var out []string
	used := map[string]bool{}
	for _, item := range body {
		block, ok := item.(*ast.IndexesBlock)
		if !ok {
			continue
		}
		for _, ix := range block.Indexes {
			if ix.Settings.Get("pk") != nil {
				continue
			}
			name, err := IndexName(table, ix)
			if err != nil {
				return nil, fmt.Errorf("table %s: %w", table, err)
			}
			if used[name] {
				return nil, fmt.Errorf("table %s: duplicate index name %q", table, name)
			}
			used[name] = true
			out = append(out, IndexDDL(table, name, ix))
		}
	}
	return out, nil
}

// ColumnDDL renders the definition of a column added by ALTER TABLE
// ADD COLUMN (spec §migrations rule 8): its type, NOT NULL, DEFAULT
// and REFERENCES with its delete: and update: actions. The flag says
// the column is unique, which the caller lowers to a CREATE UNIQUE
// INDEX, since SQLite takes no UNIQUE here.
func ColumnDDL(col *ast.Column) (def string, unique bool, err error) {
	name := col.Name.Name()
	sqlType, ok := TypeSQL(col.Type.String())
	if !ok {
		return "", false, fmt.Errorf("column %q: type %q is not a built-in (§migrations)", name, col.Type.String())
	}
	parts := []string{identQuote(name), sqlType}
	if !isNullable(col) {
		parts = append(parts, "NOT NULL")
	}
	if s := col.Settings.Get("default"); s != nil {
		d, err := defaultSQL(s.Value)
		if err != nil {
			return "", false, fmt.Errorf("column %q: %w", name, err)
		}
		parts = append(parts, "DEFAULT "+d)
	}
	unique = col.Settings.Get("unique") != nil
	if ep := refEndpoint(col); ep != nil {
		parts = append(parts, "REFERENCES "+referenceSQL(col, ep))
	}
	return strings.Join(parts, " "), unique, nil
}

func pkKeyColumns(body []ast.TableItem) []string {
	var out []string
	for _, item := range body {
		block, ok := item.(*ast.IndexesBlock)
		if !ok {
			continue
		}
		for _, ix := range block.Indexes {
			if ix.Settings.Get("pk") == nil {
				continue
			}
			for _, atom := range ix.Key {
				if id, ok := atom.(*ast.Ident); ok {
					out = append(out, id.Name())
				}
			}
		}
	}
	return out
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
// update: settings (spec §migrations rule 4, §relationships-ref).
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
