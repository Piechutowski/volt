// Pass 2: local (single-element) constraints: settings whitelists, value
// types, body shape.
package check

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (c *checker) declsCheck(f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Project:
			c.settingsCheck(d.Settings, "project", projectSettings)
		case *ast.Table:
			if ti := c.info.byName[d.Name.Name()]; ti == nil || ti.Decl != d {
				c.tableBodyCheck(d.Pos(), d.Name.Name(), d.Settings, d.Body, d.Incomplete, true)
			}
		case *ast.TablePartial:
			c.tableBodyCheck(d.Pos(), d.Name.Name(), nil, d.Body, d.Incomplete, false)
		case *ast.Ref:
			c.refSettingsCheck(d.Settings)
			c.endpointArityCheck(d.Left, d.Right)
		case *ast.Records:
			c.recordsCheck(d, true)
			if d.Table != nil {
				if ti := c.info.byName[d.Table.Name()]; ti != nil {
					c.recordsTypeCheck(ti, d)
					c.recordsCoverageCheck(ti, d)
				}
			}
		}
	}
}

func (c *checker) tableBodyCheck(at token.Position, name string, settings *ast.SettingList, body []ast.TableItem, incomplete, isTable bool) {
	c.settingsCheck(settings, "table", tableSettings)

	columns := map[string]bool{}
	nCols, nInject, nRecords := 0, 0, 0
	for _, item := range body {
		switch item := item.(type) {
		case *ast.Column:
			c.columnCheck(item)
			if columns[item.Name.Name()] {
				c.errorf(item.Pos(), "namespaces-and-uniqueness", "duplicate column name %q", item.Name.Name())
			}
			columns[item.Name.Name()] = true
			nCols++
		case *ast.PartialRef:
			if !isTable {
				c.errorf(item.Pos(), "tablepartial", "partial injection is not allowed inside TablePartial")
			}
			nInject++
		case *ast.IndexesBlock:
			for _, ix := range item.Indexes {
				c.settingsCheck(ix.Settings, "indexes", indexSettings)
				c.indexNameCheck(name, ix)
				c.pkIndexCheck(name, ix, indexSettings)
			}
		case *ast.ValidatorsBlock:
		case *ast.Records:
			if !isTable {
				c.errorf(item.Pos(), "tablepartial", "records are not allowed inside TablePartial")
			}
			nRecords++
			c.recordsCheck(item, false)
		}
	}
	if isTable && nCols == 0 && nInject == 0 && !incomplete {
		c.errorf(at, "table", "table %q must contain at least one column or partial injection", name)
	}
	if nRecords > 1 {
		c.errorf(body[0].Pos(), "records-sample-data", "at most one records block per table")
	}
}

func (c *checker) columnCheck(col *ast.Column) {
	c.settingsCheck(col.Settings, "columns", columnSettings)
	c.tagSettingsCheck(col.Settings)
	c.pkNullCheck(col)
	if req, null := col.Settings.Get("required"), col.Settings.Get("null"); req != nil && null != nil {
		c.errorf(req.Pos(), "columns", "required on %q: a column that carries null has no empty value to refuse; NULL is absence (§columns)", col.Name.Name())
	}
}

func (c *checker) pkNullCheck(col *ast.Column) {
	if s := col.Settings.Get("null"); s != nil && col.Settings.Get("pk") != nil {
		c.errorf(s.Pos(), "columns", "column %q carries pk and null: a key column is never NULL (§columns)", col.Name.Name())
	}
}

func (c *checker) pkIndexNullCheck(col *ast.Column, table string, pkIndex map[string]bool) {
	if s := col.Settings.Get("null"); s != nil && pkIndex[col.Name.Name()] && col.Settings.Get("pk") == nil {
		c.errorf(s.Pos(), "columns", "column %q of table %q is a member of the [pk] index and carries null: a key column is never NULL (§columns)", col.Name.Name(), table)
	}
}

func (c *checker) pkCountCheck(table string, cols []*ColumnDef, indexes []*ast.Index) {
	first := ""
	for _, cd := range cols {
		if s := cd.Col.Settings.Get("pk"); s != nil {
			first = c.pkSecondCheck(table, first, fmt.Sprintf("column %q", cd.Col.Name.Name()), s.Pos())
		}
	}
	for _, ix := range indexes {
		if s := ix.Settings.Get("pk"); s != nil {
			first = c.pkSecondCheck(table, first, "index "+indexKeyString(ix), s.Pos())
		}
	}
}

func (c *checker) pkSecondCheck(table, first, this string, pos token.Position) string {
	if first == "" {
		return this
	}
	c.errorf(pos, "columns", "table %q declares a second primary key, %s beside %s: a table has one primary key, one pk column or one [pk] index (§columns)", table, this, first)
	return first
}

func (c *checker) pkIndexCheck(table string, ix *ast.Index, spec settingSpec) {
	if ix.Settings.Get("pk") == nil {
		return
	}
	for _, atom := range ix.Key {
		if _, ok := atom.(*ast.FuncExpr); ok {
			c.errorf(atom.Pos(), "indexes", "index %s on %q carries pk and has an expression in its key: a primary key is columns only (§indexes)", indexKeyString(ix), table)
		}
	}
	for _, f := range []struct{ name, why string }{
		{"name", "a primary key is a table constraint and has no name"},
		{"where", "a primary key covers every row"},
	} {
		if _, known := spec.kinds[f.name]; !known {
			continue
		}
		if s := ix.Settings.Get(f.name); s != nil {
			c.errorf(s.Pos(), "indexes", "index %s on %q carries pk and %s:, which it does not take, since %s (§indexes)", indexKeyString(ix), table, f.name, f.why)
		}
	}
}

func (c *checker) endpointArityCheck(l, r *ast.RefEndpoint) {
	if l == nil || r == nil {
		return
	}
	if len(l.Columns) != len(r.Columns) {
		c.errorf(r.Pos(), "relationships-ref", "composite relationship endpoints must have the same number of columns: %d vs %d", len(l.Columns), len(r.Columns))
	}
}

var refActions = map[string]bool{
	"cascade": true, "restrict": true, "set_null": true, "set_default": true,
}

func (c *checker) refSettingsCheck(sl *ast.SettingList) {
	c.settingsCheck(sl, "relationships-ref", refSettings)
	if sl == nil {
		return
	}
	for _, s := range sl.Settings {
		if s.Name != "delete" && s.Name != "update" {
			continue
		}
		if _, ok := s.Value.(*ast.Ident); !ok {
			continue
		}
		if action := refActionOf(s); !refActions[action] {
			c.errorf(s.Pos(), "relationships-ref", "invalid referential action %q for '%s'; expected cascade, restrict, set_null or set_default; leave %s: out for SQLite's default (§relationships-ref)", action, s.Name, s.Name)
		}
	}
}

func (c *checker) recordsCheck(d *ast.Records, topLevel bool) {
	if topLevel && !d.HasColumns {
		c.errorf(d.Pos(), "records-sample-data", "top-level records must list columns")
	}
	for _, row := range d.Rows {
		if d.HasColumns && len(row.Values) != len(d.Columns) {
			c.errorf(row.Pos(), "records-sample-data", "record row has %d values but %d columns are listed", len(row.Values), len(d.Columns))
		}
		for _, v := range row.Values {
			if id, ok := v.(*ast.Ident); ok {
				low := strings.ToLower(id.Name())
				if id.Quoted() || (low != "true" && low != "false" && low != "null") {
					c.errorf(id.Pos(), "records-sample-data", "record values must be literals, null, true or false; found bare identifier %q (§records-sample-data)", id.Name())
				}
			}
		}
	}
}
