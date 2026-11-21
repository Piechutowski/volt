package lsp

import (
	"strings"
	"unicode/utf8"

	"github.com/Piechutowski/volt/lang/scanner"
)

const (
	identCharSet = `\p{L}\p{Nd}_`
	identStart   = `[\p{L}_]`
	identChar    = `[` + identCharSet + `]`
	identName    = identStart + identChar + `*`
	identBefore  = `(?:^|[^` + identCharSet + `])`
	identAfter   = `(?:[^` + identCharSet + `]|$)`
)

func identReplace(text, old, replacement string) string {
	var b strings.Builder
	next := 0
	for i := range len(text) {
		if i < next || !identAt(text, i, old) {
			continue
		}
		b.WriteString(text[next:i])
		b.WriteString(replacement)
		next = i + len(old)
	}
	b.WriteString(text[next:])
	return b.String()
}

func identAt(text string, i int, name string) bool {
	if name == "" || !strings.HasPrefix(text[i:], name) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(text[:i])
	after, _ := utf8.DecodeRuneInString(text[i+len(name):])
	return (i == 0 || !scanner.IsIdentChar(before)) && (i+len(name) == len(text) || !scanner.IsIdentChar(after))
}
