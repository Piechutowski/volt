// Package migrate is the running half of migrations (spec §migrations):
// Lower turns the Migration elements of a checked package into the
// statements the runtime applies, Read takes the structure of a SQLite
// database and Diff compares two structures, the fold against the
// target (§the-fold). The package is stdlib and the driver is the
// caller's.
package migrate

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/rt"
)

// Lower renders every migration of info in both directions, in name
// order (spec §migrations rules 5 to 13): the up statements as written,
// the down statements as the down block or the inverses of a flat body
// in reverse order, nil when the migration is irreversible. It has no
// database: the shape of a table follows from the migrations alone, so
// a rebuild of a table no create table of theirs made reads every
// column of its body from the old table, and a column the table lacks
// is SQLite's error when the fold runs (§the-fold rule 4).
func Lower(info *check.Info) ([]rt.Migration, error) {
	l := &lowerer{tables: map[string][]string{}}
	out := make([]rt.Migration, 0, len(info.Migrations))
	for _, mi := range info.Migrations {
		m, err := l.migration(mi)
		if err != nil {
			return nil, fmt.Errorf("migration %s: %w", mi.Name, err)
		}
		out = append(out, m)
	}
	return out, nil
}

// lowerer follows the column names of every table the migrations
// created, in name order, so that a rebuild knows which of its columns
// have an old column to read (spec §migrations rule 9).
type lowerer struct {
	tables map[string][]string
}

func (l *lowerer) migration(mi *check.MigrationInfo) (rt.Migration, error) {
	d := mi.Decl
	m := rt.Migration{
		Name:          mi.Name,
		NoTransaction: d.Settings.Get("no_transaction") != nil,
	}
	m.ForeignKeysOff = !m.NoTransaction && (bracketNeeded(d.Up) || bracketNeeded(d.Down))
	up, err := l.operations(d.Up)
	if err != nil {
		return m, err
	}
	m.Up = up
	if !mi.Reversible {
		return m, nil
	}
	m.Down = []string{}
	if d.UpDown {
		after := &lowerer{tables: tablesClone(l.tables)}
		down, err := after.operations(d.Down)
		if err != nil {
			return m, err
		}
		m.Down = append(m.Down, down...)
		return m, nil
	}
	for i := len(d.Up) - 1; i >= 0; i-- {
		inv, err := inverse(d.Up[i])
		if err != nil {
			return m, err
		}
		m.Down = append(m.Down, inv...)
	}
	return m, nil
}

// bracketNeeded reports whether the operations hold a rebuild table or
// a drop table, which the foreign-key bracket surrounds unless
// no_transaction disables it (spec §migrations rule 13).
func bracketNeeded(ops []ast.Operation) bool {
	for _, op := range ops {
		switch op.(type) {
		case *ast.DropTable, *ast.RebuildTable:
			return true
		}
	}
	return false
}

