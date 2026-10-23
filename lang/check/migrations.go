// Migrations (spec §migrations): the elements are collected in name
// order and rules 1 to 12 are enforced without the schema, since a
// migration may say anything that was once true; their composition is
// the fold's to judge (§the-fold).
package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
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
		kinds: map[string]valueKind{"irreversible": flagOnly, "no_transaction": flagOnly},
	}
	migrationColumnKinds = map[string]valueKind{
		"pk": flagOnly, "null": flagOnly,
		"unique": flagOnly, "index": flagOnly, "increment": flagOnly,
		"default": defaultVal, "ref": refVal,
		"delete": actionValue, "update": actionValue,
	}
	migrationColumnConflicts = [][2]string{{"unique", "index"}}
	migrationForbidden       = map[string]string{
		"tag":      "tag: describes the Go struct field, not the database, and is not a migration setting (§migrations)",
		"required": "required describes Go-side validation, not the database, and is not a migration setting (§migrations)",
		"model":    "model: names the Go model, not the database, and is not a migration setting (§migrations)",
		"note":     "note: documents the schema, which generated code carries, and is not a migration setting (§migrations)",
	}
	migrationColumnSettings = settingSpec{
		kinds:     migrationColumnKinds,
		conflicts: migrationColumnConflicts,
		forbidden: mapWith(migrationForbidden, "from", "from: names the source of a rebuilt column and is a setting of rebuild table only (§migrations)"),
	}
	rebuildColumnSettings = settingSpec{
		kinds:     mapWith(migrationColumnKinds, "from", exprValue),
		conflicts: migrationColumnConflicts,
		forbidden: migrationForbidden,
	}
	migrationIndexSettings = settingSpec{
		kinds: map[string]valueKind{
			"unique": flagOnly,
			"name":   strValue, "where": exprValue,
		},
		forbidden: map[string]string{"pk": pkNotIndexSetting("migrations")},
	}
	execSettings = settingSpec{
		kinds: map[string]valueKind{"down": exprValue},
	}
)

func mapWith[V any](m map[string]V, name string, v V) map[string]V {
	out := make(map[string]V, len(m)+1)
	for k, x := range m {
		out[k] = x
	}
	out[name] = v
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
	c.migrationWidthCheck()
	c.rebuildSourcesCheck()
}

// migrationWidthCheck enforces one digit-run width across the package's
// migration names, so that byte order and numeric order agree (spec
// §migrations rule 2); a name of another width than the narrowest is
// reported with the first name of that width.
func (c *checker) migrationWidthCheck() {
	var narrowest *MigrationInfo
	for _, mi := range c.info.Migrations {
		if narrowest == nil || len(mi.Number) < len(narrowest.Number) {
			narrowest = mi
		}
	}
	for _, mi := range c.info.Migrations {
		if len(mi.Number) != len(narrowest.Number) {
			c.errorf(mi.Decl.Name.Pos(), "migrations", "migration names %q and %q differ in digit width: every migration name of a package has the same width, so that byte order and numeric order agree; widen every name (§migrations)", narrowest.Name, mi.Name)
		}
	}
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
	if !ok {
		c.errorf(d.Name.Pos(), "migrations", "migration name %q is not four or more digits, two underscores and a label, like 0001__create_users (§migrations)", name)
	}
	c.settingsCheck(d.Settings, "migrations", migrationSettings)
	noTx := d.Settings.Get("no_transaction") != nil
	c.operationsCheck(d.Up, !d.UpDown, noTx)
	c.operationsCheck(d.Down, false, noTx)

	irreversible := d.Settings.Get("irreversible") != nil
	reversible := !irreversible
	if d.UpDown {
		reversible = reversible && d.HasDown()
	} else {
		for _, op := range d.Up {
			for _, m := range inversesMissing(op) {
				reversible = false
				if !irreversible {
					c.errorf(m.pos, "migrations", "migration %q is a flat body, which runs backward as the inverses of its operations, but %s; mark the migration irreversible or write it as up { } down { } (§migrations)", name, m.why)
				}
			}
		}
	}
	if !ok {
		return nil
	}
	return &MigrationInfo{Decl: d, Name: name, Number: number, Label: label, Reversible: reversible}
}

type missingInverse struct {
	pos token.Position
	why string
}

// inversesMissing lists what in an operation has no inverse, an alter
// table item at its own position (spec §migrations rule 12).
func inversesMissing(op ast.Operation) []missingInverse {
	switch op := op.(type) {
	case *ast.DropTable:
		if !op.HasBody() {
			return []missingInverse{{op.Pos(), "drop table " + op.Name.Name() + " has no body to recreate the table from"}}
		}
	case *ast.AlterTable:
		var out []missingInverse
		for _, item := range op.Items {
			if d, ok := item.(*ast.AlterDrop); ok && d.Type == nil {
				out = append(out, missingInverse{d.Pos(), "drop " + d.Name.Name() + " in alter table " + op.Name.Name() + " carries no type to add the column back with"})
			}
		}
		return out
	case *ast.RebuildTable:
		return []missingInverse{{op.Pos(), "rebuild table " + op.Name.Name() + " forgets the old shape"}}
	case *ast.Exec:
		if op.DownSQL() == nil {
			return []missingInverse{{op.Pos(), "exec carries no down:"}}
		}
	}
	return nil
}

