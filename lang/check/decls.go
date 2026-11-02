// Pass 2: local (single-element) constraints: settings whitelists, value
// types, body shape.
package check

import (
	"github.com/Piechutowski/volt/lang/ast"
)

func (c *checker) declsCheck(f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Project:
			c.settingsCheck(d.Settings, "project", projectSettings)
		case *ast.Table:
			if ti := c.info.byName[d.Name.Name()]; ti == nil || ti.Decl != d {
				c.tableBodyCheck(d.Name.Name(), d.Settings, d.Body, true)
			}
		case *ast.TablePartial:
			c.tableBodyCheck(d.Name.Name(), nil, d.Body, false)
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

func (c *checker) tableBodyCheck(name string, settings *ast.SettingList, body []ast.TableItem, isTable bool) {
	c.settingsCheck(settings, "table", tableSettings)

	columns := map[string]bool{}
	nRecords := 0
	for _, item := range body {
		switch item := item.(type) {
		case *ast.Column:
			c.columnCheck(item)
			if columns[item.Name.Name()] {
				c.errorf(item.Pos(), "namespaces-and-uniqueness", "duplicate column name %q", item.Name.Name())
			}
			columns[item.Name.Name()] = true
		case *ast.PartialRef:
			if !isTable {
				c.errorf(item.Pos(), "tablepartial", "partial injection is not allowed inside TablePartial")
			}
		case *ast.IndexesBlock:
			for _, ix := range item.Indexes {
				c.settingsCheck(ix.Settings, "indexes", indexSettings)
				c.indexNameCheck(name, ix)
				if indexNamed(ix) {
					c.indexPlainCheck(name, ix)
				}
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
	if nRecords > 1 {
		c.errorf(body[0].Pos(), "records-sample-data", "at most one Records block per table (§records-sample-data)")
	}
	if !isTable {
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
		c.partialIndexNamesCheck(name, cols, indexes)
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
				if name := id.Name(); name != "true" && name != "false" && name != "null" {
					c.errorf(id.Pos(), "records-sample-data", "record values must be literals, null, true or false; found bare identifier %q (§records-sample-data)", id.Name())
				}
			}
		}
	}
}
