// Migrations (spec §migrations): the elements are collected in name
// order and rules 1 to 12 are enforced without the schema, since a
// migration may say anything that was once true; their composition is
// the fold's to judge (§the-fold).
package check

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
)

// MigrationInfo is one Migration element (spec §migrations).
type MigrationInfo struct {
	Decl   *ast.Migration
	Name   string
	Number string // the digit run of the name, as written
	Label  string
	// Reversible says the migration runs backward: a flat body of
	// invertible operations, or an up block with a down block, not
	// marked irreversible (spec §migrations).
	Reversible bool
}

var (
	migrationSettings = settingSpec{
		kinds: map[string]valueKind{"irreversible": flagOnly, "no transaction": flagOnly, "note": strValue},
	}
	migrationColumnKinds = map[string]valueKind{
		"pk": flagOnly, "primary key": flagOnly,
		"null": flagOnly, "not null": flagOnly,
		"unique": flagOnly, "increment": flagOnly,
		"default": defaultVal, "ref": refVal, "note": strValue,
	}
	migrationForbidden = map[string]string{
		"tag":         "tag: describes the Go struct field, not the database, and is not a migration setting (§migrations)",
		"required":    "required describes Go-side validation, not the database, and is not a migration setting (§migrations)",
		"model":       "model: names the Go model, not the database, and is not a migration setting (§migrations)",
		"headercolor": "headercolor: describes the diagram, not the database, and is not a migration setting (§migrations)",
	}
	migrationColumnSettings = settingSpec{
		kinds:      migrationColumnKinds,
		repeatable: map[string]bool{"ref": true},
		synonyms:   map[string]string{"primary key": "pk"},
		conflicts:  [][2]string{{"null", "not null"}},
		forbidden:  migrationForbidden,
	}
	rebuildColumnSettings = settingSpec{
		kinds:      withKind(migrationColumnKinds, "from", exprValue),
		repeatable: map[string]bool{"ref": true},
		synonyms:   map[string]string{"primary key": "pk"},
		conflicts:  [][2]string{{"null", "not null"}},
		forbidden:  migrationForbidden,
	}
	migrationIndexSettings = settingSpec{
		kinds: map[string]valueKind{
			"unique": flagOnly, "pk": flagOnly,
			"name": strValue, "where": exprValue, "note": strValue,
		},
	}
	execSettings = settingSpec{
		kinds: map[string]valueKind{"down": strValue},
	}
)

func withKind(kinds map[string]valueKind, name string, kind valueKind) map[string]valueKind {
	out := make(map[string]valueKind, len(kinds)+1)
	for k, v := range kinds {
		out[k] = v
	}
	out[name] = kind
	return out
}

func (c *checker) migrationsCheck(f *ast.File) {
	var decls []*ast.Migration
	for _, d := range f.Decls {
		if m, ok := d.(*ast.Migration); ok {
			decls = append(decls, m)
		}
	}
	if len(decls) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, d := range decls {
		if len(c.info.Tables) == 0 {
			c.errorf(d.Pos(), "migrations", "migration %q in a package that declares no tables: a migration is an element of the package whose tables it migrates (§migrations)", d.Name.Name())
		}
		mi := c.migrationCheck(d)
		if mi == nil {
			continue
		}
		if seen[mi.Name] {
			c.errorf(d.Name.Pos(), "migrations", "duplicate migration name %q: migration names are unique within the package (§migrations)", mi.Name)
			continue
		}
		seen[mi.Name] = true
		c.info.Migrations = append(c.info.Migrations, mi)
	}
	sort.SliceStable(c.info.Migrations, func(i, j int) bool {
		return c.info.Migrations[i].Name < c.info.Migrations[j].Name
	})
	c.rebuildSourcesCheck()
}

func migrationNameParts(name string) (number, label string, ok bool) {
	i := 0
	for i < len(name) && '0' <= name[i] && name[i] <= '9' {
		i++
	}
	if i < 4 || !strings.HasPrefix(name[i:], "__") || len(name) == i+2 {
		return "", "", false
	}
	return name[:i], name[i+2:], true
}

func (c *checker) migrationCheck(d *ast.Migration) *MigrationInfo {
	name := d.Name.Name()
	number, label, ok := migrationNameParts(name)
	if d.Name.Quoted() || !ok {
		c.errorf(d.Name.Pos(), "migrations", "migration name %q is not four or more digits, two underscores and a label, like 0001__create_users (§migrations)", name)
	}
	c.settingsCheck(d.Settings, "migrations", migrationSettings)
	c.operationsCheck(d.Up)
	c.operationsCheck(d.Down)

	irreversible := d.Settings.Get("irreversible") != nil
	reversible := !irreversible
	if d.UpDown {
		reversible = reversible && d.HasDown()
	} else {
		for _, op := range d.Up {
			why := inverseMissing(op)
			if why == "" {
				continue
			}
			reversible = false
			if !irreversible {
				c.errorf(op.Pos(), "migrations", "migration %q is a flat body, which runs backward as the inverses of its operations, but %s; mark the migration irreversible or write it as up { } down { } (§migrations)", name, why)
			}
		}
	}
	if !ok || d.Name.Quoted() {
		return nil
	}
	return &MigrationInfo{Decl: d, Name: name, Number: number, Label: label, Reversible: reversible}
}

