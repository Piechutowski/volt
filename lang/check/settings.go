// Settings validation (§settings-lists): per-construct whitelists, value shapes, and
// the at-most-once rule.
package check

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
)

type valueKind int

const (
	flagOnly valueKind = iota
	strValue
	colorValue
	colorOrNone
	exprValue
	identValue
	actionValue
	defaultVal
	refVal
)

type settingSpec struct {
	kinds      map[string]valueKind
	repeatable map[string]bool
	conflicts  [][2]string
	// forbidden maps a setting name to the diagnostic it gets: a
	// setting known elsewhere that this construct refuses for a reason.
	forbidden map[string]string
}

var (
	tableSettings = settingSpec{
		kinds: map[string]valueKind{"headercolor": colorValue, "note": strValue, "model": strValue},
	}
	columnSettings = settingSpec{
		kinds: map[string]valueKind{
			"pk": flagOnly, "null": flagOnly,
			"unique": flagOnly, "increment": flagOnly,
			"default": defaultVal,
			"note":    strValue, "ref": refVal, "tag": strValue,
			"required": flagOnly,
		},
		repeatable: map[string]bool{"ref": true, "tag": true},
		conflicts:  [][2]string{{"required", "null"}, {"required", "default"}, {"required", "increment"}},
	}
	indexSettings = settingSpec{
		kinds: map[string]valueKind{
			"type": identValue, "name": strValue,
			"unique": flagOnly, "pk": flagOnly, "note": strValue,
			"where": exprValue,
		},
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
		if why, bad := spec.forbidden[s.Name]; bad {
			c.errorf(s.Pos(), section, "%s", why)
			continue
		}
		kind, known := spec.kinds[s.Name]
		if !known {
			c.errorf(s.Pos(), section, "unknown setting %q (§%s)", s.Name, section)
			continue
		}
		if seen[s.Name] && !spec.repeatable[s.Name] {
			c.errorf(s.Pos(), "settings-lists", "setting %q may appear at most once", s.Name)
		}
		seen[s.Name] = true
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
			c.errorf(s.Value.Pos(), "color-literals", "color literal '#%s' must be 3 or 6 hex digits", lit.Tok.Val)
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

func (c *checker) defaultValueCheck(s *ast.Setting) {
	switch v := s.Value.(type) {
	case *ast.BasicLit:
		if v.Tok.Kind == token.COLOR {
			c.errorf(v.Pos(), "default-values", "default value must be a number, string, boolean, null, expression or enum constant")
		}
	case *ast.FuncExpr, *ast.NegNumber, *ast.EnumConst:
	case *ast.Ident:
		low := strings.ToLower(v.Name())
		if v.Quoted() || (low != "true" && low != "false" && low != "null") {
			c.errorf(v.Pos(), "default-values", "default value must be a number, string, boolean, null, expression or enum constant; found bare identifier %q", v.Name())
		}
	default:
		c.errorf(s.Value.Pos(), "default-values", "default value must be a number, string, boolean, null, expression or enum constant")
	}
}

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
			continue
		}
		key, ok := tagPairKey(lit.Tok.Val)
		if !ok {
			c.errorf(lit.Pos(), "columns", `tag %q is not one key:"value" pair — write e.g. 'json:"name"' (§columns)`, lit.Tok.Val)
			continue
		}
		if key == "db" {
			c.errorf(lit.Pos(), "columns", `tag key "db" is reserved: the db tag is the scan contract (§struct-tags)`)
			continue
		}
		if seen[key] {
			c.errorf(lit.Pos(), "columns", "tag key %q appears more than once on this column (§columns)", key)
			continue
		}
		seen[key] = true
	}
}

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
		if !cd.Type.Known() {
			continue
		}
		if cd.Type.Storage != "int" || cd.Type.Decl != nil {
			c.errorf(inc.Pos(), "columns",
				"increment on %q (%s): only an int column can auto-increment (§columns, §mapping-to-sqlite-ddl)", name, col.Type.String())
			continue
		}
		isPK := col.Settings.Get("pk") != nil
		switch {
		case compositePK:
			c.errorf(inc.Pos(), "columns",
				"increment on %q: a composite primary key has no auto-assigned column — SQLite auto-assigns only the single-column integer key (§columns, §mapping-to-sqlite-ddl)", name)
		case !isPK:
			c.errorf(inc.Pos(), "columns",
				"increment on %q: only the primary key auto-increments; elsewhere the keyword would be silently ignored while no changeset ever marks the column (§columns, §mapping-to-sqlite-ddl)", name)
		}
	}
}

func (c *checker) requiredCheck(ti *TableInfo) {
	for _, cd := range ti.Columns {
		col := cd.Col
		req := col.Settings.Get("required")
		if req == nil {
			continue
		}
		name := col.Name.Name()
		if !cd.Type.Known() {
			continue
		}
		if _, ok := cd.Type.RequiredKind(); !ok {
			c.errorf(req.Pos(), "columns", "required on %q (%s): a %s has no empty value — every value is a value (§columns)", name, col.Type.String(), col.Type.String())
		}
	}
}
