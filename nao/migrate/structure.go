// Structure reading and the fold's comparison (spec §the-fold): what a
// SQLite database is, table by table, and the operations that would
// turn one such structure into another.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/nao/gen/sqlite"
)

// Schema is the structure of a database as the fold compares it (spec
// §the-fold rule 2): its tables, each with its columns as a set, its
// foreign keys and its indexes; never the ledger, never sqlite_*
// objects, never column order.
type Schema struct {
	Tables []Table // in name order
}

// Table is one table's structure (spec §the-fold rule 2).
type Table struct {
	Name        string
	Columns     []Column     // in declaration order; compared as a set
	ForeignKeys []ForeignKey // in declaration order; compared as a set
	Indexes     []Index      // in declaration order; compared by name or key
}

// Column is one column as PRAGMA table_xinfo reports it.
type Column struct {
	Name    string
	Type    string // the declared type as written
	NotNull bool
	Default string // the default as SQLite stores it; "" when none
	PK      int    // 1-based position in the primary key; 0 outside it
}

// ForeignKey is one foreign key as PRAGMA foreign_key_list reports it.
type ForeignKey struct {
	Table    string // the parent table
	From, To []string
	OnUpdate string
	OnDelete string
}

// Index is one index as PRAGMA index_list and index_xinfo report it.
// The automatic index of a UNIQUE constraint has no Name, since SQLite
// numbers those: it matches any unique index on its key.
type Index struct {
	Name   string
	Unique bool
	Where  string   // the partial predicate; "" for a whole index
	Key    []string // column names or expression text
}

// Table finds a table by name, or nil.
func (s Schema) Table(name string) *Table {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			return &s.Tables[i]
		}
	}
	return nil
}

// Column finds a column by name, or nil.
func (t *Table) Column(name string) *Column {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

// Read takes the structure of db (spec §the-fold rule 2), leaving out
// the ledger and SQLite's own objects.
func Read(ctx context.Context, db *sql.DB) (Schema, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite\_%' ESCAPE '\' AND name <> 'volt_migrations' ORDER BY name`)
	if err != nil {
		return Schema{}, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return Schema{}, err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Schema{}, err
	}
	var s Schema
	for _, name := range names {
		t, err := tableRead(ctx, db, name)
		if err != nil {
			return Schema{}, fmt.Errorf("table %s: %w", name, err)
		}
		s.Tables = append(s.Tables, t)
	}
	return s, nil
}

func tableRead(ctx context.Context, db *sql.DB, name string) (Table, error) {
	t := Table{Name: name}
	rows, err := db.QueryContext(ctx, "PRAGMA table_xinfo("+sqlite.Ident(name)+")")
	if err != nil {
		return t, err
	}
	for rows.Next() {
		var cid, notnull, pk, hidden int
		var col, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &col, &typ, &notnull, &dflt, &pk, &hidden); err != nil {
			rows.Close()
			return t, err
		}
		if hidden != 0 {
			continue
		}
		t.Columns = append(t.Columns, Column{Name: col, Type: typ, NotNull: notnull != 0, Default: dflt.String, PK: pk})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return t, err
	}
	if t.ForeignKeys, err = foreignKeysRead(ctx, db, name); err != nil {
		return t, err
	}
	if t.Indexes, err = indexesRead(ctx, db, name); err != nil {
		return t, err
	}
	return t, nil
}

func foreignKeysRead(ctx context.Context, db *sql.DB, table string) ([]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_list("+sqlite.Ident(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ForeignKey
	var ids []int
	byID := map[int]*ForeignKey{}
	for rows.Next() {
		var id, seq int
		var parent, from, onUpdate, onDelete, match string
		var to sql.NullString
		if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		fk, ok := byID[id]
		if !ok {
			fk = &ForeignKey{Table: parent, OnUpdate: onUpdate, OnDelete: onDelete}
			byID[id] = fk
			ids = append(ids, id)
		}
		fk.From = append(fk.From, from)
		fk.To = append(fk.To, to.String)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Ints(ids)
	for _, id := range ids {
		out = append(out, *byID[id])
	}
	return out, nil
}

func indexesRead(ctx context.Context, db *sql.DB, table string) ([]Index, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA index_list("+sqlite.Ident(table)+")")
	if err != nil {
		return nil, err
	}
	type listed struct {
		name, origin string
		unique       bool
		partial      bool
	}
	var list []listed
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, listed{name: name, origin: origin, unique: unique != 0, partial: partial != 0})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []Index
	for _, ix := range list {
		if ix.origin == "pk" {
			continue
		}
		key, expr, err := indexKeyRead(ctx, db, ix.name)
		if err != nil {
			return nil, err
		}
		idx := Index{Name: ix.name, Unique: ix.unique, Key: key}
		if ix.origin == "u" {
			idx.Name = ""
		}
		if expr || ix.partial {
			var text sql.NullString
			if err := db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?", ix.name).Scan(&text); err != nil {
				return nil, err
			}
			parsedKey, where := indexSQLParse(text.String)
			if expr {
				idx.Key = parsedKey
			}
			if ix.partial {
				idx.Where = where
			}
		}
		out = append(out, idx)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Name == "") != (out[j].Name == "") {
			return out[i].Name != ""
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return strings.Join(out[i].Key, ",") < strings.Join(out[j].Key, ",")
	})
	return out, nil
}

