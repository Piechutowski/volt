package ebnf

import (
	"testing"
	"unicode"
)

const miniGrammar = `
program    = { element } ;
element    = block | line ;
block      = "Block", name, [ settings ], "{", { item }, "}" ;
item       = name, name, newline ;
line       = "Line", path, newline ;
path       = "/", { name, "/" } ;
settings   = "[", name, "]" ;
name       = letter, { letter | digit } ;
letter     = ? letter ? ;
digit      = ? digit ? ;
newline    = ? U+000A LINE FEED ? ;
sp         = ? U+0020 SPACE ? | ? U+0009 TAB ? ;
comment    = "//", { any char - newline } ;
any char   = ? any character ? ;
pair       = 2 * digit ;
op         = "<" | "<>" | "-" | ">" ;
ops        = op, op ;
`

func miniLanguage(t *testing.T) *Language {
	t.Helper()
	g, err := Parse(miniGrammar)
	if err != nil {
		t.Fatal(err)
	}
	l := &Language{
		G:          g,
		Lexical:    map[string]bool{"name": true, "letter": true, "digit": true, "newline": true, "sp": true, "comment": true, "any char": true, "pair": true},
		Layout:     Rep{X: Alt{Items: []Expr{Ref{"sp"}, Ref{"comment"}}}},
		Newline:    "newline",
		Contiguous: map[string]bool{"path": true},
		SameLine:   map[string]bool{"settings": true},
		Specials: map[string]func(rune) bool{
			"letter":           unicode.IsLetter,
			"digit":            unicode.IsDigit,
			"U+000A LINE FEED": func(r rune) bool { return r == '\n' },
			"U+0020 SPACE":     func(r rune) bool { return r == ' ' },
			"U+0009 TAB":       func(r rune) bool { return r == '\t' },
			"any character":    func(r rune) bool { return true },
		},
		Cut: true,
	}
	if err := l.Check(); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestRecognizerDecidesSentences(t *testing.T) {
	l := miniLanguage(t)
	cases := []struct {
		rule, text string
		want       bool
	}{
		{"block", "Block a {}", true},
		{"block", "Block a{}", true},
		{"block", "Block\ta // c\n{\n}", true},
		{"block", "Blocka {}", false},
		{"block", "Block a { x y\n}", true},
		{"block", "Block a { x y }", true},
		{"block", "Block a { x y", false},
		{"block", "Block a { xy\n}", false},
		{"block", "Block a { x y\n\n z w\n}", true},
		{"block", "Block a {\n// only a comment\n}", true},
		{"block", "Block a [s] {}", true},
		{"block", "Block a\n[s] {}", false},
		{"block", "Block a [s]\n{}", true},
		{"line", "Line /a/b/\n", true},
		{"line", "Line /a/ b/\n", false},
		{"line", "Line / \n", true},
		{"line", "Line /", true},
		{"ops", "<>-", true},
		{"ops", "< >", true},
		{"ops", "<>", false},
		{"ops", "<--", false},
		{"pair", "12", true},
		{"pair", "1", false},
		{"pair", "123", false},
		{"program", "Block a {}\nBlock b {}", true},
		{"program", "Block a {}Block b {}", true},
		{"program", "Block a {\n}\nLine /x/\n", true},
		{"program", "Block a [\ns] {}", false},
		{"program", "Block a {\nx y\n}", true},
		{"program", "Block a\n{}", true},
		{"program", "Block\na {}", false},
		{"program", "", true},
		{"program", "// nothing\n", true},
		{"program", "Line /a/\nBlock b {\n\tc d\n}\n", true},
	}
	for _, c := range cases {
		if got := l.Accepts(c.rule, c.text); got != c.want {
			t.Errorf("%s %q: accepts %v, want %v", c.rule, c.text, got, c.want)
		}
	}
}

func TestElementStarts(t *testing.T) {
	cases := map[string][]int{
		"Block a {}\nBlock b {}":   {11},
		"Block a {\nx y\n}\nBlock": {16},
		"Block a {\n}\n  x\ny":     {16},
		"a\n\"x\ny\"\nb":           {5, 8},
		"a\n\"\"\"x\ny\"\"\"\nb":   {12},
		"a\n`x\ny`\nb":             {8},
		"a\n/* x\ny */\nb":         {12},
		"a\n// x\nb":               {7},
		"a\n\"q\"\n1b":             {6},
		"a\n{\nb\n}\nc\n}\nd":      {8, 12},
		"a\néb":                    {2},
		"a\n\n\n":                  nil,
		"a\n[\nb":                  {4},
		"a\n0\nb":                  {4},
		"a\n1.5\nb":                {6},
		"a\n1e2\nb":                {6},
		"a\n1e+2\nb":               {7},
		"a\n2fa\nb":                {2, 6},
		"a\n1e2x\nb":               {2, 7},
	}
	for text, want := range cases {
		got := ElementStarts(text)
		if len(got) != len(want) {
			t.Errorf("%q: starts %v, want %v", text, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: starts %v, want %v", text, got, want)
				break
			}
		}
	}
}
