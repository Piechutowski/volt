// Pass 2: local (single-element) constraints: settings whitelists, value
// types, body shape.
package check

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (c *checker) declsCheck(f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Use:
			c.useCheck(d)
		case *ast.Project:
		case *ast.Table:
			if ti := c.info.byTable[canonKey(d.Name)]; ti == nil || ti.Decl != d {
				c.tableBodyCheck(d.Pos(), d.Name.String(), d.Settings, d.Body, true)
			}
		case *ast.TablePartial:
			c.tableBodyCheck(d.Pos(), d.Name.Name(), d.Settings, d.Body, false)
		case *ast.Enum:
			c.enumCheck(d)
		case *ast.Ref:
			c.refSettingsCheck(d.Settings)
			c.endpointArityCheck(d.Left, d.Right)
		case *ast.StickyNote:
			c.settingsCheck(d.Settings, "notes", stickySettings)
		case *ast.TableGroup:
			c.settingsCheck(d.Settings, "tablegroup", groupSettings)
		case *ast.DiagramView:
			c.viewCheck(d)
		case *ast.Records:
			c.recordsCheck(d, true)
			if d.Table != nil {
				if ti := c.info.byTable[canonKey(d.Table)]; ti != nil {
					c.recordsTypeCheck(ti, d)
				}
			}
		}
	}
}

func (c *checker) useCheck(d *ast.Use) {
	kw := "use"
	if d.Reuse {
		kw = "reuse"
	}
	c.errorf(d.Pos(), "file-imports-removed", "%s is not part of the Volt language (§file-imports-removed); replace file imports with the package system — see docs/spec.md §compatibility-with-dbml for migration", kw)
}

func (c *checker) tableBodyCheck(at token.Position, name string, settings *ast.SettingList, body []ast.TableItem, isTable bool) {
	c.settingsCheck(settings, "table", tableSettings)

	columns := map[string]bool{}
	nCols, nInject, nNotes, nRecords := 0, 0, 0, 0
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
			}
		case *ast.ValidatorsBlock:
		case *ast.Note:
			nNotes++
		case *ast.Records:
			if !isTable {
				c.errorf(item.Pos(), "tablepartial", "records are not allowed inside TablePartial")
			}
			nRecords++
			c.recordsCheck(item, false)
		}
	}
	if isTable && nCols == 0 && nInject == 0 {
		c.errorf(at, "table", "table %q must contain at least one column or partial injection", name)
	}
	if nNotes > 1 {
		c.errorf(bodyPos(body, settings), "notes", "at most one Note per %s body", kindWord(isTable))
	}
	if nRecords > 1 {
		c.errorf(bodyPos(body, settings), "records-sample-data", "at most one records block per table")
	}
}

func kindWord(isTable bool) string {
	if isTable {
		return "Table"
	}
	return "TablePartial"
}

func bodyPos(body []ast.TableItem, settings *ast.SettingList) token.Position {
	if len(body) > 0 {
		return body[0].Pos()
	}
	if settings != nil {
		return settings.Pos()
	}
	return token.Position{}
}

func (c *checker) columnCheck(col *ast.Column) {
	c.legacyFlagsCheck(col)
	c.settingsCheck(col.Settings, "columns", columnSettings)
	c.tagSettingsCheck(col.Settings)
}

func (c *checker) legacyFlagsCheck(col *ast.Column) {
	for _, f := range col.LegacyFlags {
		low := strings.ToLower(f.Name())
		if f.Quoted() || (low != "pk" && low != "unique") {
			c.errorf(f.Pos(), "columns", "only the legacy flags pk and unique may follow the column type; found %q", f.Name())
		}
	}
}

func (c *checker) enumCheck(d *ast.Enum) {
	if len(d.Values) == 0 {
		c.errorf(d.Pos(), "enum", "enum %q must contain at least one value", d.Name.String())
	}
	seen := map[string]bool{}
	for _, v := range d.Values {
		if seen[v.Name.Name()] {
			c.errorf(v.Pos(), "namespaces-and-uniqueness", "duplicate enum value %q", v.Name.Name())
		}
		seen[v.Name.Name()] = true
		c.settingsCheck(v.Settings, "enum", enumValueSettings)
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
	"cascade": true, "restrict": true, "set null": true,
	"set default": true, "no action": true,
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
		if action := refActionOf(s); !refActions[action] {
			c.errorf(s.Pos(), "relationships-ref", "invalid referential action %q for '%s'; expected cascade, restrict, set null, set default or no action", action, s.Name)
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
					c.errorf(id.Pos(), "records-sample-data", "record values must be literals, null, true, false or enum constants; found bare identifier %q", id.Name())
				}
			}
		}
	}
}

var categoryKinds = map[string]bool{
	"tables": true, "notes": true, "tablegroups": true, "schemas": true,
}

func (c *checker) viewCheck(d *ast.DiagramView) {
	seen := map[string]bool{}
	for _, cat := range d.Categories {
		low := strings.ToLower(cat.Kind.Name())
		if !categoryKinds[low] || cat.Kind.Quoted() {
			c.errorf(cat.Kind.Pos(), "diagramview", "unknown view category %q; expected Tables, Notes, TableGroups or Schemas", cat.Kind.Name())
			continue
		}
		if seen[low] {
			c.errorf(cat.Kind.Pos(), "diagramview", "category %q may appear at most once per view", cat.Kind.Name())
		}
		seen[low] = true
	}
}