// indexKeyRead reads the key columns of an index and reports whether an
// expression is among them, in which case the key must come from the
// index's SQL.
func indexKeyRead(ctx context.Context, db *sql.DB, index string) (key []string, expr bool, err error) {
	rows, err := db.QueryContext(ctx, "PRAGMA index_xinfo("+sqlite.Ident(index)+")")
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var seqno, cid, desc, inKey int
		var name sql.NullString
		var coll string
		if err := rows.Scan(&seqno, &cid, &name, &desc, &coll, &inKey); err != nil {
			return nil, false, err
		}
		if inKey == 0 {
			continue
		}
		if cid == -2 || !name.Valid {
			expr = true
		}
		key = append(key, name.String)
	}
	return key, expr, rows.Err()
}

var whereClause = regexp.MustCompile(`(?i)\bWHERE\b`)

// indexSQLParse splits a CREATE INDEX statement into its key atoms,
// identifiers unquoted and expressions as written, and its WHERE
// predicate.
func indexSQLParse(stmt string) (key []string, where string) {
	depth, start := 0, -1
	for i := 0; i < len(stmt); i++ {
		switch c := stmt[i]; c {
		case '\'', '"', '`', '[':
			i = quotedEnd(stmt, i)
		case '(':
			if depth == 0 {
				start = i
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				key = atomsSplit(stmt[start+1 : i])
				rest := stmt[i+1:]
				if loc := whereClause.FindStringIndex(rest); loc != nil {
					where = spaceCollapse(rest[loc[1]:])
				}
				return key, where
			}
		}
	}
	return key, where
}

// quotedEnd returns the index of the closing quote of the quoted run
// that opens at i, doubled closing quotes included.
func quotedEnd(s string, i int) int {
	open := s[i]
	close := open
	if open == '[' {
		close = ']'
	}
	for j := i + 1; j < len(s); j++ {
		if s[j] != close {
			continue
		}
		if open != '[' && j+1 < len(s) && s[j+1] == close {
			j++
			continue
		}
		return j
	}
	return len(s) - 1
}

func atomsSplit(inner string) []string {
	var atoms []string
	depth, start := 0, 0
	for i := 0; i < len(inner); i++ {
		switch c := inner[i]; c {
		case '\'', '"', '`', '[':
			i = quotedEnd(inner, i)
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				atoms = append(atoms, atomNormalize(inner[start:i]))
				start = i + 1
			}
		}
	}
	return append(atoms, atomNormalize(inner[start:]))
}

func atomNormalize(atom string) string {
	atom = strings.TrimSpace(atom)
	for strings.HasPrefix(atom, "(") && parenMatches(atom) {
		atom = strings.TrimSpace(atom[1 : len(atom)-1])
	}
	if len(atom) >= 2 && atom[0] == '"' && atom[len(atom)-1] == '"' {
		return strings.ReplaceAll(atom[1:len(atom)-1], `""`, `"`)
	}
	return spaceCollapse(atom)
}

// parenMatches reports whether the opening parenthesis of atom closes at
// its last byte.
func parenMatches(atom string) bool {
	depth := 0
	for i := 0; i < len(atom); i++ {
		switch atom[i] {
		case '\'', '"', '`', '[':
			i = quotedEnd(atom, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(atom)-1
			}
		}
	}
	return false
}

func spaceCollapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// Drift is one difference between the fold and the target (spec
// §the-fold rule 3): the operation that would close it, as Volt text,
// and the reason.
type Drift struct {
	Table  string
	Volt   string
	Reason string
}

