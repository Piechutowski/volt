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
	nameValue
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
			"unique": flagOnly, "index": flagOnly, "autoincrement": flagOnly,
			"default": defaultVal,
			"note":    strValue, "ref": refVal, "tag": strValue,
			"required": flagOnly,
		},
		repeatable: map[string]bool{"tag": true},
		conflicts:  [][2]string{{"required", "default"}, {"unique", "index"}},
	}
	indexSettings = settingSpec{
		kinds: map[string]valueKind{
			"name":   strValue,
			"unique": flagOnly,
			"where":  exprValue,
		},
		forbidden: map[string]string{"pk": pkNotIndexSetting("indexes")},
	}
	refSettings = settingSpec{
		kinds: map[string]valueKind{"delete": actionValue, "update": actionValue, "as": nameValue, "reverse": nameValue},
	}
)

// pkNotIndexSetting is the diagnostic of pk on an index, citing section
// (spec §indexes, §migrations).
func pkNotIndexSetting(section string) string {
	return "pk is a column setting, not an index setting: write pk on each key column, and several pk columns form one composite primary key in declaration order (§" + section + ")"
}

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
			c.errorf(s.Pos(), "settings-lists", "setting %q may appear at most once (§settings-lists)", s.Name)
		}
		seen[s.Name] = true
		c.settingValueCheck(s, section, kind)
	}
	for _, pair := range spec.conflicts {
		if seen[pair[0]] && seen[pair[1]] {
			c.errorf(sl.Pos(), section, "%q and %q cannot both be set (§%s)", pair[0], pair[1], section)
		}
	}
}

func (c *checker) settingValueCheck(s *ast.Setting, section string, kind valueKind) {
	switch kind {
	case flagOnly:
		if s.Value != nil {
			c.errorf(s.Pos(), section, "%q must not have a value (§%s)", s.Name, section)
		}
		return
	default:
		if s.Value == nil {
			c.errorf(s.Pos(), section, "%q requires a value (§%s)", s.Name, section)
			return
		}
	}
	switch kind {
	case strValue:
		if !isString(s.Value) {
			c.errorf(s.Value.Pos(), section, "%q must be a string (§%s)", s.Name, section)
		}
	case exprValue:
		if _, ok := s.Value.(*ast.FuncExpr); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be a backtick expression (§%s)", s.Name, section)
		}
	case actionValue:
		if _, ok := s.Value.(*ast.Ident); !ok {
			c.errorf(s.Value.Pos(), section, "%q must be a referential action: cascade, restrict, set_null or set_default (§%s)", s.Name, section)
		}
	case nameValue:
		if _, ok := s.Value.(*ast.Ident); !ok {
			c.errorf(s.Value.Pos(), section, "%q takes a name (§%s)", s.Name, section)
		}
	case defaultVal:
		c.defaultValueCheck(s)
	case refVal:
		ep, ok := s.Value.(*ast.RefEndpoint)
		if !ok {
			c.errorf(s.Value.Pos(), section, "'ref' must be an endpoint, table.column (§%s)", section)
		} else if ep.Paren {
			c.errorf(ep.Pos(), "columns", "an inline ref endpoint is table.column, without parentheses: a foreign key over several columns is a ref element (§columns)")
		}
	}
}

func (c *checker) defaultValueCheck(s *ast.Setting) {
	switch v := s.Value.(type) {
	case *ast.BasicLit, *ast.FuncExpr, *ast.NegNumber:
	case *ast.Ident:
		switch name := v.Name(); {
		case name == "null":
			c.errorf(v.Pos(), "default-values", "null is not a default value: a column that carries null and has no default is NULL when a write leaves it out (§default-values)")
		case name != "true" && name != "false":
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
			c.errorf(lit.Pos(), "columns", `tag %q is not one key:"value" pair: write e.g. "json:\"name\"" (§columns)`, lit.Tok.Val)
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

// rowidAliasCheck enforces that autoincrement is written on the rowid
// alias alone (spec §columns), the one column SQLite takes it on.
func (c *checker) rowidAliasCheck(ti *TableInfo) {
	pk := ti.PK()
	alias := ti.RowidAlias()
	for _, cd := range ti.Columns {
		col := cd.Col
		name := col.Name.Name()
		auto := col.Settings.Get("autoincrement")
		if cd == alias || auto == nil || !cd.Type.Known() {
			continue
		}
		isPK := col.Settings.Get("pk") != nil
		switch {
		case cd.Type.Decl != nil && cd.Type.Storage == "int":
			c.errorf(auto.Pos(), "columns",
				"autoincrement on %q (%s): only a key of the built-in int is the rowid alias, which alone takes AUTOINCREMENT; a declared type stored as int is not (§columns, §mapping-to-sqlite-ddl)", name, col.Type.String())
		case cd.Type.Storage != "int" || cd.Type.Decl != nil:
			c.errorf(auto.Pos(), "columns",
				"autoincrement on %q (%s): only an int column can be the rowid alias, which alone takes AUTOINCREMENT (§columns, §mapping-to-sqlite-ddl)", name, col.Type.String())
		case len(pk) > 1 && isPK:
			key := make([]string, len(pk))
			for i, cd := range pk {
				key[i] = cd.Col.Name.Name()
			}
			c.errorf(auto.Pos(), "columns",
				"autoincrement on %q in table %q, whose primary key is (%s): a composite primary key has no rowid alias, since SQLite aliases the rowid only by a single-column int key (§columns)", name, ti.Key, strings.Join(key, ", "))
		default:
			c.errorf(auto.Pos(), "columns",
				"autoincrement on %q: only the rowid alias, the single-column int primary key, takes AUTOINCREMENT; elsewhere SQLite refuses the keyword (§columns, §mapping-to-sqlite-ddl)", name)
		}
	}
}

func (c *checker) requiredCheck(ti *TableInfo) {
	alias := ti.RowidAlias()
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
		if cd == alias {
			c.errorf(req.Pos(), "columns", "required on %q, the rowid alias of table %q: the database assigns it whenever an insert leaves it unmarked, so its zero value is no missing value (§columns, §changesets)", name, ti.Key)
			continue
		}
		if _, ok := cd.Type.RequiredKind(); !ok {
			c.errorf(req.Pos(), "columns", "required on %q (%s): a %s has no empty value — every value is a value (§columns)", name, col.Type.String(), col.Type.String())
		}
	}
}