// inverseMissing says why an operation has no inverse, or "" when it
// has one (spec §migrations rule 12).
func inverseMissing(op ast.Operation) string {
	switch op := op.(type) {
	case *ast.DropTable:
		if !op.HasBody() {
			return "drop table " + op.Name.Name() + " has no body to recreate the table from"
		}
	case *ast.AlterTable:
		for _, item := range op.Items {
			if d, ok := item.(*ast.AlterDrop); ok && d.Type == nil {
				return "drop " + d.Name.Name() + " in alter table " + op.Name.Name() + " carries no type to add the column back with"
			}
		}
	case *ast.RebuildTable:
		return "rebuild table " + op.Name.Name() + " forgets the old shape"
	case *ast.Exec:
		if op.DownSQL() == nil {
			return "exec carries no down:"
		}
	}
	return ""
}

func (c *checker) operationsCheck(ops []ast.Operation) {
	for _, op := range ops {
		switch op := op.(type) {
		case *ast.CreateTable:
			shape := c.shapeBodyCheck(op.Name.Name(), op.Body, migrationColumnSettings)
			if len(shape.Columns) == 0 {
				c.errorf(op.Pos(), "migrations", "create table %q must declare at least one column (§migrations)", op.Name.Name())
			}
		case *ast.DropTable:
			if op.HasBody() {
				c.shapeBodyCheck(op.Name.Name(), op.Body, migrationColumnSettings)
			}
		case *ast.AlterTable:
			for _, item := range op.Items {
				switch item := item.(type) {
				case *ast.AlterAdd:
					c.addColumnCheck(op.Name.Name(), item.Column)
				case *ast.AlterDrop:
					if item.Type != nil {
						c.migrationColumnCheck(&ast.Column{Name: item.Name, Type: item.Type, Settings: item.Settings}, migrationColumnSettings, nil)
					}
				}
			}
		case *ast.RebuildTable:
			c.shapeBodyCheck(op.Name.Name(), op.Body, rebuildColumnSettings)
		case *ast.CreateIndex:
			c.migrationIndexCheck(op.Table.Name(), op.Index)
		case *ast.DropIndex:
			c.migrationIndexCheck(op.Table.Name(), op.Index)
		case *ast.Exec:
			c.settingsCheck(op.Settings, "migrations", execSettings)
		}
	}
}

// shapeBodyCheck checks the body of a create, drop or rebuild table
// operation as the schema checks a table (spec §migrations rules 3 to
// 5) and returns its columns and indexes.
func (c *checker) shapeBodyCheck(table string, body []ast.TableItem, spec settingSpec) *TableInfo {
	ti := &TableInfo{}
	for _, item := range body {
		if b, ok := item.(*ast.IndexesBlock); ok {
			ti.Indexes = append(ti.Indexes, b.Indexes...)
		}
	}
	pk := pkIndexColumns(ti.Indexes)
	seen := map[string]bool{}
	for _, item := range body {
		switch item := item.(type) {
		case *ast.Column:
			name := item.Name.Name()
			if seen[name] {
				c.errorf(item.Pos(), "migrations", "duplicate column name %q in table %q (§migrations)", name, table)
			}
			seen[name] = true
			ti.Columns = append(ti.Columns, c.migrationColumnCheck(item, spec, pk))
		case *ast.IndexesBlock:
			for _, ix := range item.Indexes {
				c.migrationIndexCheck(table, ix)
			}
		}
	}
	c.incrementCheck(ti)
	return ti
}

// migrationColumnCheck checks one column of a migration: its settings
// against the whitelist, its type against the built-ins, its default
// against the type (spec §migrations rules 3 and 4).
func (c *checker) migrationColumnCheck(col *ast.Column, spec settingSpec, pk map[string]bool) *ColumnDef {
	c.legacyFlagsCheck(col)
	c.settingsCheck(col.Settings, "migrations", spec)
	cd := &ColumnDef{Col: col, Nullable: isNullable(col) && !pk[col.Name.Name()]}
	typ, ok := builtins[col.Type.String()]
	if !ok {
		c.errorf(col.Type.Pos(), "migrations", "column %q: type %q is not a built-in; a migration column is bool, int, float64, string, []byte or time.Time, since the database holds the storage type (§migrations)", col.Name.Name(), col.Type.String())
		return cd
	}
	cd.Type = typ
	c.defaultTypeCheck(cd)
	return cd
}

