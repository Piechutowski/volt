// Settings validation (§4.2): per-construct whitelists, value shapes, and
// the at-most-once rule. Table-driven so adding a setting is one map entry.
package check

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
)

// valueKind describes what value shape a setting accepts.
type valueKind int

const (
	flagOnly    valueKind = iota // no value allowed
	strValue                     // string literal
	colorValue                   // color literal (3 or 6 hex digits)
	colorOrNone                  // color literal or the identifier none
	exprValue                    // backtick expression
	identValue                   // an identifier (e.g. index type)
	actionValue                  // referential action; validated separately
	defaultVal                   // §6.4 default value
	refVal                       // §6.7 inline relationship
)

// settingSpec describes the settings a construct accepts.
type settingSpec struct {
	kinds      map[string]valueKind
	repeatable map[string]bool
	synonyms   map[string]string // alt spelling -> canonical
	conflicts  [][2]string       // mutually exclusive pairs
}

var (
	tableSettings = settingSpec{
		// model is our extension (D10): the singular Go model name when the
		// inflector's guess is wrong or unwanted.
		kinds: map[string]valueKind{"headercolor": colorValue, "note": strValue, "model": strValue},
	}
	columnSettings = settingSpec{
		// tag is our extension (D60): one verbatim Go struct-tag pair per
		// setting, shaped and deduplicated by tagSettingsCheck.
		kinds: map[string]valueKind{
			"pk": flagOnly, "primary key": flagOnly,
			"null": flagOnly, "not null": flagOnly,
			"unique": flagOnly, "increment": flagOnly,
			"default": defaultVal, "check": exprValue,
			"note": strValue, "ref": refVal, "tag": strValue,
		},
		repeatable: map[string]bool{"check": true, "ref": true, "tag": true},
		synonyms:   map[string]string{"primary key": "pk"},
		conflicts:  [][2]string{{"null", "not null"}},
	}
	indexSettings = settingSpec{
		kinds: map[string]valueKind{
			"type": identValue, "name": strValue,
			"unique": flagOnly, "pk": flagOnly, "note": strValue,
		},
	}
	settingsCheck = settingSpec{
		kinds: map[string]valueKind{"name": strValue},
	}
	refSettings = settingSpec{
		kinds: map[string]valueKind{
			"delete": actionValue, "update": actionValue,
			"color": colorValue, "inactive": flagOnly,
		},
	}
	enumValueSettings = settingSpec{
		kinds: map[string]valueKind{"note": strValue},
	}
	groupSettings = settingSpec{
		kinds: map[string]valueKind{"note": strValue, "color": colorValue},
	}
	stickySettings = settingSpec{
		kinds: map[string]valueKind{"color": colorOrNone},
	}
)

func (c *checker) settingsCheck(sl *ast.SettingList, section string, spec settingSpec) {
	if sl == nil {
		return
	}
	seen := map[string]bool{}
	for _, s := range sl.Settings {
		canon := s.Name
		if alt, ok := spec.synonyms[canon]; ok {
			canon = alt
		}
		kind, known := spec.kinds[s.Name]
		if !known {
			c.errorf(s.Pos(), section, "unknown setting %q", s.Name)
			continue
		}
		if seen[canon] && !spec.repeatable[canon] {
			c.errorf(s.Pos(), "4.2", "setting %q may appear at most once", s.Name)
		}
		seen[canon] = true
		c.settingValueCheck(s, section, kind)
	}
	for _, pair := range spec.conflicts {
		if seen[pair[0]] && seen[pair[1]] {
			c.errorf(sl.Pos(), section, "%q and %q cannot both be set", pair[0], pair[1])
		}
	}
}

func (c *checker) settingValueCheck(s *ast.Setting, section string, kind valueKind) {
	switch kind {
	case flagOnly:
		if s.Value != nil {
			c.errorf(s.Pos(), section, "%q must not have a value", s.Name)
		}
		return
	default:
		if s.Value == nil {
			c.errorf(s.Pos(), section, "%q requires a value", s.Name)
			return
		}
	}
	switch kind {
	case strValue:
		if !isString(s.Value) {
			c.errorf(s.Value.Pos(), section, "%q must be a string", s.Name)
		}
	case colorValue, colorOrNone:
		if kind == colorOrNone {
			if id, ok := s.Value.(*ast.Ident); ok && strings.EqualFold(id.Name(), "none") && !id.Quoted() {
				return
			}
		}
		lit, ok := s.Value.(*ast.BasicLit)
		if !ok || lit.Tok.Kind != token.COLOR {
			c.errorf(s.Value.Pos(), section, "%q must be a color literal", s.Name)
			return
		}
		if !validColor(lit.Tok.Val) {
			c.errorf(s.Value.Pos(), "3.11", "color literal '#%s' must be 3 or 6 hex digits", lit.Tok.Val)
		}
	case exprValue:
		if _, ok := s.Value.(*ast.FuncExpr); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be a backtick expression", s.Name)
		}
	case identValue:
		if _, ok := s.Value.(*ast.Ident); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be an identifier", s.Name)
		}
	case actionValue:
		switch s.Value.(type) {
		case *ast.Ident, parser.MultiWord:
			// the action vocabulary is validated in refSettingsCheck
		default:
			c.errorf(s.Value.Pos(), section, "%q must be a referential action", s.Name)
		}
	case defaultVal:
		c.defaultValueCheck(s)
	case refVal:
		if _, ok := s.Value.(*ast.RefValue); !ok {
			c.errorf(s.Value.Pos(), section, "'ref' must be a relationship operator followed by an endpoint")
		}
	}
}

