package check

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

// ColumnType is a column's resolved type (spec §columns): a built-in, an
// enum, or a declared Type.
type ColumnType struct {
	Go      string
	Storage string
	Import  string
	Alias   string
	Enum    *EnumInfo
	Decl    *TypeInfo
}

// Known reports whether the type resolved.
func (t ColumnType) Known() bool { return t.Storage != "" || t.Enum != nil }

// Nilable reports whether the Go type's nil already expresses NULL, so a
// nullable column stays bare instead of rt.Null[T] (spec §mapping-to-go).
func (t ColumnType) Nilable() bool { return t.Storage == "[]byte" && t.Decl == nil }

// RequiredKind reports how the type spells "non-empty": "text" (not ”),
// "numeric" (not 0) or "bytes" (length > 0); false for a type with no
// empty value (spec §columns).
func (t ColumnType) RequiredKind() (string, bool) {
	if t.Enum != nil {
		return "text", true
	}
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
	if t.Enum != nil {
		return ""
	}
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
}

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
	enums map[string]*EnumInfo
	types map[string]*TypeInfo
	keys  []string
}

func typeEnvOf(enums map[string]*EnumInfo, types map[string]*TypeInfo) *typeEnv {
	keys := make([]string, 0, len(enums)+len(types))
	for k := range enums {
		keys = append(keys, "enum:"+k)
	}
	for k, t := range types {
		keys = append(keys, "type:"+k+"="+t.Storage.Storage+"="+t.Path)
	}
	sort.Strings(keys)
	return &typeEnv{enums: enums, types: types, keys: keys}
}

// resolve resolves a column type written in a table of tableSchema
// (spec §columns): a built-in, then a declared type, then an enum of the
// written schema, the public schema, or the table's own schema.
func (e *typeEnv) resolve(tr *ast.TypeRef, tableSchema string) ColumnType {
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
	if en := e.enums[canonKey(tr.Name)]; en != nil {
		return ColumnType{Storage: "string", Enum: en}
	}
	if tr.Name.Schema() == "" && tableSchema != "" {
		if en := e.enums[tableSchema+"."+tr.Name.Base()]; en != nil {
			return ColumnType{Storage: "string", Enum: en}
		}
	}
	return ColumnType{}
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
	if en := c.enums[canonKey(d.Name)]; en != nil {
		c.errorf(d.Name.Pos(), "types", "Type %q collides with the enum of that name (§types)", key)
		return nil
	}
	storage, ok := builtins[d.Storage.String()]
	if !ok {
		c.errorf(d.Storage.Pos(), "types", "storage type %q of Type %q is not a built-in: bool, int, float64, string, []byte or time.Time (§types)", d.Storage.String(), key)
		return nil
	}
	ti.Storage = storage
	switch {
	case d.Name.Schema() != "" && d.From == nil:
		c.errorf(d.Name.Pos(), "types", "Type %q names a package but has no from path (§types)", key)
		return nil
	case d.Name.Schema() == "" && d.From != nil:
		c.errorf(d.From.Pos(), "types", "Type %q is a type of this package and takes no from path (§types)", key)
		return nil
	}
	if d.From != nil {
		ti.Path = d.From.Tok.Val
	}
	return ti
}

func (c *checker) typesResolve(ti *TableInfo) {
	schema := ti.Decl.Name.Schema()
	for _, cd := range ti.Columns {
		cd.Type = c.env.resolve(cd.Col.Type, schema)
		if !cd.Type.Known() {
			c.errorf(cd.Col.Type.Pos(), "columns", "column %q: unknown type %q; a column type is bool, int, float64, string, []byte, time.Time, an enum or a declared Type (§columns)", cd.Col.Name.Name(), cd.Col.Type.String())
			continue
		}
		c.defaultTypeCheck(cd)
	}
}

// defaultTypeCheck enforces that a default fits the column's type (spec
// §default-values).
func (c *checker) defaultTypeCheck(cd *ColumnDef) {
	if s := cd.Col.Settings.Get("default"); s != nil {
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
		if !d.HasColumns && len(row.Values) != len(cols) {
			c.errorf(row.Pos(), "records-sample-data", "record row has %d values but %s has %d columns", len(row.Values), ti.Decl.Name.String(), len(cols))
			continue
		}
		for i, v := range row.Values {
			if i >= len(cols) || cols[i] == nil || !cols[i].Type.Known() {
				continue
			}
			c.valueTypeCheck(cols[i], v, "records-sample-data", "record value for")
		}
	}
}

// valueTypeCheck is the one rule a default and a record value share (spec
// §default-values, §records-sample-data): the value fits the column's type.
func (c *checker) valueTypeCheck(cd *ColumnDef, v ast.Node, section, what string) {
	name := cd.Col.Name.Name()
	typ := cd.Col.Type.String()
	switch x := v.(type) {
	case *ast.FuncExpr:
		return
	case *ast.EnumConst:
		if cd.Type.Enum == nil {
			c.errorf(x.Pos(), section, "%s %q (%s) is an enum constant but the column is not an enum (§%s)", what, name, typ, section)
			return
		}
		if x.Enum.Name() != cd.Type.Enum.Decl.Name.Base() {
			c.errorf(x.Pos(), section, "%s %q names enum %q, the column's enum is %q (§%s)", what, name, x.Enum.Name(), cd.Type.Enum.Decl.Name.Base(), section)
			return
		}
		for _, ev := range cd.Type.Enum.Decl.Values {
			if ev.Name.Name() == x.Value.Name() {
				return
			}
		}
		c.errorf(x.Pos(), section, "%s %q: %q is not a value of enum %q (§%s)", what, name, x.Value.Name(), cd.Type.Enum.Decl.Name.Base(), section)
		return
	}
	kind := defaultKind(v)
	switch kind {
	case "value":
		return
	case "null":
		if !isNullable(cd.Col) {
			c.errorf(v.Pos(), section, "%s %q is null, but the column is not nullable (§%s)", what, name, section)
		}
		return
	}
	want, article := "", "a"
	switch {
	case cd.Type.Enum != nil:
		want, article = "enum constant", "an"
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
	case *ast.Empty:
		return "null"
	case *ast.Ident:
		switch strings.ToLower(x.Name()) {
		case "true", "false":
			return "boolean"
		case "null":
			return "null"
		}
	}
	return "value"
}

func isNullable(col *ast.Column) bool {
	if col.Settings.Get("not null") != nil || col.Settings.Get("pk") != nil || col.Settings.Get("primary key") != nil {
		return false
	}
	for _, f := range col.LegacyFlags {
		if strings.EqualFold(f.Name(), "pk") {
			return false
		}
	}
	return true
}
