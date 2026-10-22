// Package scanner tokenizes Volt source (spec §lexical-structure), playing the role of
// go/scanner in the Go toolchain.
//
// Two rules govern tokenization (spec §tokenization): left-to-right single pass,
// and longest match. Newlines are not emitted as tokens; instead each
// token records whether a line break precedes it (Token.NLBefore).
package scanner

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// Scan tokenizes src. It always returns the tokens produced so far, with a
// terminating EOF token, plus any lexical errors found (spec §lexical-structure). Scanning
// continues after an error where possible.
func Scan(filename, src string) ([]token.Token, []diag.Diagnostic) {
	return ScanFile(token.NewFile(filename, src))
}

// ScanFile scans a whole file as written; every position is in f. A
// carriage return is discarded wherever it appears (§line-structure-and-whitespace) and stays
// in the text: it keeps its byte, so offsets match the file, and has
// no column. Every token that begins a top-level element
// (§line-structure-and-whitespace) is flagged; nothing stops there.
func ScanFile(f *token.File) ([]token.Token, []diag.Diagnostic) {
	s := newScanner(f, make([]token.Token, 0, len(f.Src)/4+16))
	s.run()
	s.emit(token.EOF, "")
	return s.toks, s.errs
}

// ScanChunk scans the first top-level element of f's text (§line-structure-and-whitespace):
// the tokens before the next element start after the first
// byte, then an EOF standing at that start, which is the end returned
// and whose text names what it is; without one, the whole text and an
// EOF at its end. bound reports the former. buf is a token slice to
// reuse, or nil.
func ScanChunk(f *token.File, buf []token.Token) (toks []token.Token, errs []diag.Diagnostic, end int, bound bool) {
	s := newScanner(f, buf)
	s.chunk = true
	s.run()
	if s.done {
		s.pos = s.start
		s.emit(token.EOF, ElementStart)
		return s.toks, s.errs, s.start.Offset, true
	}
	s.emit(token.EOF, "")
	return s.toks, s.errs, len(s.src), false
}

// ElementStart is how a chunk's EOF reads in a diagnostic: the token
// the parser found is the next element's start, not the file's end.
const ElementStart = "the start of the next element (§line-structure-and-whitespace)"

// StartsDecl reports whether text begins, at its very first byte, a
// top-level element (§line-structure-and-whitespace). One token is scanned, whatever the
// text's length.
func StartsDecl(text string) bool {
	var buf [1]token.Token
	s := newScanner(token.NewFile("", text), buf[:0])
	for state := anyScan; state != nil && len(s.toks) == 0; {
		state = state(s)
	}
	return len(s.toks) == 1 && s.toks[0].DeclStart && s.toks[0].Pos.Offset() == 0
}

func newScanner(f *token.File, buf []token.Token) *Scanner {
	s := &Scanner{src: f.Src, file: f, pos: cursor{line: 1, col: 1}, toks: buf[:0]}
	s.nlBefore = true
	return s
}

func (s *Scanner) run() {
	for state := anyScan; state != nil && !s.done; {
		state = state(s)
	}
}

type stateFn func(*Scanner) stateFn

type Scanner struct {
	src  string
	file *token.File

	pos   cursor
	start cursor

	nlBefore bool
	spBefore bool
	cr       bool

	depth       int
	chunk, done bool

	toks []token.Token
	errs []diag.Diagnostic

	val strings.Builder
}

type cursor struct {
	Offset, line, col int
}

func (c cursor) position(f *token.File) token.Position { return f.At(c.Offset, c.line, c.col) }

const eof = rune(-1)

func (s *Scanner) crSkip(off int) int {
	for off < len(s.src) && s.src[off] == '\r' {
		off++
	}
	return off
}

func (s *Scanner) peek() rune {
	off := s.crSkip(s.pos.Offset)
	if off >= len(s.src) {
		return eof
	}
	if c := s.src[off]; c < utf8.RuneSelf {
		return rune(c)
	}
	r, _ := utf8.DecodeRuneInString(s.src[off:])
	return r
}

func (s *Scanner) peekAt(n int) rune {
	off := s.crSkip(s.pos.Offset)
	for ; n > 0; n-- {
		if off >= len(s.src) || s.src[off] == '\n' {
			return eof
		}
		_, w := utf8.DecodeRuneInString(s.src[off:])
		off = s.crSkip(off + w)
	}
	if off >= len(s.src) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(s.src[off:])
	return r
}

