package ebnf

import (
	"strings"
	"testing"
)

func miniWitness(l *Language) {
	l.Witness = map[string]string{
		"letter":           "a",
		"digit":            "1",
		"U+000A LINE FEED": "\n",
		"U+0020 SPACE":     " ",
		"U+0009 TAB":       "\t",
		"any character":    "c",
	}
}

func TestSentencesCoverEveryChoiceAndAreRecognized(t *testing.T) {
	l := miniLanguage(t)
	miniWitness(l)
	sentences, unreachable, err := l.Sentences()
	if err != nil {
		t.Fatal(err)
	}
	// the line break is rendered, never a hole; layout rules hang off
	// no rule; ops and pair off no rule the start reaches
	if got := strings.Join(unreachable, ","); got != "newline,sp,comment,any char,pair,op,ops" {
		t.Errorf("unreachable %q", got)
	}
	var texts []string
	byRule := map[string][]string{}
	for _, s := range sentences {
		text := Text(s.Pieces)
		texts = append(texts, text)
		byRule[s.Rule] = append(byRule[s.Rule], text)
		if !l.Accepts("program", text) {
			t.Errorf("derived for %s but not recognized: %q", s.Rule, text)
		}
	}
	want := map[string][]string{
		// block: settings taken and left, items zero, one and two
		"block": {"Block a { }", "Block a [ a ] { }", "Block a { a a\n\t}", "Block a { a a\n\ta a\n\t}"},
		// path: names zero, one and two, contiguous
		"path": {"Line /\n", "Line /a/\n", "Line /a/a/\n"},
		// name: one letter, a second letter, a digit, two more characters
		"name": {"Block a { }", "Block aa { }", "Block a1 { }", "Block aaa { }"},
		// program: zero, one and two elements, on lines of their own
		"program": {"", "Line /\n", "Line /\n\nLine /\n"},
	}
	for rule, w := range want {
		got := byRule[rule]
		for _, s := range w {
			found := false
			for _, g := range got {
				if g == s {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no sentence %q among %q", rule, s, got)
			}
		}
	}
	if len(texts) < 20 {
		t.Errorf("only %d sentences:\n%s", len(texts), strings.Join(texts, "\n"))
	}
}

func TestDeletionsDropOneToken(t *testing.T) {
	pieces := []Piece{{Text: "Block"}, {Text: "a"}, {Text: "{"}, {Text: "x"}, {Text: "y"}, {Text: "\n", NL: true}, {Text: "}"}}
	got := Deletions(pieces)
	want := []string{"Block a x y\n\t}", "Block a { x\n\t}", "Block a { x y\n", "Block a { y\n\t}", "Block { x y\n\t}", "a { x y\n\t}"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("deletions %q, want %q", got, want)
	}
}
