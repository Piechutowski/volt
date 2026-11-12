package check

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/rt"
)

// ColumnType is a column's resolved type (spec §columns): a built-in or
// a declared Type.
type ColumnType struct {
	Go      string
	Storage string
	Import  string
	Alias   string
	Decl    *TypeInfo
}

// Known reports whether the type resolved.
func (t ColumnType) Known() bool { return t.Storage != "" }

// Kinded reports a declared type whose Go declaration spells its storage
// type, which database/sql converts by kind (spec §types).
func (t ColumnType) Kinded() bool { return t.Decl != nil && t.Decl.Kinded }

// RequiredKind reports how the type spells "non-empty": "text" (not ”),
// "numeric" (not 0) or "bytes" (length > 0); false for a type with no
// empty value (spec §columns).
func (t ColumnType) RequiredKind() (string, bool) {
	switch t.Storage {
	case "string":
		return "text", true
	case "int", "float64":
		return "numeric", true
	case "[]byte":
		return "bytes", true
	}
	return "", false
}

// Class is the predicate type class of the type (spec §predicates):
// "numeric", "text", "time", "bool", "eq" for a declared type, "" for a
// type predicates refuse.
func (t ColumnType) Class() string {
	if t.Decl != nil {
		return "eq"
	}
	switch t.Storage {
	case "int", "float64":
		return "numeric"
	case "string":
		return "text"
	case "time.Time":
		return "time"
	case "bool":
		return "bool"
	}
	return ""
}

// StorageClass is the predicate class of the storage type, the class a
// literal must have against a declared type (spec §predicates).
func (t ColumnType) StorageClass() string {
	return ColumnType{Storage: t.Storage}.Class()
}

// TypeInfo is one Type declaration (spec §types).
type TypeInfo struct {
	Decl    *ast.TypeDecl
	Key     string
	Storage ColumnType
	Path    string
	// Kinded reports a bare type whose Go declaration spells its storage
	// type, bool, int, float64 or string: it needs neither Scanner nor
	// Valuer, and generated code asserts nothing for it (spec §types).
	Kinded bool
}

// KindedStorage is the set of storage types database/sql converts by
// kind, so a declared type spelling one as its underlying type needs no
// methods (spec §types).
var KindedStorage = map[string]bool{"bool": true, "int": true, "float64": true, "string": true}

// Builtins is the closed set of built-in column types (spec §columns).
var Builtins = []string{"bool", "int", "float64", "string", "[]byte", "time.Time"}

var builtins = map[string]ColumnType{
	"bool":      {Go: "bool", Storage: "bool"},
	"int":       {Go: "int", Storage: "int"},
	"float64":   {Go: "float64", Storage: "float64"},
	"string":    {Go: "string", Storage: "string"},
	"[]byte":    {Go: "rt.Bytes", Storage: "[]byte"},
	"time.Time": {Go: "time.Time", Storage: "time.Time", Import: "time"},
}

// Builtin resolves a built-in type by its spelling.
func Builtin(name string) (ColumnType, bool) {
	t, ok := builtins[name]
	return t, ok
}

type typeEnv struct {
	types  map[string]*TypeInfo
	failed map[string]bool
	keys   []string
}

func typeEnvOf(types map[string]*TypeInfo, failed map[string]bool) *typeEnv {
	keys := make([]string, 0, len(types)+len(failed))
	for k, t := range types {
		keys = append(keys, "type:"+k+"="+t.Storage.Storage+"="+t.Path+"="+strconv.FormatBool(t.Kinded))
	}
	for k := range failed {
		keys = append(keys, "failed:"+k)
	}
	sort.Strings(keys)
	return &typeEnv{types: types, failed: failed, keys: keys}
}

// resolve resolves a column type (spec §columns): a built-in, then a
// declared type.
func (e *typeEnv) resolve(tr *ast.TypeRef) ColumnType {
	name := tr.String()
	if b, ok := builtins[name]; ok {
		return b
	}
	if tr.Slice {
		return ColumnType{}
	}
	if ti := e.types[name]; ti != nil {
		return ColumnType{Go: name, Storage: ti.Storage.Storage, Import: ti.Path, Alias: typeAlias(name), Decl: ti}
	}
	return ColumnType{}
}

// ReservedAliases are the package names generated code imports for
// itself, each with the one import path a Type may bind it to; an empty
// path reserves the name outright (spec §types).
var ReservedAliases = map[string]string{
	"context": "context",
	"driver":  "database/sql/driver",
	"errors":  "errors",
	"fmt":     "fmt",
	"gob":     "encoding/gob",
	"http":    "net/http",
	"json":    "encoding/json",
	"rt":      "",
	"sql":     "database/sql",
	"strings": "strings",
	"time":    "time",
	"volt":    "",
}

