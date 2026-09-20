package scanner

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/token"
)

// kinds scans src and returns the token kinds without the trailing EOF.
func kinds(t *testing.T, src string) []token.Kind {
	t.Helper()
	toks, errs := Scan("test.dbml", src)
	if len(errs) > 0 {
		t.Fatalf("Scan(%q) errors: %v", src, errs)
	}
	out := make([]token.Kind, 0, len(toks)-1)
	for _, tk := range toks[:len(toks)-1] {
		out = append(out, tk.Kind)
	}
	return out
}

func one(t *testing.T, src string) token.Token {
	t.Helper()
	toks, errs := Scan("test.dbml", src)
	if len(errs) > 0 {
		t.Fatalf("Scan(%q) errors: %v", src, errs)
	}
	if len(toks) != 2 {
		t.Fatalf("Scan(%q) = %d tokens, want 1 + EOF", src, len(toks)-1)
	}
	return toks[0]
}

func TestMaximalMunch(t *testing.T) {
	// §3.1: longest match wins.
	tests := []struct {
		src  string
		want []token.Kind
	}{
		{"<>", []token.Kind{token.LTGT}},
		{"< >", []token.Kind{token.LT, token.GT}},
		{">1", []token.Kind{token.GT, token.NUMBER}},
		{"user_id2", []token.Kind{token.IDENT}},
		{"''''''", []token.Kind{token.STRING}}, // empty multi-line string, not three empties
	}
	for _, tt := range tests {
		got := kinds(t, tt.src)
		if len(got) != len(tt.want) {
			t.Errorf("kinds(%q) = %v, want %v", tt.src, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("kinds(%q)[%d] = %v, want %v", tt.src, i, got[i], tt.want[i])
			}
		}
	}
}

func TestNumbersAndIdents(t *testing.T) {
	// §3.4 constraint 2 and §3.9.
	tests := []struct {
		src  string
		kind token.Kind
	}{
		{"42", token.NUMBER},
		{"3.14", token.NUMBER},
		{"1e2", token.NUMBER},
		{"1.5e10", token.NUMBER},
		{"3.14e-5", token.NUMBER},
		{"2E+3", token.NUMBER},
		{"2fa_codes", token.IDENT},
		{"1e2abc", token.IDENT},
		{"żółć", token.IDENT},
		{"_x", token.IDENT},
	}
	for _, tt := range tests {
		if got := one(t, tt.src); got.Kind != tt.kind {
			t.Errorf("Scan(%q) kind = %v, want %v", tt.src, got.Kind, tt.kind)
		}
	}
	// 12.3abc is a lexical error (§3.9.3)
	if _, errs := Scan("t", "12.3abc"); len(errs) == 0 {
		t.Error("Scan(12.3abc): want error, got none")
	}
}

func TestEscapes(t *testing.T) {
	// §3.8.
	tests := []struct{ src, val string }{
		{`'a\tb'`, "a\tb"},
		{`'a\nb'`, "a\nb"},
		{`'it\'s'`, "it's"},
		{`'back\\slash'`, `back\slash`},
		{`'A'`, "A"},
		{`'\q'`, "q"},      // unknown escape yields the char
		{"'a\\\nb'", "ab"}, // line continuation
	}
	for _, tt := range tests {
		if got := one(t, tt.src); got.Val != tt.val {
			t.Errorf("Scan(%s) val = %q, want %q", tt.src, got.Val, tt.val)
		}
	}
	if _, errs := Scan("t", `'\u12'`); len(errs) == 0 {
		t.Error(`Scan('\u12'): want error for short unicode escape`)
	}
}

func TestMultilineIndentStripping(t *testing.T) {
	// §3.7.4.
	src := "'''\n  line one\n  line two\n'''"
	got := one(t, src)
	want := "line one\nline two"
	if got.Val != want {
		t.Errorf("multiline val = %q, want %q", got.Val, want)
	}
	if !got.Multi {
		t.Error("Multi flag not set")
	}
}

func TestFuncExprIsRaw(t *testing.T) {
	// §3.12: backslash is not an escape inside backticks.
	got := one(t, "`a\\nb`")
	if got.Val != `a\nb` {
		t.Errorf("funcexpr val = %q, want %q", got.Val, `a\nb`)
	}
}

func TestNewlineFlag(t *testing.T) {
	// §3.2: the NLBefore flag drives statement termination.
	toks, _ := Scan("t", "a b\nc")
	if toks[1].NLBefore {
		t.Error("b should not have NLBefore")
	}
	if !toks[2].NLBefore {
		t.Error("c should have NLBefore")
	}
	// a line comment does not swallow the newline's significance (§3.3.4)
	toks, _ = Scan("t", "a // comment\nc")
	if !toks[1].NLBefore {
		t.Error("token after line comment + newline should have NLBefore")
	}
}

