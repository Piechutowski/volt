package parser

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

// ElementWords are the words that open a top-level element, in their
// one spelling (spec §keywords).
var ElementWords = []string{
	"package", "import",
	"Project", "Table", "TablePartial", "Type", "Ref", "Records",
	"Pipeline", "Scope", "Group", "Pred", "Select", "Migration",
}

// SettingNames are the names of every setting of the language, in
// their one spelling (spec §keywords).
var SettingNames = []string{
	"pk", "null", "unique", "increment", "default", "note", "ref", "tag",
	"required", "model", "name", "where", "delete", "update", "from",
	"down", "irreversible", "no_transaction",
	"pipe", "error_handler", "api", "only", "except", "param", "singular",
	"strip", "order",
}

// RefActions are the values of delete: and update: (spec §keywords).
var RefActions = []string{"cascade", "restrict", "set_null", "set_default"}

// ResourceActions are the action names a resources declaration lists
// in only: and except: (spec §keywords).
var ResourceActions = []string{"index", "new", "create", "show", "edit", "update", "delete"}

// ParamTypes are the types a route parameter takes, written
// :name(type) (spec §keywords, §route-paths).
var ParamTypes = []string{"int", "string"}

// Verbs are the words that open a route (spec §keywords).
var Verbs = []string{"get", "post", "put", "patch", "delete", "options", "head", "any"}

var declWords = foldMap(ElementWords)

func foldMap(words []string) map[string]string {
	m := make(map[string]string, len(words))
	for _, w := range words {
		m[strings.ToLower(w)] = w
	}
	return m
}

func foldTo(word string, words []string) (string, bool) {
	for _, w := range words {
		if strings.EqualFold(word, w) {
			return w, true
		}
	}
	return "", false
}

func (p *parser) caseErr(t token.Token, right string) {
	p.errorf(t, "write %s, not %s: keywords are case-sensitive (§keywords)", right, t.Val)
}

func (p *parser) kwTake(kw string) token.Token {
	t := p.next()
	if t.Val != kw {
		p.caseErr(t, kw)
		t.Val = kw
	}
	return t
}

func (p *parser) wordFold(id *ast.Ident, words []string) {
	if id == nil {
		return
	}
	if right, ok := foldTo(id.Tok.Val, words); ok && right != id.Tok.Val {
		p.caseErr(id.Tok, right)
		id.Tok.Val = right
	}
}

var literalWords = []string{"true", "false", "null"}
var orderWords = []string{"asc", "desc"}
var predWords = []string{"and", "or", "not", "in", "like", "is", "null"}