// operationsCheck checks the operations of one body; flat says the
// body is a flat one, whose typed drops are inverse adds, and noTx
// that the migration is no_transaction (spec §migrations rules 8 and
// 12).
func (c *checker) operationsCheck(ops []ast.Operation, flat, noTx bool) {
	for _, op := range ops {
		switch op := op.(type) {
		case *ast.CreateTable:
			c.shapeBodyCheck("create table", op.Name.Name(), op.Pos(), op.Body, op.Incomplete, migrationColumnSettings)
		case *ast.DropTable:
			if op.HasBody() {
				c.shapeBodyCheck("drop table", op.Name.Name(), op.Pos(), op.Body, op.Incomplete, migrationColumnSettings)
			}
		case *ast.AlterTable:
			for _, item := range op.Items {
				switch item := item.(type) {
				case *ast.AlterAdd:
					c.addColumnCheck(item.Column, fmt.Sprintf("add %q to table %q", item.Column.Name.Name(), op.Name.Name()), noTx)
				case *ast.AlterDrop:
					if item.Type == nil {
						break
					}
					shape := &ast.Column{Name: item.Name, Type: item.Type, Settings: item.Settings}
					if flat {
						c.addColumnCheck(shape, fmt.Sprintf("drop %q from table %q, whose shape is the inverse add", item.Name.Name(), op.Name.Name()), noTx)
					} else {
						c.migrationColumnCheck(shape, migrationColumnSettings)
					}
				}
			}
		case *ast.RebuildTable:
			c.shapeBodyCheck("rebuild table", op.Name.Name(), op.Pos(), op.Body, op.Incomplete, rebuildColumnSettings)
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
// 6 and 9, §indexes) and returns its columns and indexes.
func (c *checker) shapeBodyCheck(what, table string, pos token.Position, body []ast.TableItem, incomplete bool, spec settingSpec) *TableInfo {
	ti := &TableInfo{Key: table}
	var named []namedIndex
	seen := map[string]bool{}
	for _, item := range body {
		switch item := item.(type) {
		case *ast.Column:
			name := item.Name.Name()
			if seen[name] {
				c.errorf(item.Pos(), "migrations", "duplicate column name %q in table %q (§migrations)", name, table)
			}
			seen[name] = true
			ti.Columns = append(ti.Columns, c.migrationColumnCheck(item, spec))
		case *ast.IndexesBlock:
			for _, ix := range item.Indexes {
				c.migrationIndexCheck(table, ix)
				ti.Indexes = append(ti.Indexes, ix)
				if indexNamed(ix) {
					c.indexPlainCheck(table, ix)
					named = append(named, namedIndex{ix: ix})
				}
			}
		}
	}
	if len(ti.Columns) == 0 && !incomplete {
		c.errorf(pos, "migrations", "%s %q: the body must declare at least one column (§migrations)", what, table)
	}
	c.indexNamesCheck(table, ti.Columns, named)
	c.incrementCheck(ti)
	return ti
}

// migrationColumnCheck checks one column of a migration: its settings
// against the whitelist, the referential actions of its ref:, its type
// against the built-ins, its default against the type, and null
// against pk (spec §migrations rules 3 and 4, §columns).
func (c *checker) migrationColumnCheck(col *ast.Column, spec settingSpec) *ColumnDef {
	c.settingsCheck(col.Settings, "migrations", spec)
	c.migrationRefCheck(col)
	c.pkNullCheck(col)
	cd := &ColumnDef{Col: col, Nullable: isNullable(col)}
	typ, ok := builtins[col.Type.String()]
	if !ok {
		c.errorf(col.Type.Pos(), "migrations", "column %q: type %q is not a built-in; a migration column is bool, int, float64, string, []byte or time.Time, since the database holds the storage type (§migrations)", col.Name.Name(), col.Type.String())
		return cd
	}
	cd.Type = typ
	c.defaultTypeCheck(cd)
	return cd
}

func (c *checker) migrationRefCheck(col *ast.Column) {
	if col.Settings == nil {
		return
	}
	name := col.Name.Name()
	hasRef := col.Settings.Get("ref") != nil
	for _, s := range col.Settings.Settings {
		switch s.Name {
		case "delete", "update":
			if !hasRef {
				c.errorf(s.Pos(), "migrations", "column %q: %s: names the action of the column's ref:, and the column declares none (§migrations)", name, s.Name)
				continue
			}
			if _, ok := s.Value.(*ast.Ident); !ok {
				continue
			}
			if action := refActionOf(s); !refActions[action] {
				c.errorf(s.Pos(), "migrations", "column %q: invalid referential action %q for '%s'; expected cascade, restrict, set_null or set_default; leave %s: out for SQLite's default (§migrations)", name, action, s.Name, s.Name)
				continue
			}
			c.refActionNullCheck(s, col, name, "migrations")
		}
	}
}

func (c *checker) refActionNullCheck(s *ast.Setting, col *ast.Column, column, section string) {
	switch action := refActionOf(s); {
	case action == "set_null" && !isNullable(col):
		c.errorf(s.Pos(), section, "%s: set_null on foreign-key column %q, which does not carry null: the database would write a NULL the column refuses (§%s)", s.Name, column, section)
	case action == "set_default" && !isNullable(col) && col.Settings.Get("default") == nil:
		c.errorf(s.Pos(), section, "%s: set_default on foreign-key column %q, which carries neither null nor a default:, so its default is NULL, which the column refuses (§%s)", s.Name, column, section)
	}
}

func refActionOf(s *ast.Setting) string {
	if v, ok := s.Value.(*ast.Ident); ok {
		return v.Name()
	}
	return ""
}

// addColumnCheck enforces SQLite's ADD COLUMN rules on an added column
// and, in a flat body, on the shape of a dropped one, which is the
// inverse add (spec §migrations rules 8 and 12); head names the item
// for the diagnostic and noTx says the migration is no_transaction,
// which cannot hold the schema edit of an expression default.
func (c *checker) addColumnCheck(col *ast.Column, head string, noTx bool) {
	cd := c.migrationColumnCheck(col, migrationColumnSettings)
	keyed := false
	for _, s := range []string{"pk", "increment"} {
		if setting := col.Settings.Get(s); setting != nil {
			keyed = true
			c.errorf(setting.Pos(), "migrations", "%s: %s cannot be added to an existing table; use rebuild table (§migrations)", head, s)
		}
	}
	def := col.Settings.Get("default")
	ref := col.Settings.Get("ref")
	if !cd.Nullable && def == nil && !keyed && ref == nil {
		c.errorf(col.Pos(), "migrations", "%s: a column without null is NOT NULL and needs a default:, since the rows already in the table must be filled (§migrations)", head)
	}
	if def != nil && noTx {
		if _, expr := def.Value.(*ast.FuncExpr); expr {
			c.errorf(def.Value.Pos(), "migrations", "%s: an expression default is added by editing the table's definition inside the migration's transaction, and a no_transaction migration has none; move the add to a migration of its own (§migrations)", head)
		}
	}
	if ref == nil {
		return
	}
	if !cd.Nullable {
		c.errorf(ref.Pos(), "migrations", "%s: a column with ref: must carry null when added, since the rows already in the table reference nothing; use rebuild table for a NOT NULL foreign key (§migrations)", head)
	}
	if def != nil {
		c.errorf(def.Value.Pos(), "migrations", "%s: a column with ref: takes no default, since SQLite refuses to add a REFERENCES column with a default other than NULL to a table that has rows (§migrations)", head)
	}
}

// migrationIndexCheck checks an index of a shape body or of a create
// index or drop index operation (spec §migrations rules 4 and 10).
func (c *checker) migrationIndexCheck(table string, ix *ast.Index) {
	c.settingsCheck(ix.Settings, "migrations", migrationIndexSettings)
	c.indexNameCheck(table, ix)
}

// indexNameCheck enforces that an index whose key holds an expression
// carries name:, since no name derives from it, and that a written
// name is not empty and does not begin with sqlite_, which SQLite
// keeps for its own objects (spec §indexes).
func (c *checker) indexNameCheck(table string, ix *ast.Index) {
	if s := ix.Settings.Get("name"); s != nil {
		lit, ok := s.Value.(*ast.BasicLit)
		switch {
		case !ok:
		case lit.Tok.Val == "":
			c.errorf(s.Pos(), "indexes", "index %s on %q has an empty name:; write a name, or leave name: out for the derived one (§indexes)", indexKeyString(ix), table)
		case strings.HasPrefix(sqliteFold(lit.Tok.Val), "sqlite_"):
			c.errorf(s.Pos(), "indexes", "index %s on %q is named %q: a name beginning with sqlite_ is SQLite's own and refused for an index (§indexes)", indexKeyString(ix), table, lit.Tok.Val)
		}
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
	if ix.Composite {
		return "(" + indexAtoms(ix) + ")"
	}
	return indexAtoms(ix)
}

// rebuildSourcesCheck follows the column names of every table through
// the package's migrations in name order, so that a rebuild's NOT NULL
// column whose source would be NULL, no from:, no default:, no old
// column of its name, is an error (spec §migrations rule 9). A rowid
// alias (int [pk, increment]) takes NULL and the database assigns it.
// A table no create table operation made is unknown and goes unchecked.
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
	for _, item := range op.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		name := col.Name.Name()
		if isNullable(col) || col.Settings.Get("increment") != nil || col.Settings.Get("from") != nil || col.Settings.Get("default") != nil || contains(old, name) {
			continue
		}
		c.errorf(col.Pos(), "migrations", "rebuild table %q in migration %q: column %q is NOT NULL but its source is NULL, since no column of that name exists before the rebuild; give it a from: or a default: (§migrations)", op.Name.Name(), mig, name)
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