// addColumnCheck enforces SQLite's ADD COLUMN rules on an add item
// (spec §migrations rule 8).
func (c *checker) addColumnCheck(table string, col *ast.Column) {
	cd := c.migrationColumnCheck(col, migrationColumnSettings, nil)
	name := col.Name.Name()
	keyed := false
	for _, s := range []string{"pk", "primary key", "increment"} {
		if setting := col.Settings.Get(s); setting != nil {
			keyed = true
			c.errorf(setting.Pos(), "migrations", "add %q to table %q: %s cannot be added to an existing table; use rebuild table (§migrations)", name, table, s)
		}
	}
	for _, f := range col.LegacyFlags {
		if strings.EqualFold(f.Name(), "pk") {
			keyed = true
			c.errorf(f.Pos(), "migrations", "add %q to table %q: pk cannot be added to an existing table; use rebuild table (§migrations)", name, table)
		}
	}
	def := col.Settings.Get("default")
	if !cd.Nullable && def == nil && !keyed {
		c.errorf(col.Pos(), "migrations", "add %q to table %q: a not null column needs a default:, since the rows already in the table must be filled (§migrations)", name, table)
	}
	if def != nil {
		if _, expr := def.Value.(*ast.FuncExpr); expr {
			c.errorf(def.Value.Pos(), "migrations", "add %q to table %q: the default must be a literal; SQLite's ADD COLUMN takes no expression default (§migrations)", name, table)
		}
	}
	if ref := col.Settings.Get("ref"); ref != nil && !cd.Nullable {
		c.errorf(ref.Pos(), "migrations", "add %q to table %q: a column with ref: must be nullable when added, since the rows already in the table reference nothing; use rebuild table for a not null foreign key (§migrations)", name, table)
	}
}

func (c *checker) migrationIndexCheck(table string, ix *ast.Index) {
	c.settingsCheck(ix.Settings, "migrations", migrationIndexSettings)
	c.indexNameCheck(table, ix)
}

// indexNameCheck enforces that an index whose key holds an expression
// carries name:, since no name derives from it (spec §indexes).
func (c *checker) indexNameCheck(table string, ix *ast.Index) {
	if ix.Settings.Get("name") != nil || ix.Settings.Get("pk") != nil {
		return
	}
	for _, atom := range ix.Key {
		if _, ok := atom.(*ast.FuncExpr); ok {
			c.errorf(ix.Pos(), "indexes", "index %s on %q has an expression in its key and must carry name:, since a name derives from columns only (§indexes)", indexKeyString(ix), table)
			return
		}
	}
}

func indexKeyString(ix *ast.Index) string {
	parts := make([]string, 0, len(ix.Key))
	for _, atom := range ix.Key {
		switch a := atom.(type) {
		case *ast.Ident:
			parts = append(parts, a.Name())
		case *ast.FuncExpr:
			parts = append(parts, "`"+a.Text()+"`")
		}
	}
	if ix.Composite {
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return strings.Join(parts, ", ")
}

// rebuildSourcesCheck follows the column names of every table through
// the package's migrations in name order, so that a rebuild's not null
// column whose source would be NULL, no from:, no default:, no old
// column of its name, is an error (spec §migrations rule 9). A table
// no create table operation made is unknown and goes unchecked.
func (c *checker) rebuildSourcesCheck() {
	tables := map[string][]string{}
	for _, mi := range c.info.Migrations {
		for _, op := range mi.Decl.Up {
			switch op := op.(type) {
			case *ast.CreateTable:
				tables[op.Name.Name()] = shapeColumnNames(op.Body)
			case *ast.DropTable:
				delete(tables, op.Name.Name())
			case *ast.RenameTable:
				if cols, known := tables[op.From.Name()]; known {
					delete(tables, op.From.Name())
					tables[op.To.Name()] = cols
				}
			case *ast.AlterTable:
				cols, known := tables[op.Name.Name()]
				if !known {
					continue
				}
				for _, item := range op.Items {
					switch item := item.(type) {
					case *ast.AlterAdd:
						cols = append(cols, item.Column.Name.Name())
					case *ast.AlterDrop:
						cols = without(cols, item.Name.Name())
					case *ast.AlterRename:
						cols = append(without(cols, item.From.Name()), item.To.Name())
					}
				}
				tables[op.Name.Name()] = cols
			case *ast.RebuildTable:
				if old, known := tables[op.Name.Name()]; known {
					c.rebuildSourceCheck(mi.Name, op, old)
				}
				tables[op.Name.Name()] = shapeColumnNames(op.Body)
			}
		}
	}
}

func (c *checker) rebuildSourceCheck(mig string, op *ast.RebuildTable, old []string) {
	var indexes []*ast.Index
	for _, item := range op.Body {
		if b, ok := item.(*ast.IndexesBlock); ok {
			indexes = append(indexes, b.Indexes...)
		}
	}
	pk := pkIndexColumns(indexes)
	for _, item := range op.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		name := col.Name.Name()
		if isNullable(col) && !pk[name] || col.Settings.Get("from") != nil || col.Settings.Get("default") != nil || contains(old, name) {
			continue
		}
		c.errorf(col.Pos(), "migrations", "rebuild table %q in migration %q: column %q is not null but its source is NULL, since no column of that name exists before the rebuild; give it a from: or a default: (§migrations)", op.Name.Name(), mig, name)
	}
}

func shapeColumnNames(body []ast.TableItem) []string {
	var out []string
	for _, item := range body {
		if col, ok := item.(*ast.Column); ok {
			out = append(out, col.Name.Name())
		}
	}
	return out
}

func without(names []string, name string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
