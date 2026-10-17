// Package scanner tokenizes DBML source (spec §lexical-structure), playing the role of
// go/scanner in the Go toolchain.
//
// Two rules govern tokenization (spec §tokenization): left-to-right single pass,
// and longest match. Newlines are not emitted as tokens; instead each
// token records whether a line break precedes it (Token.NLBefore).
package scanner

import (
	"strconv"
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
	t.DeclStart = t.Kind == token.IDENT && !t.Quoted && s.start.col == 1 && s.depth == 0
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
	return r == '_' || unicode.IsLetter(r) || unicode.IsMark(r)
}
func isDigit(r rune) bool     { return '0' <= r && r <= '9' }
func isIdentChar(r rune) bool { return isLetter(r) || isDigit(r) }
func isHex(r rune) bool {
	return isDigit(r) || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
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
	case r == '\'':
		if s.peekAt(1) == '\'' && s.peekAt(2) == '\'' {
			return multiStringScan
		}
		return stringScan
	case r == '"':
		return quotedIdentScan
	case r == '`':
		return funcExprScan
	case r == '#':
		return colorScan
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

func (s *Scanner) escapeScan() {
	switch r := s.next(); r {
	case 't':
		s.val.WriteByte('\t')
	case 'n':
		s.val.WriteByte('\n')
	case 'r':
		s.val.WriteByte('\r')
	case '0':
		s.val.WriteByte(0)
	case 'b':
		s.val.WriteByte('\b')
	case 'v':
		s.val.WriteByte('\v')
	case 'f':
		s.val.WriteByte('\f')
	case '\n':
	case 'u':
		hex := make([]rune, 0, 4)
		for i := 0; i < 4; i++ {
			if !isHex(s.peek()) {
				s.errorf("syntax", `invalid unicode escape '\u%s': exactly 4 hex digits required (§escape-sequences)`, string(hex))
				return
			}
			hex = append(hex, s.next())
		}
		n, _ := strconv.ParseUint(string(hex), 16, 32)
		s.val.WriteRune(rune(n))
	case eof:
		s.val.WriteByte('\\')
	default:
		s.val.WriteRune(r)
	}
}

func stringScan(s *Scanner) stateFn {
	s.next()
	for {
		switch r := s.peek(); r {
		case eof:
			s.errorf("syntax", "unterminated string (§string-literals)")
			return nil
		case '\n':
			s.errorf("syntax", "newline in single-line string (§string-literals)")
			return anyScan
		case '\'':
			s.next()
			s.tokEmit(token.Token{Kind: token.STRING, Val: s.val.String()})
			return anyScan
		case '\\':
			s.next()
			s.escapeScan()
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
		case r == '\'' && s.peekAt(1) == '\'' && s.peekAt(2) == '\'':
			s.next()
			s.next()
			s.next()
			s.tokEmit(token.Token{Kind: token.STRING, Val: indentStrip(s.val.String()), Multi: true})
			return anyScan
		case r == '\\':
			s.next()
			s.escapeScan()
		default:
			s.val.WriteRune(s.next())
		}
	}
}

func indentStrip(v string) string {
	lines := strings.Split(v, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	min := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		n := len(ln) - len(strings.TrimLeft(ln, " "))
		if min < 0 || n < min {
			min = n
		}
	}
	if min > 0 {
		for i, ln := range lines {
			if len(ln) >= min {
				lines[i] = ln[min:]
			} else {
				lines[i] = strings.TrimLeft(ln, " ")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func quotedIdentScan(s *Scanner) stateFn {
	s.next()
	for {
		switch r := s.peek(); r {
		case eof:
			s.errorf("syntax", "unterminated quoted identifier (§identifiers)")
			return nil
		case '\n':
			s.errorf("syntax", "newline in quoted identifier (§identifiers)")
			return anyScan
		case '"':
			s.next()
			s.tokEmit(token.Token{Kind: token.IDENT, Val: s.val.String(), Quoted: true})
			return anyScan
		case '\\':
			s.next()
			s.escapeScan()
		default:
			s.val.WriteRune(s.next())
		}
	}
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

func colorScan(s *Scanner) stateFn {
	s.next()
	for isIdentChar(s.peek()) {
		s.next()
	}
	s.tokEmit(token.Token{Kind: token.COLOR, Val: strings.TrimPrefix(s.rawVal(), "#")})
	return anyScan
}

func identScan(s *Scanner) stateFn {
	off := s.pos.Offset
	for off < len(s.src) && asciiIdent[s.src[off]] {
		off++
	}
	s.pos.col += off - s.pos.Offset
	s.pos.Offset = off
	for isIdentChar(s.peek()) {
		s.next()
	}
	s.tokEmit(token.Token{Kind: token.IDENT, Val: s.rawVal()})
	return anyScan
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
		switch s.peek() {
		case '>':
			s.next()
			s.emit(token.LTGT, "<>")
		case '=':
			s.next()
			s.emit(token.LE, "<=")
		default:
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
	if k, ok := singleOps[r]; ok {
		s.emit(k, string(r))
		return anyScan
	}
	s.errorf("syntax", "unexpected character %q (§lexical-structure)", string(r))
	s.tokEmit(token.Token{Kind: token.ILLEGAL, Val: string(r)})
	return anyScan
}
