package ebnf

import (
	"strings"
	"testing"
)

func TestParseEveryConstruct(t *testing.T) {
	src := `
(* a comment before *)
program   = { statement } ;
statement = name, "=", value, [ "," ], newline ;   (* trailing comment *)
value     = number | 'q"q' | "'" | 4 * hex digit | any char - newline ;
hex digit = "a" | "b" ;
empty     = ;
either    = | "x" ;
newline   = ? U+000A LINE FEED ? ;
any char  = ? any Unicode
              character ? ;
nested    = ( "a", { "b" | "c" } ), [ "d", ( "e" | "f" ) ] - "z" ;
`
	g, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"program":   `{ statement }`,
		"statement": `name, "=", value, [ "," ], newline`,
		"value":     `number | 'q"q' | "'" | 4 * hex digit | any char - newline`,
		"hex digit": `"a" | "b"`,
		"empty":     ``,
		"either":    ` | "x"`,
		"newline":   `? U+000A LINE FEED ?`,
		"any char":  `? any Unicode character ?`,
		"nested":    `( "a", { "b" | "c" } ), [ "d", ( "e" | "f" ) ] - "z"`,
	}
	if len(g.Rules) != len(want) {
		t.Fatalf("%d rules, want %d", len(g.Rules), len(want))
	}
	for name, w := range want {
		r := g.Lookup(name)
		if r == nil {
			t.Errorf("%s: not defined", name)
			continue
		}
		if got := Format(r.Expr); got != w {
			t.Errorf("%s: formatted %q, want %q", name, got, w)
		}
	}
	if g.Lookup("newline").Line != 9 {
		t.Errorf("newline defined at line %d, want 9", g.Lookup("newline").Line)
	}
	if refs := Refs(g.Lookup("statement").Expr); strings.Join(refs, ",") != "name,value,newline" {
		t.Errorf("refs of statement: %v", refs)
	}
}

func TestFormatRoundTrips(t *testing.T) {
	src := `a = ( "x" | "y" ), { "z" } - ( "z", "z" ), [ b | c, d ] ;
b = 3 * ( "p" | "q" ) ;
c = ;
d = "d" ;`
	g, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var again strings.Builder
	for _, r := range g.Rules {
		again.WriteString(r.Name + " = " + Format(r.Expr) + " ;\n")
	}
	g2, err := Parse(again.String())
	if err != nil {
		t.Fatalf("formatted grammar does not read back: %v\n%s", err, again.String())
	}
	for _, r := range g.Rules {
		if Format(g2.Lookup(r.Name).Expr) != Format(r.Expr) {
			t.Errorf("%s: %q reads back as %q", r.Name, Format(r.Expr), Format(g2.Lookup(r.Name).Expr))
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		`a = "x"`:              "line 1: expected \";\" ending a, found end of grammar",
		`a = "x" ; a = "y" ;`:  "line 1: a is defined twice (first at line 1)",
		`a = ( "x" ;`:          "line 1: expected \")\" closing a group, found ;",
		`a = "x ;`:             "line 1: unterminated terminal",
		`a = ? x ;`:            "line 1: unterminated special sequence",
		`a = (* x ;`:           "line 1: unterminated comment",
		`a = 3 "x" ;`:          "line 1: expected \"*\" after a repetition count, found \"x\"",
		`= "x" ;`:              "line 1: expected a rule name, found =",
		"a = \"x\" ;\nb = , ;": "line 2: expected a name, a terminal, a special sequence or a bracket, found ,",
		`a = "x" @ ;`:          "line 1: unexpected character '@'",
	}
	for src, want := range cases {
		_, err := Parse(src)
		if err == nil || err.Error() != want {
			t.Errorf("%q: error %v, want %s", src, err, want)
		}
	}
}
