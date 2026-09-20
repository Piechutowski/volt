// Package ebnf reads the grammar notation of docs/spec.md: ISO/IEC
// 14977 as the spec's Notation section lists it, with nonterminal
// names of lowercase words separated by spaces. It gives tests the
// grammar as data, to check it against itself and against the parser
// (D100).
package ebnf

import (
	"fmt"
	"strconv"
	"strings"
)

// Grammar is a list of rules in definition order.
type Grammar struct {
	Rules  []*Rule
	byName map[string]*Rule
}

// Rule defines one nonterminal.
type Rule struct {
	Name string
	Expr Expr
	Line int // the line of the definition in the source
}

// Lookup returns the rule defining name, or nil.
func (g *Grammar) Lookup(name string) *Rule { return g.byName[name] }

// Add appends a rule; a name already defined is an error.
func (g *Grammar) Add(r *Rule) error {
	if g.byName == nil {
		g.byName = map[string]*Rule{}
	}
	if prev := g.byName[r.Name]; prev != nil {
		return fmt.Errorf("%s is defined twice (lines %d and %d)", r.Name, prev.Line, r.Line)
	}
	g.Rules = append(g.Rules, r)
	g.byName[r.Name] = r
	return nil
}

// Set replaces the rule of the same name in place, or appends.
func (g *Grammar) Set(r *Rule) {
	if g.byName == nil {
		g.byName = map[string]*Rule{}
	}
	if prev := g.byName[r.Name]; prev != nil {
		for i, x := range g.Rules {
			if x == prev {
				g.Rules[i] = r
			}
		}
	} else {
		g.Rules = append(g.Rules, r)
	}
	g.byName[r.Name] = r
}

// Expr is a node of a rule's right-hand side.
type Expr interface{ isExpr() }

// Seq is a concatenation a, b, c; an empty Seq matches the empty
// string (the spec's `empty = ;`).
type Seq struct{ Items []Expr }

// Alt is a choice a | b | c.
type Alt struct{ Items []Expr }

// Opt is an option [ x ].
type Opt struct{ X Expr }

// Rep is a repetition { x }, zero or more.
type Rep struct{ X Expr }

// Times is a counted repetition n * x, exactly n.
type Times struct {
	N int
	X Expr
}

// Except is an exception x - y: what matches x and not y.
type Except struct{ X, Y Expr }

// Term is a terminal "text"; the text is taken literally, without
// escapes, as ISO 14977 has it.
type Term struct{ Text string }

// Special is a special sequence ? text ?, described in prose.
type Special struct{ Text string }

// Ref names a nonterminal.
type Ref struct{ Name string }

func (Seq) isExpr()     {}
func (Alt) isExpr()     {}
func (Opt) isExpr()     {}
func (Rep) isExpr()     {}
func (Times) isExpr()   {}
func (Except) isExpr()  {}
func (Term) isExpr()    {}
func (Special) isExpr() {}
func (Ref) isExpr()     {}

// Parse reads a grammar. A name defined twice is an error: the spec's
// collected grammars repeat the sections' definitions, but each block
// is read on its own.
func Parse(src string) (*Grammar, error) {
	p := &parser{src: src, line: 1}
	p.next()
	g := &Grammar{byName: map[string]*Rule{}}
	for p.tok.kind != tkEOF {
		r, err := p.rule()
		if err != nil {
			return nil, err
		}
		if g.byName[r.Name] != nil {
			return nil, fmt.Errorf("line %d: %s is defined twice (first at line %d)", r.Line, r.Name, g.byName[r.Name].Line)
		}
		g.Rules = append(g.Rules, r)
		g.byName[r.Name] = r
	}
	return g, nil
}

// Refs lists the nonterminals an expression names, in order of first
// appearance.
func Refs(e Expr) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(Expr)
	walk = func(e Expr) {
		switch x := e.(type) {
		case Seq:
			for _, it := range x.Items {
				walk(it)
			}
		case Alt:
			for _, it := range x.Items {
				walk(it)
			}
		case Opt:
			walk(x.X)
		case Rep:
			walk(x.X)
		case Times:
			walk(x.X)
		case Except:
			walk(x.X)
			walk(x.Y)
		case Ref:
			if !seen[x.Name] {
				seen[x.Name] = true
				out = append(out, x.Name)
			}
		}
	}
	walk(e)
	return out
}