// defaultValueCheck enforces §6.4: number, string, boolean, null,
// expression, or a dotted enum constant — never a bare identifier.
func (c *checker) defaultValueCheck(s *ast.Setting) {
	switch v := s.Value.(type) {
	case *ast.BasicLit:
		if v.Tok.Kind == token.COLOR {
			c.errorf(v.Pos(), "6.4", "default value must be a number, string, boolean, null, expression or enum constant")
		}
	case *ast.FuncExpr, *ast.NegNumber, *ast.EnumConst:
	case *ast.Ident:
		low := strings.ToLower(v.Name())
		if v.Quoted() || (low != "true" && low != "false" && low != "null") {
			c.errorf(v.Pos(), "6.4", "default value must be a number, string, boolean, null, expression or enum constant; found bare identifier %q", v.Name())
		}
	default:
		c.errorf(s.Value.Pos(), "6.4", "default value must be a number, string, boolean, null, expression or enum constant")
	}
}

// tagSettingsCheck enforces the tag: extension's shape rules (§6.3,
// D60): one key:"value" pair per setting, keys unique per column, db
// reserved for the scan contract.
func (c *checker) tagSettingsCheck(sl *ast.SettingList) {
	if sl == nil {
		return
	}
	seen := map[string]bool{}
	for _, s := range sl.Settings {
		if s.Name != "tag" {
			continue
		}
		lit, ok := s.Value.(*ast.BasicLit)
		if !ok || lit.Tok.Kind != token.STRING {
			continue // shape already reported by settingValueCheck
		}
		key, ok := tagPairKey(lit.Tok.Val)
		if !ok {
			c.errorf(lit.Pos(), "6.3", `tag %q is not one key:"value" pair — write e.g. 'json:"name"' (§6.3)`, lit.Tok.Val)
			continue
		}
		if key == "db" {
			c.errorf(lit.Pos(), "6.3", `tag key "db" is reserved: the db tag is the scan contract (Appendix A.5)`)
			continue
		}
		if seen[key] {
			c.errorf(lit.Pos(), "6.3", "tag key %q appears more than once on this column (§6.3)", key)
			continue
		}
		seen[key] = true
	}
}

// tagPairKey parses one key:"value" struct-tag pair, returning its key.
// The shape rules are §6.3's: no spaces, colons, double quotes or
// backticks in the key; a double-quoted value free of both quote kinds.
func tagPairKey(pair string) (string, bool) {
	i := strings.IndexByte(pair, ':')
	if i <= 0 {
		return "", false
	}
	key, val := pair[:i], pair[i+1:]
	if strings.ContainsAny(key, " \t\"`") {
		return "", false
	}
	if len(val) < 2 || val[0] != '"' || val[len(val)-1] != '"' {
		return "", false
	}
	if strings.ContainsAny(val[1:len(val)-1], "\"`") {
		return "", false
	}
	return key, true
}

func isString(n ast.Node) bool {
	lit, ok := n.(*ast.BasicLit)
	return ok && lit.Tok.Kind == token.STRING
}

func validColor(v string) bool {
	if len(v) != 3 && len(v) != 6 {
		return false
	}
	for _, r := range v {
		if !(('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')) {
			return false
		}
	}
	return true
}

// integerTypes is the integer family of Appendix A/B: the only types an
// increment column may have (§6.3 extension). nao/gen/golang's tests
// pin this set against its own type map so the two cannot drift.
var integerTypes = map[string]bool{
	"tinyint": true, "int2": true, "smallint": true, "smallserial": true,
	"int": true, "integer": true, "int4": true, "mediumint": true, "serial": true,
	"bigint": true, "int8": true, "bigserial": true,
	"tinyint unsigned": true, "smallint unsigned": true, "int unsigned": true,
	"integer unsigned": true, "bigint unsigned": true,
}

// IntegerType reports whether a declared type (arguments stripped,
// case-insensitive) belongs to the integer family.
func IntegerType(declType string) bool {
	base := strings.ToLower(declType)
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	return integerTypes[base]
}

// incrementCheck enforces the §6.3 extension: increment is honored in
// exactly one shape — the single-column integer primary key, SQLite's
// rowid alias (Appendix B) — and is an error anywhere else, because a
// keyword that is typed must be applied.
func (c *checker) incrementCheck(ti *TableInfo) {
	compositePK := false
	for _, ix := range ti.Indexes {
		if ix.Settings.Get("pk") != nil {
			compositePK = true
		}
	}
	for _, cd := range ti.Columns {
		col := cd.Col
		inc := col.Settings.Get("increment")
		if inc == nil {
			continue
		}
		name := col.Name.Name()
		if !IntegerType(col.Type.String()) {
			c.errorf(inc.Pos(), "6.3",
				"increment on %q (%s): only an integer column can auto-increment (§6.3, Appendix B)", name, col.Type.String())
			continue
		}
		isPK := col.Settings.Get("pk") != nil || col.Settings.Get("primary key") != nil
		for _, f := range col.LegacyFlags {
			if strings.EqualFold(f.Name(), "pk") {
				isPK = true
			}
		}
		switch {
		case compositePK:
			c.errorf(inc.Pos(), "6.3",
				"increment on %q: a composite primary key has no auto-assigned column — SQLite auto-assigns only the single-column integer key (§6.3, Appendix B)", name)
		case !isPK:
			c.errorf(inc.Pos(), "6.3",
				"increment on %q: only the primary key auto-increments; elsewhere the keyword would be silently ignored while CreateParams omits the column (§6.3, Appendix B)", name)
		}
	}
}
