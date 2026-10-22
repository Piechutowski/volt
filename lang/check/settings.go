// Settings validation (§settings-lists): per-construct whitelists, value shapes, and
// the at-most-once rule.
package check

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

type valueKind int

const (
	flagOnly valueKind = iota
	strValue
	exprValue
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
	projectSettings = settingSpec{
		kinds: map[string]valueKind{"note": strValue},
	}
	tableSettings = settingSpec{
		kinds: map[string]valueKind{"note": strValue, "model": strValue},
	}
	columnSettings = settingSpec{
		kinds: map[string]valueKind{
			"pk": flagOnly, "null": flagOnly,
			"unique": flagOnly, "increment": flagOnly,
			"default": defaultVal,
			"note":    strValue, "ref": refVal, "tag": strValue,
			"required": flagOnly,
		},
		repeatable: map[string]bool{"tag": true},
		conflicts:  [][2]string{{"required", "default"}, {"required", "increment"}},
	}
	indexSettings = settingSpec{
		kinds: map[string]valueKind{
			"name":   strValue,
			"unique": flagOnly, "pk": flagOnly,
			"where": exprValue,
		},
	}
	refSettings = settingSpec{
		kinds: map[string]valueKind{"delete": actionValue, "update": actionValue},
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
	case exprValue:
		if _, ok := s.Value.(*ast.FuncExpr); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be a backtick expression", s.Name)
		}
	case actionValue:
		if _, ok := s.Value.(*ast.Ident); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be a referential action: cascade, restrict, set_null or set_default (§%s)", s.Name, section)
		}
	case defaultVal:
		c.defaultValueCheck(s)
	case refVal:
		if _, ok := s.Value.(*ast.RefEndpoint); !ok {
			c.errorf(s.Value.Pos(), section, "'ref' must be an endpoint, table.column")
		}
	}
}

func (c *checker) defaultValueCheck(s *ast.Setting) {
	switch v := s.Value.(type) {
	case *ast.BasicLit, *ast.FuncExpr, *ast.NegNumber:
	case *ast.Ident:
		low := strings.ToLower(v.Name())
		switch {
		case !v.Quoted() && low == "null":
			c.errorf(v.Pos(), "default-values", "null is not a default value: a column that carries null and has no default is NULL when a write leaves it out (§default-values)")
		case v.Quoted() || (low != "true" && low != "false"):
			c.errorf(v.Pos(), "default-values", "default value must be a number, string, boolean or expression literal; found bare identifier %q (§default-values)", v.Name())
		}
	case *ast.QualIdent:
		c.errorf(v.Pos(), "default-values", "default value must be a number, string, boolean or expression literal; found the dotted name %s.%s, and a declared type takes a literal of its storage type (§default-values)", v.Qual.Name(), v.Name.Name())
	default:
		c.errorf(s.Value.Pos(), "default-values", "default value must be a number, string, boolean or expression literal (§default-values)")
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