// Format writes an expression back in the notation, one line, so two
// definitions can be compared and printed.
func Format(e Expr) string {
	var b strings.Builder
	format(&b, e, 0)
	return b.String()
}

// Precedence for printing: alternation < sequence < exception < factor.
func format(b *strings.Builder, e Expr, prec int) {
	switch x := e.(type) {
	case Seq:
		if len(x.Items) == 0 {
			return
		}
		if prec > 1 && len(x.Items) > 1 {
			b.WriteString("( ")
			defer b.WriteString(" )")
		}
		for i, it := range x.Items {
			if i > 0 {
				b.WriteString(", ")
			}
			format(b, it, 2)
		}
	case Alt:
		if prec > 0 && len(x.Items) > 1 {
			b.WriteString("( ")
			defer b.WriteString(" )")
		}
		for i, it := range x.Items {
			if i > 0 {
				b.WriteString(" | ")
			}
			format(b, it, 1)
		}
	case Opt:
		b.WriteString("[ ")
		format(b, x.X, 0)
		b.WriteString(" ]")
	case Rep:
		b.WriteString("{ ")
		format(b, x.X, 0)
		b.WriteString(" }")
	case Times:
		fmt.Fprintf(b, "%d * ", x.N)
		format(b, x.X, 3)
	case Except:
		if prec > 2 {
			b.WriteString("( ")
			defer b.WriteString(" )")
		}
		format(b, x.X, 3)
		b.WriteString(" - ")
		format(b, x.Y, 3)
	case Term:
		if strings.Contains(x.Text, `"`) {
			b.WriteString("'" + x.Text + "'")
		} else {
			b.WriteString(`"` + x.Text + `"`)
		}
	case Special:
		b.WriteString("? " + x.Text + " ?")
	case Ref:
		b.WriteString(x.Name)
	}
}

/* ===== reading ===== */

type tokKind int

const (
	tkEOF tokKind = iota
	tkName
	tkInt
	tkTerm
	tkSpecial
	tkPunct
)

type token struct {
	kind tokKind
	text string
	line int
}

type parser struct {
	src  string
	pos  int
	line int
	tok  token
	err  error
}

func (p *parser) errorf(line int, format string, args ...any) error {
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

// next scans the next token, skipping layout and (* comments *).
func (p *parser) next() {
	if p.err != nil {
		return
	}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\n':
			p.line++
			p.pos++
		case c == ' ' || c == '\t' || c == '\r':
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "(*"):
			end := strings.Index(p.src[p.pos+2:], "*)")
			if end < 0 {
				p.err = p.errorf(p.line, "unterminated comment")
				p.tok = token{kind: tkEOF, line: p.line}
				return
			}
			p.line += strings.Count(p.src[p.pos:p.pos+2+end], "\n")
			p.pos += 2 + end + 2
		default:
			p.scan()
			return
		}
	}
	p.tok = token{kind: tkEOF, line: p.line}
}

func isLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isDigit(c byte) bool  { return '0' <= c && c <= '9' }

func (p *parser) scan() {
	c := p.src[p.pos]
	start := p.pos
	switch {
	case isLetter(c):
		// words separated by single spaces form one name
		for {
			for p.pos < len(p.src) && (isLetter(p.src[p.pos]) || isDigit(p.src[p.pos])) {
				p.pos++
			}
			if p.pos+1 < len(p.src) && p.src[p.pos] == ' ' && isLetter(p.src[p.pos+1]) {
				p.pos++
				continue
			}
			break
		}
		p.tok = token{kind: tkName, text: p.src[start:p.pos], line: p.line}
	case isDigit(c):
		for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
			p.pos++
		}
		p.tok = token{kind: tkInt, text: p.src[start:p.pos], line: p.line}
	case c == '"' || c == '\'':
		end := strings.IndexByte(p.src[p.pos+1:], c)
		if end < 0 || strings.Contains(p.src[p.pos+1:p.pos+1+end], "\n") {
			p.err = p.errorf(p.line, "unterminated terminal")
			p.tok = token{kind: tkEOF, line: p.line}
			return
		}
		p.tok = token{kind: tkTerm, text: p.src[p.pos+1 : p.pos+1+end], line: p.line}
		p.pos += end + 2
	case c == '?':
		end := strings.IndexByte(p.src[p.pos+1:], '?')
		if end < 0 {
			p.err = p.errorf(p.line, "unterminated special sequence")
			p.tok = token{kind: tkEOF, line: p.line}
			return
		}
		text := strings.Join(strings.Fields(p.src[p.pos+1:p.pos+1+end]), " ")
		p.line += strings.Count(p.src[p.pos:p.pos+1+end], "\n")
		p.tok = token{kind: tkSpecial, text: text, line: p.line}
		p.pos += end + 2
	case strings.IndexByte("=,|[]{}()-*;", c) >= 0:
		p.pos++
		p.tok = token{kind: tkPunct, text: string(c), line: p.line}
	default:
		p.err = p.errorf(p.line, "unexpected character %q", c)
		p.tok = token{kind: tkEOF, line: p.line}
	}
}

