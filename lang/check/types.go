package check

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
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
	"[]byte":    {Go: "[]byte", Storage: "[]byte"},
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
		c.errorf(d.Name.Pos(), "types", "Type %q spells a built-in type (§types)", key)
		return nil
	}
	if _, dup := c.types[key]; dup {
		c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate Type name %q", key)
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
		c.errorf(d.Name.Pos(), "types", "Type %q names a package but has no from path (§types)", key)
		return nil
	case d.Name.Qualifier() == "" && d.From != nil:
		c.errorf(d.From.Pos(), "types", "Type %q is a type of this package and takes no from path (§types)", key)
		return nil
	}
	if base := d.Name.Base(); !exported(base) {
		if d.Name.Qualifier() == "" {
			c.errorf(d.Name.Pos(), "types", "Type %q is not an exported Go name; a declared type's name begins with an upper-case letter, as %s does (§types)", key, exportedSpelling(base))
		} else {
			c.errorf(d.Name.Pos(), "types", "Type %q names an unexported Go type of package %s, which no other package can name (§types)", key, d.Name.Qualifier())
		}
		return nil
	}
	if d.From != nil {
		path := d.From.Tok.Val
		if path == "" {
			c.errorf(d.From.Pos(), "types", "Type %q has an empty from path; the path is a non-empty import path (§types)", key)
			return nil
		}
		alias := d.Name.Qualifier()
		if std, reserved := ReservedAliases[alias]; reserved && std != path {
			if std == "" {
				c.errorf(d.Name.Pos(), "types", "Type %q: package name %q is reserved by generated code (§types)", key, alias)
			} else {
				c.errorf(d.Name.Pos(), "types", "Type %q: package name %q is reserved by generated code for %q (§types)", key, alias, std)
			}
			return nil
		}
		ti.Path = path
	}
	if d.From == nil && c.goTypes != nil && KindedStorage[storage.Storage] && c.goTypes[key] == storage.Storage {
		ti.Kinded = true
	}
	return ti
}

// pkIndexColumns names the columns of the composite primary key, which
// never carry null (spec §columns, §indexes).
func pkIndexColumns(indexes []*ast.Index) map[string]bool {
	out := map[string]bool{}
	for _, ix := range indexes {
		if ix.Settings.Get("pk") == nil {
			continue
		}
		for _, k := range ix.Key {
			if id, ok := k.(*ast.Ident); ok {
				out[id.Name()] = true
			}
		}
	}
	return out
}

func (c *checker) typesResolve(ti *TableInfo) {
	pkIndex := pkIndexColumns(ti.Indexes)
	for _, cd := range ti.Columns {
		cd.Nullable = isNullable(cd.Col)
		c.pkIndexNullCheck(cd.Col, ti.Key, pkIndex)
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
	for _, cd := range ti.Columns {
		col := cd.Col
		rowid := col.Settings.Get("pk") != nil && col.Settings.Get("increment") != nil
		if listed[col.Name.Name()] || cd.Nullable || rowid || col.Settings.Get("default") != nil {
			continue
		}
		c.errorf(d.Pos(), "records-sample-data", "records of table %q do not list column %q, which is NOT NULL and has no default: every row would write NULL into it; list the column (§records-sample-data)", ti.Key, col.Name.Name())
	}
}

// valueTypeCheck is the one rule a default and a record value share (spec
// §default-values, §records-sample-data): the value fits the column's type.
func (c *checker) valueTypeCheck(cd *ColumnDef, v ast.Node, section, what string) {
	name := cd.Col.Name.Name()
	typ := cd.Col.Type.String()
	if _, expr := v.(*ast.FuncExpr); expr {
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
	}
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
		if x.Quoted() {
			return "value"
		}
		switch strings.ToLower(x.Name()) {
		case "true", "false":
			return "boolean"
		case "null":
			return "null"
		}
	}
	return "value"
}

func isNullable(col *ast.Column) bool { return col.Settings.Get("null") != nil }