func (l *lowerer) operations(ops []ast.Operation) ([]string, error) {
	var out []string
	for _, op := range ops {
		stmts, err := l.operation(op)
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	return out, nil
}

func (l *lowerer) operation(op ast.Operation) ([]string, error) {
	switch op := op.(type) {
	case *ast.CreateTable:
		name := op.Name.Name()
		stmts, err := shapeCreate(name, op.Body)
		if err != nil {
			return nil, err
		}
		l.tables[name] = shapeColumns(op.Body)
		return stmts, nil
	case *ast.DropTable:
		name := op.Name.Name()
		delete(l.tables, name)
		return []string{"DROP TABLE " + sqlite.Ident(name)}, nil
	case *ast.RenameTable:
		from, to := op.From.Name(), op.To.Name()
		if cols, known := l.tables[from]; known {
			delete(l.tables, from)
			l.tables[to] = cols
		}
		return []string{"ALTER TABLE " + sqlite.Ident(from) + " RENAME TO " + sqlite.Ident(to)}, nil
	case *ast.AlterTable:
		return l.alter(op)
	case *ast.RebuildTable:
		return l.rebuild(op)
	case *ast.CreateIndex:
		return indexCreate(op.Table.Name(), op.Index)
	case *ast.DropIndex:
		return indexDrop(op.Table.Name(), op.Index)
	case *ast.Exec:
		return []string{op.SQL.Tok.Val}, nil
	}
	return nil, fmt.Errorf("unknown operation %T", op)
}

func (l *lowerer) alter(op *ast.AlterTable) ([]string, error) {
	table := op.Name.Name()
	cols, known := l.tables[table]
	var out []string
	for _, item := range op.Items {
		switch item := item.(type) {
		case *ast.AlterAdd:
			stmts, err := columnAdd(table, item.Column)
			if err != nil {
				return nil, err
			}
			out = append(out, stmts...)
			cols = append(cols, item.Column.Name.Name())
		case *ast.AlterDrop:
			out = append(out, "ALTER TABLE "+sqlite.Ident(table)+" DROP COLUMN "+sqlite.Ident(item.Name.Name()))
			cols = without(cols, item.Name.Name())
		case *ast.AlterRename:
			out = append(out, "ALTER TABLE "+sqlite.Ident(table)+" RENAME COLUMN "+sqlite.Ident(item.From.Name())+" TO "+sqlite.Ident(item.To.Name()))
			cols = append(without(cols, item.From.Name()), item.To.Name())
		}
	}
	if known {
		l.tables[table] = cols
	}
	return out, nil
}

// rebuild lowers a rebuild table to its five steps (spec §migrations
// rule 9): the new table, the copy, the drop, the rename and the
// body's indexes, named for the table as it stands after the rename.
// The source of a column of a table the migrations created is its
// from:, else the old column of its name, else its default:, else
// NULL, which is what a rowid alias the rebuild introduces reads. A
// table no create table made is of unknown shape, and every column of
// the body reads the old column of its name, since without a fold the
// lowering cannot tell which exist.
func (l *lowerer) rebuild(op *ast.RebuildTable) ([]string, error) {
	table := op.Name.Name()
	tmp := table + "__new"
	create, err := sqlite.TableDDL(tmp, op.Body)
	if err != nil {
		return nil, err
	}
	old, known := l.tables[table]
	var cols, sources []string
	for _, item := range op.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		name := col.Name.Name()
		cols = append(cols, sqlite.Ident(name))
		src := "NULL"
		switch {
		case col.Settings.Get("from") != nil:
			if fe, ok := col.Settings.Get("from").Value.(*ast.FuncExpr); ok {
				src = fe.Text()
			}
		case !known:
			src = sqlite.Ident(name)
		case contains(old, name):
			src = sqlite.Ident(name)
		case col.Settings.Get("default") != nil:
			d, err := sqlite.DefaultSQL(col.Settings.Get("default").Value)
			if err != nil {
				return nil, fmt.Errorf("rebuild table %s: column %q: %w", table, name, err)
			}
			src = d
		}
		sources = append(sources, src)
	}
	out := []string{
		create,
		"INSERT INTO " + sqlite.Ident(tmp) + " (" + strings.Join(cols, ", ") + ")\nSELECT " + strings.Join(sources, ", ") + "\nFROM " + sqlite.Ident(table),
		"DROP TABLE " + sqlite.Ident(table),
		"ALTER TABLE " + sqlite.Ident(tmp) + " RENAME TO " + sqlite.Ident(table),
	}
	indexes, err := sqlite.IndexesDDL(table, op.Body)
	if err != nil {
		return nil, err
	}
	l.tables[table] = shapeColumns(op.Body)
	return append(out, indexes...), nil
}

