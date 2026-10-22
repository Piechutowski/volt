// Identifier conversion: schema names (snake_case, Unicode) to
// exported Go names, following the Go community conventions for
// initialisms (user_id -> UserID, not UserId).
package golang

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
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

func goName(schemaName string) (string, error) {
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

func modelName(t *ast.Table) (string, error) {
	if s := t.Settings.Get("model"); s != nil {
		if lit, ok := s.Value.(*ast.BasicLit); ok {
			return goName(lit.Tok.Val)
		}
	}
	singular, _ := inflect.SingularLast(t.Name.Name())
	return goName(singular)
}