func (p *parser) is(punct string) bool { return p.tok.kind == tkPunct && p.tok.text == punct }

func (p *parser) expect(punct string, where string) error {
	if p.err != nil {
		return p.err
	}
	if !p.is(punct) {
		return p.errorf(p.tok.line, "expected %q %s, found %s", punct, where, p.describe())
	}
	p.next()
	return p.err
}

func (p *parser) describe() string {
	switch p.tok.kind {
	case tkEOF:
		return "end of grammar"
	case tkTerm:
		return strconv.Quote(p.tok.text)
	default:
		return p.tok.text
	}
}

func (p *parser) rule() (*Rule, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.tok.kind != tkName {
		return nil, p.errorf(p.tok.line, "expected a rule name, found %s", p.describe())
	}
	r := &Rule{Name: p.tok.text, Line: p.tok.line}
	p.next()
	if err := p.expect("=", "after "+r.Name); err != nil {
		return nil, err
	}
	e, err := p.alt()
	if err != nil {
		return nil, err
	}
	r.Expr = e
	if err := p.expect(";", "ending "+r.Name); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *parser) alt() (Expr, error) {
	var items []Expr
	for {
		s, err := p.seq()
		if err != nil {
			return nil, err
		}
		items = append(items, s)
		if !p.is("|") {
			break
		}
		p.next()
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return Alt{Items: items}, nil
}

// seq reads a possibly empty sequence: empty before ";", "|" and a
// closing bracket.
func (p *parser) seq() (Expr, error) {
	var items []Expr
	if p.tok.kind == tkEOF || p.is(";") || p.is("|") || p.is("]") || p.is("}") || p.is(")") {
		return Seq{}, p.err
	}
	for {
		f, err := p.factor()
		if err != nil {
			return nil, err
		}
		items = append(items, f)
		if !p.is(",") {
			break
		}
		p.next()
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return Seq{Items: items}, nil
}

func (p *parser) factor() (Expr, error) {
	x, err := p.primary()
	if err != nil {
		return nil, err
	}
	if p.is("-") {
		p.next()
		y, err := p.primary()
		if err != nil {
			return nil, err
		}
		return Except{X: x, Y: y}, nil
	}
	return x, nil
}

func (p *parser) primary() (Expr, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.tok.kind == tkInt {
		n, _ := strconv.Atoi(p.tok.text)
		p.next()
		if err := p.expect("*", "after a repetition count"); err != nil {
			return nil, err
		}
		x, err := p.atom()
		if err != nil {
			return nil, err
		}
		return Times{N: n, X: x}, nil
	}
	return p.atom()
}

func (p *parser) atom() (Expr, error) {
	if p.err != nil {
		return nil, p.err
	}
	switch {
	case p.tok.kind == tkName:
		e := Ref{Name: p.tok.text}
		p.next()
		return e, p.err
	case p.tok.kind == tkTerm:
		e := Term{Text: p.tok.text}
		p.next()
		return e, p.err
	case p.tok.kind == tkSpecial:
		e := Special{Text: p.tok.text}
		p.next()
		return e, p.err
	case p.is("["):
		p.next()
		x, err := p.alt()
		if err != nil {
			return nil, err
		}
		return Opt{X: x}, p.expect("]", "closing an option")
	case p.is("{"):
		p.next()
		x, err := p.alt()
		if err != nil {
			return nil, err
		}
		return Rep{X: x}, p.expect("}", "closing a repetition")
	case p.is("("):
		p.next()
		x, err := p.alt()
		if err != nil {
			return nil, err
		}
		return x, p.expect(")", "closing a group")
	default:
		return nil, p.errorf(p.tok.line, "expected a name, a terminal, a special sequence or a bracket, found %s", p.describe())
	}
}