func (s *Scanner) next() rune {
	if off := s.crSkip(s.pos.Offset); off != s.pos.Offset {
		s.pos.Offset = off
		s.cr = true
	}
	if s.pos.Offset >= len(s.src) {
		return eof
	}
	r, w := rune(s.src[s.pos.Offset]), 1
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRuneInString(s.src[s.pos.Offset:])
	}
	s.pos.Offset += w
	if r == '\n' {
		s.pos.line++
		s.pos.col = 1
	} else {
		s.pos.col++
	}
	return r
}

func (s *Scanner) mark() {
	s.pos.Offset = s.crSkip(s.pos.Offset)
	s.start = s.pos
	s.val.Reset()
	s.cr = false
}

func (s *Scanner) raw() string { return s.src[s.start.Offset:s.pos.Offset] }

func (s *Scanner) rawVal() string {
	if s.cr {
		return strings.ReplaceAll(s.raw(), "\r", "")
	}
	return s.raw()
}

func (s *Scanner) emit(kind token.Kind, val string) {
	s.tokEmit(token.Token{Kind: kind, Val: val})
}

func (s *Scanner) tokEmit(t token.Token) {
	t.Pos = s.start.position(s.file)
	t.Len = int32(s.pos.Offset - s.start.Offset)
	t.NLBefore = s.nlBefore
	t.SpBefore = s.spBefore || s.nlBefore
	t.DeclStart = t.Kind == token.IDENT && s.start.col == 1 && s.depth == 0
	if t.DeclStart && s.chunk && s.start.Offset > 0 {
		s.done = true
		return
	}
	switch t.Kind {
	case token.LBRACE:
		s.depth++
	case token.RBRACE:
		if s.depth > 0 {
			s.depth--
		}
	}
	s.toks = append(s.toks, t)
	s.nlBefore, s.spBefore = false, false
}

func (s *Scanner) errorf(code, format string, args ...any) {
	s.errs = append(s.errs, diag.Errorf(s.start.position(s.file), code, format, args...))
}

func isLetter(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
	}
	return unicode.IsLetter(r)
}
func isDigit(r rune) bool { return '0' <= r && r <= '9' }
func isIdentChar(r rune) bool {
	return isLetter(r) || isDigit(r) || (r >= utf8.RuneSelf && unicode.IsDigit(r))
}

func anyScan(s *Scanner) stateFn {
	for {
		if r := s.peek(); r == ' ' || r == '\t' {
			s.next()
			s.spBefore = true
		} else if r == '\n' {
			s.next()
			s.nlBefore = true
		} else {
			break
		}
	}
	s.mark()
	switch r := s.peek(); {
	case r == eof:
		return nil
	case r == '/' && s.peekAt(1) == '/':
		return lineCommentScan
	case r == '/' && s.peekAt(1) == '*':
		return blockCommentScan
	case r == '"':
		if s.peekAt(1) == '"' && s.peekAt(2) == '"' {
			return multiStringScan
		}
		return stringScan
	case r == '\'':
		return singleQuoteScan
	case r == '`':
		return funcExprScan
	case isDigit(r):
		return numberOrIdentScan
	case isLetter(r):
		return identScan
	default:
		return operatorScan
	}
}

func lineCommentScan(s *Scanner) stateFn {
	for r := s.peek(); r != '\n' && r != eof; r = s.peek() {
		s.next()
	}
	s.spBefore = true
	return anyScan
}

func blockCommentScan(s *Scanner) stateFn {
	s.next()
	s.next()
	for {
		switch {
		case s.peek() == eof:
			s.errorf("syntax", "unterminated block comment (§comments)")
			return nil
		case s.peek() == '*' && s.peekAt(1) == '/':
			s.next()
			s.next()
			s.spBefore = true
			return anyScan
		default:
			s.next()
		}
	}
}

const (
	msgBackslashNewline = `a backslash before a line break continues nothing: a string ends on its line, and a multi-line string is written """...""" (§escape-sequences)`
	msgUnknownEscape    = `unknown escape sequence \%c: a string takes \\, \", \n and \t only (§escape-sequences)`
	msgNewlineInString  = `newline in a string: a string ends on its line, and a multi-line string is written """...""" (§string-literals)`
	msgSingleQuote      = `a string is written in double quotes, "...", not single quotes (§string-literals)`
	msgTripleSingle     = `a multi-line string is written """...""", not '''...''' (§multi-line-string-literals)`
	msgQuoteBeforeClose = `a multi-line string cannot end with a double quote: write it \" before the closing """ (§multi-line-string-literals)`
)