// Diff compares the fold with the target and returns the operations
// that would make the fold the target, table by table in name order
// (spec §the-fold rule 3). A missing or extra table is a create or a
// drop; a column only on one side is an add or a drop; a column that
// differs, a foreign key that differs or a unique constraint the target
// lacks is a rebuild of the whole table; an index only on one side or
// changed is a create or a drop.
func Diff(target, fold Schema) []Drift {
	names := map[string]bool{}
	for _, t := range target.Tables {
		names[t.Name] = true
	}
	for _, t := range fold.Tables {
		names[t.Name] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	var out []Drift
	for _, name := range sorted {
		tt, ft := target.Table(name), fold.Table(name)
		switch {
		case ft == nil:
			out = append(out, Drift{Table: name, Volt: "create table " + name + " {\n" + shapeVolt(tt) + "}", Reason: fmt.Sprintf("table %s is in the schema but not in the fold", name)})
		case tt == nil:
			out = append(out, Drift{Table: name, Volt: "drop table " + name, Reason: fmt.Sprintf("table %s is in the fold but not in the schema", name)})
		default:
			out = append(out, tableDiff(tt, ft)...)
		}
	}
	return out
}

func tableDiff(target, fold *Table) []Drift {
	var reasons []string
	var adds, drops []Column
	for _, tc := range target.Columns {
		fc := fold.Column(tc.Name)
		if fc == nil {
			adds = append(adds, tc)
			continue
		}
		reasons = append(reasons, columnDiff(target.Name, tc, *fc)...)
	}
	for _, fc := range fold.Columns {
		if target.Column(fc.Name) == nil {
			drops = append(drops, fc)
		}
	}
	reasons = append(reasons, foreignKeysDiff(target, fold)...)
	creates, dropsIx, constraintReasons := indexesDiff(target, fold)
	reasons = append(reasons, constraintReasons...)
	if len(reasons) > 0 {
		for _, c := range adds {
			reasons = append(reasons, fmt.Sprintf("column %s.%s is in the schema but not in the fold", target.Name, c.Name))
		}
		for _, c := range drops {
			reasons = append(reasons, fmt.Sprintf("column %s.%s is in the fold but not in the schema", target.Name, c.Name))
		}
		return []Drift{{Table: target.Name, Volt: "rebuild table " + target.Name + " {\n" + shapeVolt(target) + "}", Reason: strings.Join(reasons, "; ")}}
	}
	var out []Drift
	for _, c := range adds {
		out = append(out, Drift{
			Table:  target.Name,
			Volt:   "alter table " + target.Name + " {\n  add " + columnVolt(target, c) + "\n}",
			Reason: fmt.Sprintf("column %s.%s is in the schema but not in the fold", target.Name, c.Name),
		})
	}
	for _, c := range drops {
		out = append(out, Drift{
			Table:  target.Name,
			Volt:   "alter table " + target.Name + " {\n  drop " + c.Name + "\n}",
			Reason: fmt.Sprintf("column %s.%s is in the fold but not in the schema", target.Name, c.Name),
		})
	}
	for _, ix := range dropsIx {
		out = append(out, Drift{
			Table:  target.Name,
			Volt:   "drop index " + target.Name + " " + indexVolt(target.Name, ix),
			Reason: fmt.Sprintf("index %s on %s is in the fold but not in the schema", indexDescribe(ix), target.Name),
		})
	}
	for _, ix := range creates {
		if ix.Name == "" && ix.Where == "" && len(ix.Key) == 1 && columnIn(ix.Key[0], adds) {
			continue
		}
		out = append(out, Drift{
			Table:  target.Name,
			Volt:   "create index " + target.Name + " " + indexVolt(target.Name, ix),
			Reason: fmt.Sprintf("index %s on %s is in the schema but not in the fold", indexDescribe(ix), target.Name),
		})
	}
	return out
}

func columnIn(name string, cols []Column) bool {
	for _, c := range cols {
		if c.Name == name {
			return true
		}
	}
	return false
}

func columnDiff(table string, target, fold Column) []string {
	var out []string
	at := "column " + table + "." + target.Name
	if !strings.EqualFold(target.Type, fold.Type) {
		out = append(out, fmt.Sprintf("%s is %s in the schema, %s in the fold", at, target.Type, fold.Type))
	}
	if target.NotNull != fold.NotNull {
		out = append(out, fmt.Sprintf("%s is %s in the schema, %s in the fold", at, nullness(target.NotNull), nullness(fold.NotNull)))
	}
	if target.Default != fold.Default {
		out = append(out, fmt.Sprintf("%s defaults to %s in the schema, %s in the fold", at, defaultDescribe(target.Default), defaultDescribe(fold.Default)))
	}
	if (target.PK != 0) != (fold.PK != 0) {
		out = append(out, fmt.Sprintf("%s is %s in the schema, %s in the fold", at, pkness(target.PK), pkness(fold.PK)))
	}
	return out
}

func nullness(notNull bool) string {
	if notNull {
		return "not null"
	}
	return "nullable"
}

func pkness(pk int) string {
	if pk != 0 {
		return "in the primary key"
	}
	return "outside the primary key"
}

func defaultDescribe(d string) string {
	if d == "" {
		return "nothing"
	}
	return d
}

func foreignKeysDiff(target, fold *Table) []string {
	var out []string
	for _, fk := range target.ForeignKeys {
		if !foreignKeyIn(fk, fold.ForeignKeys) {
			out = append(out, fmt.Sprintf("foreign key %s is in the schema but not in the fold", foreignKeyDescribe(target.Name, fk)))
		}
	}
	for _, fk := range fold.ForeignKeys {
		if !foreignKeyIn(fk, target.ForeignKeys) {
			out = append(out, fmt.Sprintf("foreign key %s is in the fold but not in the schema", foreignKeyDescribe(fold.Name, fk)))
		}
	}
	return out
}

func foreignKeyIn(fk ForeignKey, list []ForeignKey) bool {
	for _, other := range list {
		if foreignKeyEqual(fk, other) {
			return true
		}
	}
	return false
}

func foreignKeyEqual(a, b ForeignKey) bool {
	return a.Table == b.Table && strings.Join(a.From, ",") == strings.Join(b.From, ",") &&
		strings.Join(a.To, ",") == strings.Join(b.To, ",") &&
		strings.EqualFold(a.OnUpdate, b.OnUpdate) && strings.EqualFold(a.OnDelete, b.OnDelete)
}

func foreignKeyDescribe(table string, fk ForeignKey) string {
	return fmt.Sprintf("%s (%s) references %s (%s)", table, strings.Join(fk.From, ", "), fk.Table, strings.Join(fk.To, ", "))
}

// indexesDiff matches the indexes of two tables, named ones by name and
// a unique constraint's unnamed index by its key against any unique
// index of the same key, and returns what the fold lacks, what it has
// too many of, and the unique constraints the fold has that no
// operation but a rebuild removes.
func indexesDiff(target, fold *Table) (creates, drops []Index, constraintReasons []string) {
	tUsed := make([]bool, len(target.Indexes))
	fUsed := make([]bool, len(fold.Indexes))
	for i, ti := range target.Indexes {
		if ti.Name == "" {
			continue
		}
		for j, fi := range fold.Indexes {
			if fUsed[j] || fi.Name != ti.Name {
				continue
			}
			tUsed[i], fUsed[j] = true, true
			if !indexEqual(ti, fi) {
				drops = append(drops, fi)
				creates = append(creates, ti)
			}
			break
		}
	}
	for i, ti := range target.Indexes {
		if tUsed[i] {
			continue
		}
		for j, fi := range fold.Indexes {
			if fUsed[j] || (ti.Name != "" && fi.Name != "") {
				continue
			}
			if ti.Unique && fi.Unique && ti.Where == "" && fi.Where == "" && keyEqual(ti.Key, fi.Key) {
				tUsed[i], fUsed[j] = true, true
				break
			}
		}
	}
	for i, ti := range target.Indexes {
		if !tUsed[i] {
			creates = append(creates, ti)
		}
	}
	for j, fi := range fold.Indexes {
		if fUsed[j] {
			continue
		}
		if fi.Name == "" {
			constraintReasons = append(constraintReasons, fmt.Sprintf("unique constraint on %s (%s) is in the fold but not in the schema", fold.Name, strings.Join(fi.Key, ", ")))
			continue
		}
		drops = append(drops, fi)
	}
	return creates, drops, constraintReasons
}

func indexEqual(a, b Index) bool {
	return a.Unique == b.Unique && a.Where == b.Where && keyEqual(a.Key, b.Key)
}

func keyEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexDescribe(ix Index) string {
	if ix.Name != "" {
		return ix.Name
	}
	return "(" + strings.Join(ix.Key, ", ") + ")"
}

// shapeVolt renders a table's columns and indexes block as the body of
// a create table or rebuild table operation (spec §migrations), one
// line per column, two-space indented.
func shapeVolt(t *Table) string {
	var b strings.Builder
	for _, c := range t.Columns {
		b.WriteString("  " + columnVolt(t, c) + "\n")
	}
	var lines []string
	if pk := pkColumns(t); len(pk) > 1 {
		lines = append(lines, "("+strings.Join(pk, ", ")+") [pk]")
	}
	for _, ix := range t.Indexes {
		if ix.Name == "" && len(ix.Key) == 1 && ix.Where == "" {
			continue
		}
		lines = append(lines, indexVolt(t.Name, ix))
	}
	for _, fk := range t.ForeignKeys {
		if len(fk.From) > 1 {
			lines = append(lines, "// foreign key ("+strings.Join(fk.From, ", ")+") references "+fk.Table+" ("+strings.Join(fk.To, ", ")+"): write it as a Ref element")
		}
	}
	if len(lines) > 0 {
		b.WriteString("\n  indexes {\n")
		for _, l := range lines {
			b.WriteString("    " + l + "\n")
		}
		b.WriteString("  }\n")
	}
	return b.String()
}

func pkColumns(t *Table) []string {
	var cols []Column
	for _, c := range t.Columns {
		if c.PK > 0 {
			cols = append(cols, c)
		}
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].PK < cols[j].PK })
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}

