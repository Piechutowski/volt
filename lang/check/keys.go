// Keys and indexes (spec §columns, §indexes): the primary key is the
// columns that carry pk, in the table's column order; a single-column
// index is the column setting unique or index; an Indexes block holds
// composite, expression and partial indexes; no two indexes of a table
// share a name.
package check

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
)

// PK returns the columns of the table's primary key: the columns that
// carry pk, in the table's column order, an injected column at its
// injected position (spec §columns); nil for a table without one.
func (t *TableInfo) PK() []*ColumnDef {
	var out []*ColumnDef
	for _, cd := range t.Columns {
		if cd.Col.Settings.Get("pk") != nil {
			out = append(out, cd)
		}
	}
	return out
}

// ColumnsUnique reports whether the named columns are unique together
// (spec §indexes, §relationships-ref): they are exactly the primary
// key's columns, or one column that carries unique, or exactly the key
// of a unique index whose key is columns only and which has no where:.
// A partial or an expression unique index makes no column unique.
func (t *TableInfo) ColumnsUnique(names []string) bool {
	if len(names) == 0 {
		return false
	}
	var pk []string
	for _, cd := range t.PK() {
		pk = append(pk, cd.Col.Name.Name())
	}
	if sameNames(pk, names) {
		return true
	}
	if len(names) == 1 {
		if cd := t.Column(names[0]); cd != nil && cd.Col.Settings.Get("unique") != nil {
			return true
		}
	}
	for _, ix := range t.Indexes {
		if ix.Settings.Get("unique") == nil || ix.Settings.Get("where") != nil {
			continue
		}
		if key, ok := indexColumns(ix); ok && sameNames(key, names) {
			return true
		}
	}
	return false
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, n := range a {
		seen[n]++
	}
	for _, n := range b {
		if seen[n] == 0 {
			return false
		}
		seen[n]--
	}
	return true
}

// indexColumns returns the column names of an index key, false when an
// expression is among its atoms.
func indexColumns(ix *ast.Index) ([]string, bool) {
	out := make([]string, 0, len(ix.Key))
	for _, atom := range ix.Key {
		id, ok := atom.(*ast.Ident)
		if !ok {
			return nil, false
		}
		out = append(out, id.Name())
	}
	return out, true
}

// ColumnIndex returns the index a column's unique or index setting
// declares, keyed by the column alone and unique for unique, or nil
// when the column carries neither (spec §indexes).
func ColumnIndex(col *ast.Column) *ast.Index {
	if s := col.Settings.Get("unique"); s != nil {
		return &ast.Index{Key: []ast.Node{col.Name}, Settings: &ast.SettingList{Settings: []*ast.Setting{s}}}
	}
	if col.Settings.Get("index") != nil {
		return &ast.Index{Key: []ast.Node{col.Name}}
	}
	return nil
}

// IndexName is the SQLite name of an index of table (spec §indexes,
// §mapping-to-sqlite-ddl): its name: when given, else
// idx_<table>_<col>[_<col>...] over a key of columns; false for an
// expression key without name:.
func IndexName(table string, ix *ast.Index) (string, bool) {
	if s := ix.Settings.Get("name"); s != nil {
		if lit, ok := s.Value.(*ast.BasicLit); ok {
			return lit.Tok.Val, true
		}
		return "", false
	}
	cols, ok := indexColumns(ix)
	if !ok {
		return "", false
	}
	return "idx_" + table + "_" + strings.Join(cols, "_"), true
}

// indexPlainCheck enforces that an index of an Indexes block is
// composite, an expression or partial: a plain single-column index is
// the column setting unique or index (spec §indexes).
func (c *checker) indexPlainCheck(table string, ix *ast.Index) {
	if len(ix.Key) != 1 || ix.Settings.Get("where") != nil {
		return
	}
	id, ok := ix.Key[0].(*ast.Ident)
	if !ok {
		return
	}
	setting := "index"
	if ix.Settings.Get("unique") != nil {
		setting = "unique"
	}
	c.errorf(ix.Pos(), "indexes", "index %s on %q is a plain single-column index: write the column setting %s on %q; an Indexes block holds composite, expression and partial indexes (§indexes)", indexKeyString(ix), table, setting, id.Name())
}

