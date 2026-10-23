// Package naming decides the names and shapes of a checked package's
// generated data surface without generating any code: every Go and SQL
// name the data files mint, the methods they declare and the statements
// those methods run (spec §mapping-to-go, §generated-query-surface). The
// data generator renders it; the checker, vet and the language server read
// it.
package naming

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/inflect"
)

var commonInitialisms = map[string]bool{
	"acl": true, "api": true, "ascii": true, "cpu": true, "css": true,
	"db": true, "dns": true, "eof": true, "guid": true, "html": true,
	"http": true, "https": true, "id": true, "ip": true, "json": true,
	"lhs": true, "qps": true, "ram": true, "rhs": true, "rpc": true,
	"sla": true, "smtp": true, "sql": true, "ssh": true, "tcp": true,
	"tls": true, "ttl": true, "udp": true, "ui": true, "uid": true,
	"uuid": true, "uri": true, "url": true, "utf8": true, "vm": true,
	"xml": true, "xmpp": true, "xsrf": true, "xss": true,
}

// GoName converts a schema name to an exported Go identifier, following
// the Go conventions for initialisms (user_id is UserID; spec
// §mapping-to-go).
func GoName(schemaName string) (string, error) {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range schemaName {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	if len(words) == 0 {
		return "", fmt.Errorf("name %q contains no characters usable in a Go identifier", schemaName)
	}

	var b strings.Builder
	for _, w := range words {
		if commonInitialisms[strings.ToLower(w)] {
			b.WriteString(strings.ToUpper(w))
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	name := b.String()
	if r := []rune(name)[0]; unicode.IsDigit(r) {
		name = "X" + name
	}
	return name, nil
}

// ModelName derives the Go model type name for a table: the [model:]
// override when present, else the singularized table name (spec
// §mapping-to-go).
func ModelName(t *ast.Table) (string, error) {
	if s := t.Settings.Get("model"); s != nil {
		if lit, ok := s.Value.(*ast.BasicLit); ok {
			return GoName(lit.Tok.Val)
		}
	}
	singular, _ := inflect.SingularLast(t.Name.Name())
	return GoName(singular)
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true,
	"for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true,
	"switch": true, "type": true, "var": true,
}

// ArgName is the unexported spelling of an exported Go name, the stem of
// the package's unexported helpers (spec §generated-query-surface); a Go
// keyword and a name the generated signatures take (ctx, arg, q) take a
// trailing underscore.
func ArgName(exported string) string {
	runes := []rune(exported)
	upper := 0
	for upper < len(runes) && runes[upper] >= 'A' && runes[upper] <= 'Z' {
		upper++
	}
	switch {
	case upper == 0:
		if len(runes) > 0 {
			runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
		}
	case upper == len(runes) || upper == 1:
		for i := 0; i < upper; i++ {
			runes[i] = runes[i] - 'A' + 'a'
		}
	default:
		for i := 0; i < upper-1; i++ {
			runes[i] = runes[i] - 'A' + 'a'
		}
	}
	s := string(runes)
	if goKeywords[s] || s == "ctx" || s == "arg" || s == "q" {
		s += "_"
	}
	return s
}

func paramName(goField string) string {
	arg := ArgName(goField)
	if arg == "rt" || LocalName(arg) {
		arg += "_"
	}
	return arg
}

// LocalName reports whether name has the form of a local the data
// generator declares: volt, an upper-case letter, and no underscore, as in
// voltErr and voltRows (spec §generated-query-surface). No column's Go
// parameter and no select parameter takes that form.
func LocalName(name string) bool {
	rest, ok := strings.CutPrefix(name, "volt")
	if !ok || strings.Contains(rest, "_") {
		return false
	}
	for _, r := range rest {
		return unicode.IsUpper(r)
	}
	return false
}

// SettingNote is the text of a list's note setting, "" when it has none
// (spec §settings-lists).
func SettingNote(sl *ast.SettingList) string {
	s := sl.Get("note")
	if s == nil {
		return ""
	}
	if lit, ok := s.Value.(*ast.BasicLit); ok {
		return lit.Tok.Val
	}
	return ""
}

// ImportKey is the key of one import of the generated files: the path,
// or alias=path for a declared type's package whose alias is not the one
// the path implies (spec §types).
func ImportKey(alias, path string) string {
	if path == "" || alias == "" || check.ReservedAliases[alias] == path {
		return path
	}
	return alias + "=" + path
}

func fieldTag(col *ast.Column) string {
	name := col.Name.Name()
	jsonPair := `json:"` + name + `"`
	var extra []string
	if col.Settings != nil {
		for _, s := range col.Settings.Settings {
			if s.Name != "tag" {
				continue
			}
			lit, ok := s.Value.(*ast.BasicLit)
			if !ok || lit.Tok.Kind != token.STRING {
				continue
			}
			pair := lit.Tok.Val
			if key, _, ok := strings.Cut(pair, ":"); ok && key == "json" {
				jsonPair = pair
				continue
			}
			extra = append(extra, pair)
		}
	}
	parts := append([]string{`db:"` + name + `"`, jsonPair}, extra...)
	return strings.Join(parts, " ")
}

func wireName(colName, tag string) string {
	const key = `json:"`
	i := strings.Index(tag, key)
	if i < 0 {
		return colName
	}
	rest := tag[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return colName
	}
	name, _, _ := strings.Cut(rest[:j], ",")
	if name == "" || name == "-" {
		return colName
	}
	return name
}
