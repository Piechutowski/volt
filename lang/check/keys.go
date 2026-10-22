// Keys and indexes (spec §columns, §indexes): the primary key is the
// columns that carry pk, in the table's column order; a single-column
// index is the column setting unique or index; an Indexes block holds
// composite, expression and partial indexes; no two indexes of the
// schema share a name, and no index takes a table's.
package check

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
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
// the column whose setting declares it (nil for an Indexes entry), the
// partial that brought it (nil for the body's own), and where a
// diagnostic about it goes when that is not the index itself.
type namedIndex struct {
	ix      *ast.Index
	col     *ast.Column
	partial *ast.TablePartial
	pos     token.Position
}

func (n namedIndex) describe() string {
	if n.col != nil {
		return fmt.Sprintf("the %s setting of column %q", columnIndexSetting(n.col).Name, n.col.Name.Name())
	}
	return "index " + indexKeyString(n.ix)
}

func (n namedIndex) at() token.Position {
	if n.pos.IsValid() {
		return n.pos
	}
	if n.col != nil {
		return columnIndexSetting(n.col).Pos()
	}
	return n.ix.Pos()
}

// indexNamed reports whether an index of an Indexes block takes part in
// the name rule: one carrying pk is the error of §indexes rule 5 alone.
func indexNamed(ix *ast.Index) bool {
	return ix.Settings.Get("pk") == nil
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
		if indexNamed(ix) {
			all = append(all, namedIndex{ix: ix})
		}
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

func (s nameScope) subject(first, second string) string {
	names := namesQuote(first, second)
	if s.partial != "" {
		return fmt.Sprintf("TablePartial %q declares two indexes named %s in every table that injects it", s.partial, names)
	}
	return fmt.Sprintf("table %q has two indexes named %s", s.table, names)
}

func (s nameScope) advice(a, b namedIndex) string {
	if s.partial != "" {
		return "declare one of them in a table body instead, since a name: written in a TablePartial is the same in every table that injects it"
	}
	return nameAdvice(a, b)
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
	type entry struct {
		n    namedIndex
		name string
	}
	seen := map[string]entry{}
	for _, n := range all {
		name, ok := IndexName(s.table, n.ix)
		if !ok || name == "" {
			continue
		}
		key := sqliteFold(name)
		first, dup := seen[key]
		if !dup {
			seen[key] = entry{n, name}
			continue
		}
		if s.reported(first.n, n) {
			continue
		}
		c.errorf(n.at(), "indexes", "%s, %s and %s: SQLite names an index once in the database, so an index name is unique in the schema; %s (§indexes)", s.subject(first.name, name), first.n.describe(), n.describe(), s.advice(first.n, n))
	}
}

func namesQuote(first, second string) string {
	if second == first {
		return fmt.Sprintf("%q", first)
	}
	return fmt.Sprintf("%q and %q (one name to SQLite)", first, second)
}

// nameAdvice says how to part two indexes of one name: give an Indexes
// entry of a table body another name:, inject a partial that writes a
// name: into one table, change the name: a partial writes, or rename
// what a derived name derives from (spec §indexes).
func nameAdvice(a, b namedIndex) string {
	for _, n := range []namedIndex{b, a} {
		if n.col == nil && n.partial == nil {
			return "give " + n.describe() + " another name:"
		}
	}
	if a.partial != nil && a.partial == b.partial {
		return fmt.Sprintf("a name: written in a TablePartial is the same in every table that injects it, so TablePartial %q is injected into one table only", a.partial.Name.Name())
	}
	for _, n := range []namedIndex{b, a} {
		if n.partial != nil && n.col == nil && n.ix.Settings.Get("name") != nil {
			return fmt.Sprintf("change the name: TablePartial %q writes", n.partial.Name.Name())
		}
	}
	return "rename a table or a column, since no name: reaches a column's index"
}

// sqliteFold is a name as SQLite compares it: ASCII letters fold to
// lower case and every other character stands (spec §indexes).
func sqliteFold(name string) string {
	upper := false
	for i := 0; i < len(name); i++ {
		if 'A' <= name[i] && name[i] <= 'Z' {
			upper = true
			break
		}
	}
	if !upper {
		return name
	}
	b := []byte(name)
	for i, ch := range b {
		if 'A' <= ch && ch <= 'Z' {
			b[i] = ch + 'a' - 'A'
		}
	}
	return string(b)
}

// sitedIndex is one index of the schema as its name is checked across
// tables: the table it lands in and the index.
type sitedIndex struct {
	ti *TableInfo
	n  namedIndex
}

// sitedIndexes appends a table's indexes to dst as their names are
// checked across the schema: its columns' indexes, then its Indexes
// entries and those its partials inject in body order; a partial's
// index, and the index of a partial's column, sit at the injection that
// brought it (spec §indexes).
func (c *checker) sitedIndexes(dst []sitedIndex, ti *TableInfo) []sitedIndex {
	for _, cd := range ti.Columns {
		ix := ColumnIndex(cd.Col)
		if ix == nil {
			continue
		}
		n := namedIndex{ix: ix, col: cd.Col, partial: cd.Partial}
		if cd.Partial != nil {
			n.pos = injectionOf(ti, cd.Partial)
		}
		dst = append(dst, sitedIndex{ti, n})
	}
	for _, item := range ti.Decl.Body {
		switch item := item.(type) {
		case *ast.IndexesBlock:
			for _, ix := range item.Indexes {
				if indexNamed(ix) {
					dst = append(dst, sitedIndex{ti, namedIndex{ix: ix}})
				}
			}
		case *ast.PartialRef:
			pi := c.partials[item.Name.Name()]
			if pi == nil {
				continue
			}
			for _, pit := range pi.Decl.Body {
				block, ok := pit.(*ast.IndexesBlock)
				if !ok {
					continue
				}
				for _, ix := range block.Indexes {
					if indexNamed(ix) {
						dst = append(dst, sitedIndex{ti, namedIndex{ix: ix, partial: pi.Decl, pos: item.Pos()}})
					}
				}
			}
		}
	}
	return dst
}

// injectionOf is the position of the last injection of p in a table's
// body, the one whose columns stand (spec §tablepartial).
func injectionOf(ti *TableInfo, p *ast.TablePartial) token.Position {
	var pos token.Position
	for _, item := range ti.Decl.Body {
		if ref, ok := item.(*ast.PartialRef); ok && ref.Name.Name() == p.Name.Name() {
			pos = ref.Pos()
		}
	}
	return pos
}

// indexNamesSchemaCheck enforces that no two indexes of the schema share
// a name and that no index takes a table's name, names compared as
// SQLite compares them, since SQLite names indexes and tables in one
// namespace per database (spec §indexes). Two indexes of one table are
// that table's to report (indexNamesCheck), and a table whose index
// clashes with another table's is reported once per name.
func (c *checker) indexNamesSchemaCheck() {
	var all []sitedIndex
	for _, ti := range c.info.Tables {
		all = c.sitedIndexes(all, ti)
	}
	if len(all) == 0 {
		return
	}
	tables := make(map[string]*TableInfo, len(c.info.Tables))
	for _, ti := range c.info.Tables {
		tables[sqliteFold(ti.Key)] = ti
	}
	type owner struct {
		ti   *TableInfo
		n    namedIndex
		name string
	}
	seen := map[string]owner{}
	reported := map[[2]string]bool{}
	for _, s := range all {
		ti, n := s.ti, s.n
		name, ok := IndexName(ti.Key, n.ix)
		if !ok || name == "" {
			continue
		}
		key := sqliteFold(name)
		if t := tables[key]; t != nil {
			c.errorf(n.at(), "indexes", "%s of table %q is named %q, the name of table %q: SQLite names indexes and tables in one namespace, so an index takes no table's name; %s (§indexes)", schemaDescribe(n), ti.Key, name, t.Key, nameAdvice(n, n))
			continue
		}
		first, dup := seen[key]
		if !dup {
			seen[key] = owner{ti, n, name}
			continue
		}
		if first.ti == ti || reported[[2]string{key, ti.Key}] {
			continue
		}
		reported[[2]string{key, ti.Key}] = true
		which := schemaDescribe(first.n) + " and " + schemaDescribe(n)
		if first.n.ix == n.ix {
			which = schemaDescribe(n) + " in both"
		}
		c.errorf(n.at(), "indexes", "tables %q and %q each have an index named %s, %s: SQLite names an index once in the database, so an index name is unique in the schema; %s (§indexes)", first.ti.Key, ti.Key, namesQuote(first.name, name), which, nameAdvice(first.n, n))
	}
}

func schemaDescribe(n namedIndex) string {
	if n.partial != nil {
		return n.describe() + fmt.Sprintf(" of TablePartial %q", n.partial.Name.Name())
	}
	return n.describe()
}

func columnIndexSetting(col *ast.Column) *ast.Setting {
	if s := col.Settings.Get("unique"); s != nil {
		return s
	}
	return col.Settings.Get("index")
}