// namedIndex is one index of a table as its name is checked: the index,
// the column whose setting declares it (nil for an Indexes entry), and
// the partial that brought it (nil for the body's own).
type namedIndex struct {
	ix      *ast.Index
	col     *ast.Column
	partial *ast.TablePartial
}

func (n namedIndex) describe() string {
	if n.col != nil {
		return fmt.Sprintf("the %s setting of column %q", columnIndexSetting(n.col).Name, n.col.Name.Name())
	}
	return "index " + indexKeyString(n.ix)
}

// indexNamesCheck enforces that no two indexes of a table share a name,
// derived or written, the indexes of its columns' unique and index
// settings among them (spec §indexes). Two indexes of one partial are
// that partial's to report (partialIndexNamesCheck), and two indexes on
// one key of which a partial injected one are the error of
// §tablepartial; neither is reported again here.
func (c *checker) indexNamesCheck(table string, cols []*ColumnDef, indexes []namedIndex) {
	all := make([]namedIndex, 0, len(cols)+len(indexes))
	for _, cd := range cols {
		if ix := ColumnIndex(cd.Col); ix != nil {
			all = append(all, namedIndex{ix: ix, col: cd.Col, partial: cd.Partial})
		}
	}
	c.indexNamesDistinct(append(all, indexes...), nameScope{table: table})
}

// partialIndexNamesCheck enforces the same of a TablePartial's own
// indexes, which share a name in every table that injects them when
// they have one key and no name:, or one name: (spec §indexes,
// §tablepartial).
func (c *checker) partialIndexNamesCheck(partial string, cols []*ast.Column, indexes []*ast.Index) {
	all := make([]namedIndex, 0, len(cols)+len(indexes))
	for _, col := range cols {
		if ix := ColumnIndex(col); ix != nil {
			all = append(all, namedIndex{ix: ix, col: col})
		}
	}
	for _, ix := range indexes {
		all = append(all, namedIndex{ix: ix})
	}
	c.indexNamesDistinct(all, nameScope{table: "<table>", partial: partial})
}

// nameScope is where index names are checked: a table, or the own
// indexes of a partial, whose names derive for every table that
// injects it.
type nameScope struct {
	table   string
	partial string
}

func (s nameScope) subject(name string) string {
	if s.partial != "" {
		return fmt.Sprintf("TablePartial %q declares two indexes named %q in every table that injects it", s.partial, name)
	}
	return fmt.Sprintf("table %q has two indexes named %q", s.table, name)
}

// reported says whether a pair of a table's indexes of one name is
// another check's to report: two of one partial, or two on one key of
// which a partial injected one.
func (s nameScope) reported(a, b namedIndex) bool {
	if s.partial != "" {
		return false
	}
	if a.partial != nil && a.partial == b.partial {
		return true
	}
	return (a.partial != nil || b.partial != nil) && a.col == nil && b.col == nil && indexAtoms(a.ix) == indexAtoms(b.ix)
}

func (c *checker) indexNamesDistinct(all []namedIndex, s nameScope) {
	seen := map[string]namedIndex{}
	for _, n := range all {
		name, ok := IndexName(s.table, n.ix)
		if !ok {
			continue
		}
		first, dup := seen[name]
		if !dup {
			seen[name] = n
			continue
		}
		if s.reported(first, n) {
			continue
		}
		pos := n.ix.Pos()
		if n.col != nil {
			pos = columnIndexSetting(n.col).Pos()
		}
		c.errorf(pos, "indexes", "%s, %s and %s: an index name is unique within its table; give one a name: (§indexes)", s.subject(name), first.describe(), n.describe())
	}
}

func columnIndexSetting(col *ast.Column) *ast.Setting {
	if s := col.Settings.Get("unique"); s != nil {
		return s
	}
	return col.Settings.Get("index")
}
