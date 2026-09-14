// Package token defines the lexical tokens of DBML and source positions,
// mirroring the role of go/token in the Go toolchain.
package token

import "fmt"

// File is the source positions point into: its name and text held
// once, so a position is a pointer plus three offsets and a token
// borrows its text from here instead of carrying it (D81, PERF-9).
type File struct {
	Name string
	Src  string
}

// NewFile names a source text.
func NewFile(name, src string) *File { return &File{Name: name, Src: src} }

// At is the position in f at the given byte offset, 1-based line and
// 1-based rune column.
func (f *File) At(offset, line, column int) Position {
	return Position{file: f, Offset: int32(offset), Line: int32(line), Column: int32(column)}
}

// Position is a source location. Unlike go/token, DBML compiles one file at
// a time, so a plain struct replaces the FileSet/Pos machinery.
type Position struct {
	file   *File
	Offset int32 // byte offset, 0-based
	Line   int32 // 1-based
	Column int32 // 1-based, in runes
}

// At is a position in a file known by name only: a Go file a reference
// resolves to, a synthetic position at the start of a file.
func At(filename string, offset, line, column int) Position {
	return (&File{Name: filename}).At(offset, line, column)
}

// Filename is the name of the file the position is in; "" when none.
func (p Position) Filename() string {
	if p.file == nil {
		return ""
	}
	return p.file.Name
}

// File is the source the position points into; nil when known by name
// only.
func (p Position) File() *File { return p.file }

func (p Position) String() string {
	name := p.Filename()
	if name == "" {
		name = "<input>"
	}
	return fmt.Sprintf("%s:%d:%d", name, p.Line, p.Column)
}

func (p Position) IsValid() bool { return p.Line > 0 }

// Kind is the set of lexical token kinds (spec §3).
type Kind uint8

const (
	ILLEGAL Kind = iota
	EOF

	IDENT    // users, "quoted name" (spec §3.4)
	STRING   // 'text', '''multi line''' (spec §3.6, §3.7)
	NUMBER   // 42, 3.14, 1.5e10 (spec §3.9)
	COLOR    // #3498DB (spec §3.11)
	FUNCEXPR // `now()` (spec §3.12)

	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]
	LPAREN   // (
	RPAREN   // )
	COMMA    // ,
	COLON    // :
	DOT      // .
	TILDE    // ~
	STAR     // *
	SLASH    // / (Volt route paths and import paths, spec §V3-§V5)

	LT    // <
	GT    // >
	LTGT  // <>
	MINUS // -

	// Predicate and group operators (spec §V9-§V10)
	PLUS      // +
	BACKSLASH // \ (group set difference, §V9.3)
	EQ        // =
	NEQ       // !=
	LE        // <=
	GE        // >=
)

var kindNames = [...]string{
	ILLEGAL:   "illegal token",
	EOF:       "end of file",
	IDENT:     "identifier",
	STRING:    "string",
	NUMBER:    "number",
	COLOR:     "color",
	FUNCEXPR:  "expression literal",
	LBRACE:    "'{'",
	RBRACE:    "'}'",
	LBRACKET:  "'['",
	RBRACKET:  "']'",
	LPAREN:    "'('",
	RPAREN:    "')'",
	COMMA:     "','",
	COLON:     "':'",
	DOT:       "'.'",
	TILDE:     "'~'",
	STAR:      "'*'",
	SLASH:     "'/'",
	LT:        "'<'",
	GT:        "'>'",
	LTGT:      "'<>'",
	MINUS:     "'-'",
	PLUS:      "'+'",
	BACKSLASH: "'\\'",
	EQ:        "'='",
	NEQ:       "'!='",
	LE:        "'<='",
	GE:        "'>='",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Token is one lexical token: 48 bytes, two pointers. Val holds the
// processed value (escape sequences applied, quotes stripped); the raw
// source text is borrowed from the file through Text (D81, PERF-9).
type Token struct {
	Kind Kind

	Quoted bool // IDENT written as "quoted identifier"
	Multi  bool // STRING written as '''multi-line'''

	// NLBefore reports whether a line break (or start of file) precedes
	// this token. DBML statements are newline-terminated; this flag plays
	// the same role as Go's automatic semicolon insertion.
	NLBefore bool
	// SpBefore reports whether any whitespace precedes this token.
	SpBefore bool

	Len int32 // bytes of source text; 0 for a synthetic token, whose Text is its Val

	Val string
	Pos Position
}

// Synthetic is a token that stands for text the source does not
// contain: an expanded route segment, a resource parameter. Its Text
// is its Val and its position is the one given.
func Synthetic(kind Kind, val string, at Position) Token {
	return Token{Kind: kind, Val: val, Pos: at}
}

// Text is the raw source text of the token.
func (t Token) Text() string {
	if t.Len == 0 || t.Pos.file == nil {
		return t.Val
	}
	return t.Pos.file.Src[t.Pos.Offset : t.Pos.Offset+t.Len]
}

func (t Token) String() string {
	if text := t.Text(); text != "" {
		return fmt.Sprintf("%s %q", t.Kind, text)
	}
	return t.Kind.String()
}

// End returns the position immediately after the token.
func (t Token) End() Position {
	end := t.Pos
	for _, r := range t.Text() {
		end.Offset++
		if r == '\n' {
			end.Line++
			end.Column = 1
		} else {
			end.Column++
		}
	}
	return end
}