// columnVolt renders one column as a migration writes it (spec
// §migrations rule 3): the built-in type, pk and increment for a rowid
// alias, not null, unique for a one-column unique constraint, default:
// and ref: for a one-column foreign key.
func columnVolt(t *Table, c Column) string {
	typ := sqlite.TypeVolt(c.Type)
	if typ == "" {
		typ = c.Type
	}
	single := len(pkColumns(t)) == 1
	var settings []string
	switch {
	case single && c.PK == 1:
		settings = append(settings, "pk")
		if strings.EqualFold(c.Type, "INTEGER") && !c.NotNull {
			settings = append(settings, "increment")
		}
	case c.NotNull:
		settings = append(settings, "not null")
	}
	for _, ix := range t.Indexes {
		if ix.Name == "" && ix.Where == "" && len(ix.Key) == 1 && ix.Key[0] == c.Name {
			settings = append(settings, "unique")
			break
		}
	}
	if c.Default != "" {
		settings = append(settings, "default: "+defaultVolt(c.Default))
	}
	for _, fk := range t.ForeignKeys {
		if len(fk.From) == 1 && fk.From[0] == c.Name {
			settings = append(settings, "ref: > "+fk.Table+"."+fk.To[0])
		}
	}
	if len(settings) == 0 {
		return c.Name + " " + typ
	}
	return c.Name + " " + typ + " [" + strings.Join(settings, ", ") + "]"
}