func exported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func exportedSpelling(name string) string {
	for _, r := range name {
		if up := unicode.ToUpper(r); up != r {
			return string(up) + name[len(string(r)):]
		}
		break
	}
	return "X" + name
}

func importPathOK(path string) bool {
	const illegal = "!\"#$%&'()*,:;<=>?[\\]^`{|}\uFFFD"
	for _, r := range path {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegal, r) {
			return false
		}
	}
	return path != ""
}

func typeAlias(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 {
		return name[:i]
	}
	return ""
}

func (c *checker) typeDeclCheck(d *ast.TypeDecl) *TypeInfo {
	key := d.Name.String()
	ti := &TypeInfo{Decl: d, Key: key}
	if _, ok := builtins[key]; ok {
		c.errorf(d.Name.Pos(), "types", "type %q spells a built-in type (§types)", key)
		return nil
	}
	if _, dup := c.types[key]; dup {
		c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate type name %q", key)
		return nil
	}
	storage, ok := builtins[d.Storage.String()]
	if !ok {
		c.errorf(d.Storage.Pos(), "types", "storage type %q of Type %q is not a built-in: bool, int, float64, string, []byte or time.Time (§types)", d.Storage.String(), key)
		return nil
	}
	ti.Storage = storage
	switch {
	case d.Name.Qualifier() != "" && d.From == nil:
		c.errorf(d.Name.Pos(), "types", "type %q names a package but has no from path (§types)", key)
		return nil
	case d.Name.Qualifier() == "" && d.From != nil:
		c.errorf(d.From.Pos(), "types", "type %q is a type of this package and takes no from path (§types)", key)
		return nil
	}
	if base := d.Name.Base(); !exported(base) {
		if d.Name.Qualifier() == "" {
			c.errorf(d.Name.Pos(), "types", "type %q is not an exported Go name; a declared type's name begins with an upper-case letter, as %s does (§types)", key, exportedSpelling(base))
		} else {
			c.errorf(d.Name.Pos(), "types", "type %q names an unexported Go type of package %s, which no other package can name (§types)", key, d.Name.Qualifier())
		}
		return nil
	}
	if d.From != nil {
		path := d.From.Tok.Val
		if path == "" {
			c.errorf(d.From.Pos(), "types", "type %q has an empty from path; the path is a non-empty import path (§types)", key)
			return nil
		}
		if !importPathOK(path) {
			c.errorf(d.From.Pos(), "types", "type %q has from path %q, which is not a Go import path (§types)", key, path)
			return nil
		}
		alias := d.Name.Qualifier()
		if std, reserved := ReservedAliases[alias]; reserved && std != path {
			if std == "" {
				c.errorf(d.Name.Pos(), "types", "type %q: package name %q is reserved by generated code (§types)", key, alias)
			} else {
				c.errorf(d.Name.Pos(), "types", "type %q: package name %q is reserved by generated code for %q (§types)", key, alias, std)
			}
			return nil
		}
		if prev, bound := c.aliases[alias]; bound && prev.From.Tok.Val != path {
			c.errorf(d.Name.Pos(), "types", "type %q imports %q under the alias %s, which Type %q at %s binds to %q: one alias names one package (§types)",
				key, path, alias, prev.Name.String(), prev.Pos(), prev.From.Tok.Val)
			return nil
		}
		if _, bound := c.aliases[alias]; !bound {
			c.aliases[alias] = d
		}
		ti.Path = path
	}
	if d.From == nil && c.goTypes != nil && KindedStorage[storage.Storage] && c.goTypes[key] == storage.Storage {
		ti.Kinded = true
	}
	return ti
}

func (c *checker) typesResolve(ti *TableInfo) {
	for _, cd := range ti.Columns {
		cd.Nullable = isNullable(cd.Col)
		cd.Type = c.env.resolve(cd.Col.Type)
		if !cd.Type.Known() {
			if c.env.failed[cd.Col.Type.String()] {
				continue
			}
			c.errorf(cd.Col.Type.Pos(), "columns", "column %q: unknown type %q; a column type is bool, int, float64, string, []byte, time.Time or a declared Type (§columns)", cd.Col.Name.Name(), cd.Col.Type.String())
			continue
		}
		c.defaultTypeCheck(cd)
	}
}