func TestCRLF(t *testing.T) {
	// §3.2.1: CR is discarded; it keeps its byte and has no column
	// (D90), so offsets match the file as written.
	toks, errs := Scan("t", "a\r\nb")
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if !toks[1].NLBefore {
		t.Error("CRLF should still mark NLBefore")
	}
	if p := toks[1].Pos; p.Line() != 2 || p.Column() != 1 || p.Offset() != 3 {
		t.Errorf("b at %s+%d, want 2:1+3", p, p.Offset())
	}
	if e := toks[0].End(); e.Line() != 1 || e.Column() != 2 || e.Offset() != 1 {
		t.Errorf("a ends at %s+%d, want 1:2+1", e, e.Offset())
	}
	// Inside a token it is discarded from the value and kept in the
	// text; between tokens it is neither a token nor whitespace.
	toks, errs = Scan("t", "'''\r\n  ab\r\n'''\r\nx\ry [\r\n")
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if s := toks[0]; s.Val != "ab" || s.Text() != "'''\r\n  ab\r\n'''" || s.End().Line() != 3 || s.End().Column() != 4 {
		t.Errorf("multi-line string = %+v (text %q), want value \"ab\" ending at 3:4", s, s.Text())
	}
	if id := toks[1]; id.Val != "xy" || id.Text() != "x\ry" || id.Pos.Line() != 4 || id.End().Column() != 3 {
		t.Errorf("identifier = %+v (text %q), want value \"xy\" ending at column 3", id, id.Text())
	}
	if br := toks[2]; br.Kind != token.LBRACKET || !br.SpBefore || br.Pos.Column() != 4 {
		t.Errorf("bracket = %+v, want column 4 after a space", br)
	}
}

func TestLexicalErrors(t *testing.T) {
	for _, src := range []string{
		"'unterminated",
		"'line\nbreak'",
		"/* unterminated",
		"`unterminated",
		`"unterminated`,
		"@",
	} {
		if _, errs := Scan("t", src); len(errs) == 0 {
			t.Errorf("Scan(%q): want error, got none", src)
		}
	}
}

func TestQuotedIdent(t *testing.T) {
	got := one(t, `"double precision"`)
	if got.Kind != token.IDENT || !got.Quoted || got.Val != "double precision" {
		t.Errorf("quoted ident = %+v", got)
	}
}

// TestElementStarts pins the element boundary of §3.2.5 in the
// scanner (D88): which tokens begin a top-level element, where a chunk
// scan stops and what its EOF says, and what StartsDecl answers about
// the bytes after a chunk.
func TestElementStarts(t *testing.T) {
	src := "Table a {\n  b int\n}\nx.y\n  indented\n\"quoted\" z\n2fa\nEnum e {\n  v\n}\n/* c */ w\n"
	toks, errs := Scan("t", src)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	var starts []string
	for _, tk := range toks {
		if tk.DeclStart {
			starts = append(starts, tk.Val)
		}
	}
	if want := []string{"Table", "x", "2fa", "Enum"}; strings.Join(starts, " ") != strings.Join(want, " ") {
		t.Errorf("element starts %q, want %q", starts, want)
	}

	chunk, _, end, bound := ScanChunk(token.NewFile("t", src), nil)
	if want := strings.Index(src, "x.y"); end != want || !bound {
		t.Errorf("chunk ends at %d bound=%v, want %d and true", end, bound, want)
	}
	eof := chunk[len(chunk)-1]
	if eof.Kind != token.EOF || eof.Val != ElementStart || eof.Pos.Line() != 4 || eof.Pos.Column() != 1 || !eof.NLBefore {
		t.Errorf("chunk EOF = %+v, want the element start at 4:1", eof)
	}
	if got := len(chunk) - 1; got != 6 { // Table a { b int }
		t.Errorf("chunk holds %d tokens, want 6", got)
	}
	open := "Table a {\n  b int\n"
	if _, _, end, bound := ScanChunk(token.NewFile("t", open), nil); end != len(open) || bound {
		t.Errorf("open chunk ends at %d bound=%v, want %d and false", end, bound, len(open))
	}

	for text, want := range map[string]bool{
		"x.y\n":            true,
		"2fa x\n":          true,
		"Enum e {":         true,
		"  indented\n":     false,
		"\"quoted\" z\n":   false,
		"// c\nTable a {}": false,
		"123\n":            false,
		"{\n":              false,
		"":                 false,
	} {
		if got := StartsDecl(text); got != want {
			t.Errorf("StartsDecl(%q) = %v, want %v", text, got, want)
		}
	}
}

// TestLookaheadStopsAtLineBreak pins the bound a chunk's independence
// rests on (D95): the scanner sees the line break ahead of it and
// nothing past it until it consumes it.
func TestLookaheadStopsAtLineBreak(t *testing.T) {
	s := newScanner(token.NewFile("t", "a\r\nbc\n"), nil)
	if s.next() != 'a' {
		t.Fatal("expected a")
	}
	if got := s.peek(); got != '\n' {
		t.Errorf("peek = %q, want the line break", got)
	}
	if got := s.peekAt(1); got != eof {
		t.Errorf("peekAt(1) = %q, want eof past the line break", got)
	}
	if got := s.peekAt(2); got != eof {
		t.Errorf("peekAt(2) = %q, want eof past the line break", got)
	}
	s.next() // the line break
	if got, want := s.peek(), 'b'; got != want {
		t.Errorf("after the line break peek = %q, want %q", got, want)
	}
	if got, want := s.peekAt(1), 'c'; got != want {
		t.Errorf("after the line break peekAt(1) = %q, want %q", got, want)
	}
	if got := s.peekAt(2); got != '\n' {
		t.Errorf("peekAt(2) = %q, want the next line break itself", got)
	}
	if got := s.peekAt(3); got != eof {
		t.Errorf("peekAt(3) = %q, want eof past the next line break", got)
	}
}