func (s *Scanner) escapeScan(at cursor) bool {
	switch r := s.peek(); r {
	case '\\':
		s.next()
		s.val.WriteByte('\\')
	case '"':
		s.next()
		s.val.WriteByte('"')
	case 'n':
		s.next()
		s.val.WriteByte('\n')
	case 't':
		s.next()
		s.val.WriteByte('\t')
	case '\n':
		s.errs = append(s.errs, diag.Errorf(at.position(s.file), "syntax", msgBackslashNewline))
		return false
	case eof:
	default:
		s.next()
		s.errs = append(s.errs, diag.Errorf(at.position(s.file), "syntax", msgUnknownEscape, r))
	}
	return true
}

func stringScan(s *Scanner) stateFn {
	s.next()
	for {
		switch r := s.peek(); r {
		case eof:
			s.errorf("syntax", "unterminated string (§string-literals)")
			return nil
		case '\n':
			s.errorf("syntax", msgNewlineInString)
			return anyScan
		case '"':
			s.next()
			s.tokEmit(token.Token{Kind: token.STRING, Val: s.val.String()})
			return anyScan
		case '\\':
			at := s.pos
			s.next()
			if !s.escapeScan(at) {
				s.next()
			}
		default:
			s.val.WriteRune(s.next())
		}
	}
}

func multiStringScan(s *Scanner) stateFn {
	s.next()
	s.next()
	s.next()
	for {
		switch r := s.peek(); {
		case r == eof:
			s.errorf("syntax", "unterminated multi-line string (§multi-line-string-literals)")
			return nil
		case r == '"' && s.peekAt(1) == '"' && s.peekAt(2) == '"':
			at := s.pos
			s.next()
			s.next()
			s.next()
			if s.peek() == '"' {
				s.errs = append(s.errs, diag.Errorf(at.position(s.file), "syntax", msgQuoteBeforeClose))
				for s.peek() == '"' {
					s.next()
					s.val.WriteByte('"')
				}
			}
			s.tokEmit(token.Token{Kind: token.STRING, Val: indentStrip(s.val.String()), Multi: true})
			return anyScan
		case r == '\\':
			at := s.pos
			s.next()
			s.escapeScan(at)
		default:
			s.val.WriteRune(s.next())
		}
	}
}

func singleQuoteScan(s *Scanner) stateFn {
	if s.peekAt(1) == '\'' && s.peekAt(2) == '\'' {
		s.next()
		s.next()
		s.next()
		for {
			switch r := s.peek(); {
			case r == eof:
				s.errorf("syntax", msgTripleSingle)
				return nil
			case r == '\'' && s.peekAt(1) == '\'' && s.peekAt(2) == '\'':
				s.next()
				s.next()
				s.next()
				s.errorf("syntax", msgTripleSingle)
				s.tokEmit(token.Token{Kind: token.STRING, Val: indentStrip(s.val.String()), Multi: true})
				return anyScan
			default:
				s.val.WriteRune(s.next())
			}
		}
	}
	s.next()
	for {
		switch r := s.peek(); r {
		case eof, '\n':
			s.errorf("syntax", msgSingleQuote)
			return anyScan
		case '\'':
			s.next()
			s.errorf("syntax", msgSingleQuote)
			s.tokEmit(token.Token{Kind: token.STRING, Val: s.val.String()})
			return anyScan
		case '\\':
			s.next()
			if p := s.peek(); p != '\n' && p != eof {
				s.val.WriteRune(s.next())
			}
		default:
			s.val.WriteRune(s.next())
		}
	}
}

func indentStrip(v string) string {
	blank := func(ln string) bool { return strings.Trim(ln, " \t") == "" }
	lines := strings.Split(v, "\n")
	if len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	if len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	m := -1
	for _, ln := range lines {
		if blank(ln) {
			continue
		}
		n := len(ln) - len(strings.TrimLeft(ln, " "))
		if m < 0 || n < m {
			m = n
		}
	}
	if m > 0 {
		for i, ln := range lines {
			n := len(ln) - len(strings.TrimLeft(ln, " "))
			lines[i] = ln[min(n, m):]
		}
	}
	return strings.Join(lines, "\n")
}

func funcExprScan(s *Scanner) stateFn {
	s.next()
	for {
		switch r := s.peek(); r {
		case eof:
			s.errorf("syntax", "unterminated expression literal (§expression-literals)")
			return nil
		case '`':
			s.next()
			s.tokEmit(token.Token{Kind: token.FUNCEXPR, Val: s.val.String()})
			return anyScan
		default:
			s.val.WriteRune(s.next())
		}
	}
}

func identScan(s *Scanner) stateFn {
	off := s.pos.Offset
	for off < len(s.src) && asciiIdent[s.src[off]] {
		off++
	}
	s.pos.col += off - s.pos.Offset
	s.pos.Offset = off
	s.identRest()
	s.tokEmit(token.Token{Kind: token.IDENT, Val: s.rawVal()})
	return anyScan
}