// defaultTypeCheck enforces that a default fits the column's type (spec
// §default-values).
func (c *checker) defaultTypeCheck(cd *ColumnDef) {
	if s := cd.Col.Settings.Get("default"); s != nil && defaultKind(s.Value) != "null" {
		c.valueTypeCheck(cd, s.Value, "default-values", "default of")
	}
}

// recordsTypeCheck enforces that every record value fits its column's
// type (spec §records-sample-data).
func (c *checker) recordsTypeCheck(ti *TableInfo, d *ast.Records) {
	var cols []*ColumnDef
	if d.HasColumns {
		for _, id := range d.Columns {
			cols = append(cols, ti.Column(id.Name()))
		}
	} else {
		cols = ti.Columns
	}
	for _, row := range d.Rows {
		if len(row.Values) != len(cols) {
			continue
		}
		for i, v := range row.Values {
			if cols[i] == nil || !cols[i].Type.Known() {
				continue
			}
			c.valueTypeCheck(cols[i], v, "records-sample-data", "record value for")
		}
	}
}

func (c *checker) recordsCoverageCheck(ti *TableInfo, d *ast.Records) {
	if !d.HasColumns {
		return
	}
	listed := map[string]bool{}
	for _, id := range d.Columns {
		listed[id.Name()] = true
	}
	alias := rowidAliasInt(ti)
	for _, cd := range ti.Columns {
		col := cd.Col
		if listed[col.Name.Name()] || cd.Nullable || cd == alias || col.Settings.Get("default") != nil {
			continue
		}
		c.errorf(d.Pos(), "records-sample-data", "records of table %q do not list column %q, which is NOT NULL and has no default: only the database-assigned key leaves a column's value to the database, so every row gives it; list the column (§records-sample-data)", ti.Key, col.Name.Name())
	}
}

// valueTypeCheck is the one rule a default and a record value share (spec
// §default-values, §records-sample-data): the value fits the column's type.
func (c *checker) valueTypeCheck(cd *ColumnDef, v ast.Node, section, what string) {
	name := cd.Col.Name.Name()
	typ := cd.Col.Type.String()
	if expr, ok := v.(*ast.FuncExpr); ok {
		if cd.Type.Storage == "time.Time" {
			c.timeExprCheck(cd, v, expr.Text(), section, what)
		}
		return
	}
	kind := defaultKind(v)
	switch kind {
	case "value":
		return
	case "null":
		if !cd.Nullable {
			c.errorf(v.Pos(), section, "%s %q is null, but the column is not nullable (§%s)", what, name, section)
		}
		return
	}
	want, article := "", "a"
	switch {
	case cd.Type.Storage == "int":
		want, article = "integer", "an"
	case cd.Type.Storage == "float64":
		want = "number"
	case cd.Type.Storage == "string":
		want = "string"
	case cd.Type.Storage == "time.Time" && section == "records-sample-data":
		want = "string"
	case cd.Type.Storage == "bool":
		want = "boolean"
	default:
		want, article = "expression literal", "an"
	}
	ok := kind == want || (want == "number" && kind == "integer")
	if !ok {
		c.errorf(v.Pos(), section, "%s %q (%s) must be %s %s, found %s (§%s)", what, name, typ, article, want, kind, section)
		return
	}
	if lit, isLit := v.(*ast.BasicLit); isLit && cd.Type.Storage == "time.Time" {
		c.timeTextCheck(cd, v, lit.Tok.Val, section, what, "")
	}
}

// timeTextCheck enforces that time text a default or a record writes is
// the one stored form of a time.Time (spec §go-types), so that a value
// read back from the row compares equal to it.
func (c *checker) timeTextCheck(cd *ColumnDef, v ast.Node, text, section, what, sqlString string) {
	canon, ok := timeTextCanonical(text)
	if ok && canon == text {
		return
	}
	name, typ := cd.Col.Name.Name(), cd.Col.Type.String()
	if !ok {
		c.errorf(v.Pos(), section, "%s %q (%s) must be%s date/time text in SQLite's canonical UTC form, such as \"2024-01-15 10:30:00\", found %q (§%s)", what, name, typ, sqlString, text, section)
		return
	}
	c.errorf(v.Pos(), section, "%s %q (%s) must be%s SQLite's canonical UTC date/time text, %q, found %q (§%s)", what, name, typ, sqlString, canon, text, section)
}

