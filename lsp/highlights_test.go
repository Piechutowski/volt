package lsp

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var (
	grammarKeyword  = regexp.MustCompile(`kw\('([a-z_]+)'\)`)
	highlightQuoted = regexp.MustCompile(`"([a-z_]+)"`)
)

// TestEveryKeywordHighlights holds the Zed highlight queries to the
// tree-sitter grammar (§keywords, editor.md section 5): every word the
// grammar matches as a keyword is captured, either by its own spelling
// or through the node it is aliased to, so a keyword the grammar gains
// cannot be left uncoloured.
func TestEveryKeywordHighlights(t *testing.T) {
	grammar, err := os.ReadFile("../grammar/grammar.js")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := os.ReadFile("../zed-extension/languages/volt/highlights.scm")
	if err != nil {
		t.Fatal(err)
	}
	byNode := map[string]string{
		"get": "(verb)", "post": "(verb)", "patch": "(verb)", "delete": "(verb)",
		"options": "(verb)", "head": "(verb)", "any": "(verb)",
		"true": "(boolean)", "false": "(boolean)", "null": "(null)",
		"count": "(aggregate_name)", "sum": "(aggregate_name)", "avg": "(aggregate_name)",
		"min": "(aggregate_name)", "max": "(aggregate_name)",
	}
	quoted := map[string]bool{}
	for _, m := range highlightQuoted.FindAllStringSubmatch(string(queries), -1) {
		quoted[m[1]] = true
	}
	seen := map[string]bool{}
	for _, m := range grammarKeyword.FindAllStringSubmatch(string(grammar), -1) {
		word := m[1]
		if seen[word] {
			continue
		}
		seen[word] = true
		if node, ok := byNode[word]; ok {
			if !strings.Contains(string(queries), node) {
				t.Errorf("keyword %q highlights through %s, which highlights.scm does not capture", word, node)
			}
			continue
		}
		if !quoted[word] {
			t.Errorf("keyword %q of the grammar is not captured by highlights.scm", word)
		}
	}
	if len(seen) < 40 {
		t.Fatalf("read only %d keywords from grammar.js", len(seen))
	}
}