func (s *Scanner) identRest() {
	for {
		r := s.peek()
		switch {
		case isIdentChar(r):
			s.next()
		case r >= utf8.RuneSelf && unicode.IsMark(r):
			at := s.pos
			s.next()
			s.errs = append(s.errs, diag.Errorf(at.position(s.file), "syntax", "U+%04X, a combining mark, cannot stand in a name: a name is a Go identifier, letters, digits and underscores (§identifiers)", r))
		default:
			return
		}
	}
}

var asciiIdent = func() (t [256]bool) {
	for c := 0; c < 256; c++ {
		t[c] = c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
	}
	return t
}()

func numberOrIdentScan(s *Scanner) stateFn {
	nDots, hasLetter := 0, false
	for {
		switch r := s.peek(); {
		case isDigit(r):
			s.next()
		case r == '.' && !hasLetter && nDots == 0 && isDigit(s.peekAt(1)):
			nDots++
			s.next()
		case (r == 'e' || r == 'E') && !hasLetter && s.validExponent():
			s.next()
			if s.peek() == '+' || s.peek() == '-' {
				s.next()
			}
			for isDigit(s.peek()) {
				s.next()
			}
			if isIdentChar(s.peek()) {
				hasLetter = true
				continue
			}
			s.tokEmit(token.Token{Kind: token.NUMBER, Val: s.rawVal()})
			return anyScan
		case isLetter(r):
			hasLetter = true
			s.next()
			s.identRest()
		default:
			if hasLetter {
				if nDots > 0 {
					s.errorf("syntax", "invalid number %q (§numeric-literals)", s.rawVal())
					s.tokEmit(token.Token{Kind: token.ILLEGAL, Val: s.rawVal()})
					return anyScan
				}
				s.tokEmit(token.Token{Kind: token.IDENT, Val: s.rawVal()})
				return anyScan
			}
			s.tokEmit(token.Token{Kind: token.NUMBER, Val: s.rawVal()})
			return anyScan
		}
	}
}

func (s *Scanner) validExponent() bool {
	c := s.peekAt(1)
	if isDigit(c) {
		return true
	}
	if c == '+' || c == '-' {
		return isDigit(s.peekAt(2))
	}
	return false
}

var singleOps = map[rune]token.Kind{
	'{': token.LBRACE, '}': token.RBRACE,
	'[': token.LBRACKET, ']': token.RBRACKET,
	'(': token.LPAREN, ')': token.RPAREN,
	',': token.COMMA, ':': token.COLON, '.': token.DOT,
	'~': token.TILDE, '*': token.STAR, '/': token.SLASH,
	'-': token.MINUS, '+': token.PLUS, '=': token.EQ,
	'\\': token.BACKSLASH,
}

func operatorScan(s *Scanner) stateFn {
	r := s.next()
	if r == '<' {
		if s.peek() == '=' {
			s.next()
			s.emit(token.LE, "<=")
		} else {
			s.emit(token.LT, "<")
		}
		return anyScan
	}
	if r == '>' {
		if s.peek() == '=' {
			s.next()
			s.emit(token.GE, ">=")
		} else {
			s.emit(token.GT, ">")
		}
		return anyScan
	}
	if r == '!' && s.peek() == '=' {
		s.next()
		s.emit(token.NEQ, "!=")
		return anyScan
	}
	if r == '\\' && s.peek() == '\n' {
		s.errorf("syntax", "a backslash before a line break continues nothing: Volt has no line continuation (§escape-sequences)")
		return anyScan
	}
	if k, ok := singleOps[r]; ok {
		s.emit(k, string(r))
		return anyScan
	}
	if unicode.IsMark(r) || unicode.IsDigit(r) {
		s.errorf("syntax", "%q (U+%04X) cannot begin a name: a name is a Go identifier, a letter or underscore followed by letters, digits and underscores (§identifiers)", string(r), r)
		s.identRest()
		s.tokEmit(token.Token{Kind: token.IDENT, Val: s.rawVal()})
		return anyScan
	}
	s.errorf("syntax", "unexpected character %q (§lexical-structure)", string(r))
	s.tokEmit(token.Token{Kind: token.ILLEGAL, Val: string(r)})
	return anyScan
}

// Quote returns s as a Volt string literal, "..." with the four escapes
// and every other character as itself (§string-literals,
// §escape-sequences). It reports false when s holds a carriage return,
// the one character Volt source cannot hold, since the scanner discards
// it (§line-structure-and-whitespace).
func Quote(s string) (string, bool) {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			return "", false
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String(), true
}