// timeExprCheck enforces that an expression literal a time.Time default
// or record value writes is one SQLite evaluates to the stored form of a
// time.Time (spec §go-types): a SQL string of that text, CURRENT_TIMESTAMP,
// or datetime() without a modifier that changes its form or its zone.
func (c *checker) timeExprCheck(cd *ColumnDef, v ast.Node, expr, section, what string) {
	e := sqlUnparen(strings.TrimSpace(expr))
	if text, isString := sqlStringOf(e); isString {
		c.timeTextCheck(cd, v, text, section, what, " a SQL string of")
		return
	}
	if timeExprStored(e) {
		return
	}
	c.errorf(v.Pos(), section, "%s %q (%s) must be an expression SQLite evaluates to the stored form of a time.Time, `CURRENT_TIMESTAMP`, `NULL`, `datetime(...)` without the 'subsec', 'localtime' or 'utc' modifier, or a SQL string of SQLite's canonical UTC date/time text, found `%s` (§%s)", what, cd.Col.Name.Name(), cd.Col.Type.String(), strings.TrimSpace(expr), section)
}

// timeExprStored reports whether e, unparenthesized, is an expression
// SQLite evaluates to the stored form of a time.Time or to NULL:
// CURRENT_TIMESTAMP, NULL, or one datetime() call none of whose string
// arguments is the 'subsec' modifier, which writes three fractional
// digits, or 'localtime' or 'utc', which shift the instant by the
// server's zone (spec §default-values).
func timeExprStored(e string) bool {
	switch strings.ToUpper(e) {
	case "CURRENT_TIMESTAMP", "NULL":
		return true
	}
	const fn = "datetime"
	if len(e) < len(fn) || !strings.EqualFold(e[:len(fn)], fn) {
		return false
	}
	rest := strings.TrimLeft(e[len(fn):], " \t\n")
	if rest == "" || rest[0] != '(' || sqlParenEnd(rest) != len(rest)-1 {
		return false
	}
	for _, arg := range sqlStringsIn(rest) {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "subsec", "localtime", "utc":
			return false
		}
	}
	return true
}

// sqlUnparen is e without the parentheses that enclose all of it, which
// change nothing SQLite evaluates.
func sqlUnparen(e string) string {
	for len(e) > 1 && e[0] == '(' && sqlParenEnd(e) == len(e)-1 {
		e = strings.TrimSpace(e[1 : len(e)-1])
	}
	return e
}

// sqlParenEnd is the index of the parenthesis that closes the one e
// starts with, skipping SQL strings and quoted identifiers, or -1.
func sqlParenEnd(e string) int {
	depth := 0
	for i := 0; i < len(e); i++ {
		switch e[i] {
		case '\'', '"', '`':
			q := e[i]
			for i++; i < len(e) && e[i] != q; i++ {
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// sqlStringsIn is the value of every SQL string literal in e.
func sqlStringsIn(e string) []string {
	var out []string
	for i := 0; i < len(e); i++ {
		if e[i] != '\'' {
			continue
		}
		var b strings.Builder
		for i++; i < len(e); i++ {
			if e[i] == '\'' {
				if i+1 < len(e) && e[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				break
			}
			b.WriteByte(e[i])
		}
		out = append(out, b.String())
	}
	return out
}

// timeTextCanonical is the stored form of the instant s names (spec
// §go-types), the text rt.TimeText writes, and whether s names one in a
// layout SQLite or the driver reads.
func timeTextCanonical(s string) (string, bool) {
	var t rt.Time
	if err := t.Scan(s); err != nil {
		return "", false
	}
	canon, err := rt.TimeText(time.Time(t))
	return canon, err == nil
}

// sqlStringOf is the value of an expression literal that is one SQL
// string literal, such as '2024-01-15 10:30:00', and whether it is one.
func sqlStringOf(expr string) (string, bool) {
	if len(expr) < 2 || expr[0] != '\'' || expr[len(expr)-1] != '\'' {
		return "", false
	}
	inner := expr[1 : len(expr)-1]
	if strings.Contains(strings.ReplaceAll(inner, "''", ""), "'") {
		return "", false
	}
	return strings.ReplaceAll(inner, "''", "'"), true
}

func defaultKind(v ast.Node) string {
	switch x := v.(type) {
	case *ast.BasicLit:
		if x.Tok.Kind == token.STRING {
			return "string"
		}
		if strings.ContainsAny(x.Tok.Val, ".eE") {
			return "number"
		}
		return "integer"
	case *ast.NegNumber:
		if strings.ContainsAny(x.Num.Tok.Val, ".eE") {
			return "number"
		}
		return "integer"
	case *ast.Ident:
		switch x.Name() {
		case "true", "false":
			return "boolean"
		case "null":
			return "null"
		}
	}
	return "value"
}

func isNullable(col *ast.Column) bool { return col.Settings.Get("null") != nil }