// defaultVolt renders a stored default as a default: value: a SQL
// string as a Volt string, a number as itself, anything else as an
// expression literal.
func defaultVolt(d string) string {
	switch {
	case len(d) >= 2 && d[0] == '\'' && d[len(d)-1] == '\'':
		return "'" + strings.ReplaceAll(strings.ReplaceAll(d[1:len(d)-1], "''", "'"), "'", `\'`) + "'"
	case strings.EqualFold(d, "NULL"):
		return "null"
	case strings.HasPrefix(d, "(") && strings.HasSuffix(d, ")") && parenMatches(d):
		return "`" + strings.TrimSpace(d[1:len(d)-1]) + "`"
	}
	if _, err := strconv.ParseFloat(d, 64); err == nil {
		return d
	}
	return "`" + d + "`"
}

// indexVolt renders an index as an indexes block line or the tail of a
// create index operation (spec §migrations rule 10): the key, then
// unique, where: and a name: that the key does not derive.
func indexVolt(table string, ix Index) string {
	atoms := make([]string, len(ix.Key))
	derived := []string{"idx", table}
	plain := true
	for i, k := range ix.Key {
		if identLike(k) {
			atoms[i] = k
			derived = append(derived, k)
			continue
		}
		atoms[i] = "`" + k + "`"
		plain = false
	}
	key := atoms[0]
	if len(atoms) > 1 || !plain {
		key = "(" + strings.Join(atoms, ", ") + ")"
	}
	var settings []string
	if ix.Unique {
		settings = append(settings, "unique")
	}
	if ix.Where != "" {
		settings = append(settings, "where: `"+ix.Where+"`")
	}
	if ix.Name != "" && (!plain || ix.Name != strings.Join(derived, "_")) {
		settings = append(settings, "name: '"+strings.ReplaceAll(ix.Name, "'", `\'`)+"'")
	}
	if len(settings) == 0 {
		return key
	}
	return key + " [" + strings.Join(settings, ", ") + "]"
}

func identLike(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || i > 0 && '0' <= c && c <= '9' {
			continue
		}
		return false
	}
	return true
}