// inverse renders the statements that undo one operation of a flat
// body (spec §migrations rule 12).
func inverse(op ast.Operation) ([]string, error) {
	switch op := op.(type) {
	case *ast.CreateTable:
		return []string{"DROP TABLE " + sqlite.Ident(op.Name.Name())}, nil
	case *ast.DropTable:
		if !op.HasBody() {
			return nil, fmt.Errorf("drop table %s has no body and no inverse (§migrations)", op.Name.Name())
		}
		return shapeCreate(op.Name.Name(), op.Body)
	case *ast.RenameTable:
		return []string{"ALTER TABLE " + sqlite.Ident(op.To.Name()) + " RENAME TO " + sqlite.Ident(op.From.Name())}, nil
	case *ast.AlterTable:
		table := op.Name.Name()
		var out []string
		for i := len(op.Items) - 1; i >= 0; i-- {
			switch item := op.Items[i].(type) {
			case *ast.AlterAdd:
				if _, err := sqlite.ColumnDDL(item.Column); err != nil {
					return nil, err
				}
				if ix := check.ColumnIndex(item.Column); ix != nil {
					stmts, err := indexDrop(table, ix)
					if err != nil {
						return nil, err
					}
					out = append(out, stmts...)
				}
				out = append(out, "ALTER TABLE "+sqlite.Ident(table)+" DROP COLUMN "+sqlite.Ident(item.Column.Name.Name()))
			case *ast.AlterDrop:
				if item.Type == nil {
					return nil, fmt.Errorf("drop %s in alter table %s has no type and no inverse (§migrations)", item.Name.Name(), table)
				}
				stmts, err := columnAdd(table, &ast.Column{Name: item.Name, Type: item.Type, Settings: item.Settings})
				if err != nil {
					return nil, err
				}
				out = append(out, stmts...)
			case *ast.AlterRename:
				out = append(out, "ALTER TABLE "+sqlite.Ident(table)+" RENAME COLUMN "+sqlite.Ident(item.To.Name())+" TO "+sqlite.Ident(item.From.Name()))
			}
		}
		return out, nil
	case *ast.CreateIndex:
		return indexDrop(op.Table.Name(), op.Index)
	case *ast.DropIndex:
		return indexCreate(op.Table.Name(), op.Index)
	case *ast.Exec:
		down := op.DownSQL()
		if down == nil {
			return nil, fmt.Errorf("exec has no down: and no inverse (§migrations)")
		}
		return []string{down.Tok.Val}, nil
	case *ast.RebuildTable:
		return nil, fmt.Errorf("rebuild table %s has no inverse (§migrations)", op.Name.Name())
	}
	return nil, fmt.Errorf("unknown operation %T", op)
}

func shapeCreate(table string, body []ast.TableItem) ([]string, error) {
	create, err := sqlite.TableDDL(table, body)
	if err != nil {
		return nil, err
	}
	indexes, err := sqlite.IndexesDDL(table, body)
	if err != nil {
		return nil, err
	}
	return append([]string{create}, indexes...), nil
}

// columnAdd lowers an add item: ADD COLUMN, then the CREATE [UNIQUE]
// INDEX of a column that carries unique or index, which SQLite's ADD
// COLUMN takes no part of (spec §migrations rule 8).
func columnAdd(table string, col *ast.Column) ([]string, error) {
	def, err := sqlite.ColumnDDL(col)
	if err != nil {
		return nil, fmt.Errorf("alter table %s: %w", table, err)
	}
	out := []string{"ALTER TABLE " + sqlite.Ident(table) + " ADD COLUMN " + def}
	if ix := check.ColumnIndex(col); ix != nil {
		stmts, err := indexCreate(table, ix)
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	return out, nil
}

func indexCreate(table string, ix *ast.Index) ([]string, error) {
	name, err := sqlite.IndexName(table, ix)
	if err != nil {
		return nil, err
	}
	return []string{sqlite.IndexDDL(table, name, ix)}, nil
}

func indexDrop(table string, ix *ast.Index) ([]string, error) {
	name, err := sqlite.IndexName(table, ix)
	if err != nil {
		return nil, err
	}
	return []string{"DROP INDEX " + sqlite.Ident(name)}, nil
}

func shapeColumns(body []ast.TableItem) []string {
	var out []string
	for _, item := range body {
		if col, ok := item.(*ast.Column); ok {
			out = append(out, col.Name.Name())
		}
	}
	return out
}

func tablesClone(tables map[string][]string) map[string][]string {
	out := make(map[string][]string, len(tables))
	for k, v := range tables {
		out[k] = append([]string(nil), v...)
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

// SQLText renders migrations for review as volt migration sql prints
// them (spec §migration-tooling rule 1): each name, its up statements,
// then its down statements, or "-- irreversible".
func SQLText(ms []rt.Migration) string {
	var b strings.Builder
	for i, m := range ms {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "-- %s\n-- up\n", m.Name)
		statementsWrite(&b, m.Up)
		if m.Down == nil {
			b.WriteString("-- irreversible\n")
			continue
		}
		b.WriteString("-- down\n")
		statementsWrite(&b, m.Down)
	}
	return b.String()
}

func statementsWrite(b *strings.Builder, stmts []string) {
	for _, s := range stmts {
		b.WriteString(strings.TrimRight(strings.TrimSpace(s), ";"))
		b.WriteString(";\n")
	}
}
