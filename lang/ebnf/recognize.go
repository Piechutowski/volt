package ebnf

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Language is a grammar with the lexical facts a recognizer needs:
// which rules are tokens, what may lie between the symbols of the
// others, where a required newline is satisfied without one, where
// symbols are contiguous, and how a program is cut into elements.
type Language struct {
	G *Grammar

	// Start is the rule whose sentences are programs; the grammar's
	// first rule when empty.
	Start string

	// Lexical names the rules whose sentences are character
	// sequences: nothing lies between their symbols and, in a
	// syntactic rule, a match is the longest one (§tokenization).
	Lexical map[string]bool

	// Layout is what may lie between two symbols of a syntactic rule
	// (§line-structure-and-whitespace, §comments): spaces, tabs and comments. A line
	// break is layout too, except before a symbol that requires one.
	Layout Expr

	// Newline is the name of the rule a line-oriented production ends
	// with. In a syntactic rule it is also satisfied, without being
	// consumed, by a following "}" and by the end of the text; and a
	// production that ends with it lies on one line: a line break
	// inside it is layout only inside brackets (§line-structure-and-whitespace).
	Newline string

	// Contiguous names the syntactic rules whose symbols admit no
	// layout between them, nested rules included (§route-paths, §imports,
	// §predicates).
	Contiguous map[string]bool

	// SameLine names the rules that a line break may not precede
	// where they are referenced (§settings-lists: a settings list is on
	// the line of its construct).
	SameLine map[string]bool

	// NoBreakAfter names the terminals that a line break may not
	// immediately follow outside a lexical rule (§escape-sequences: a
	// backslash before a line break continues nothing).
	NoBreakAfter map[string]bool

	// Specials matches one character for each special sequence of
	// the grammar, by its text; a special sequence without an entry
	// is an error when the language is checked.
	Specials map[string]func(r rune) bool

	// Witness is one character for each special sequence, the one
	// derived sentences use; its matcher must accept it.
	Witness map[string]string

	// Cut says the start rule's text is cut at every element start
	// (§line-structure-and-whitespace), each piece a sentence of that rule on its own.
	Cut bool

	puncts []string
}

// Check reports what the recognizer cannot work with: a reference to
// no rule, a special sequence without a matcher, a lexical name that
// is not a rule.
func (l *Language) Check() error {
	var problems []string
	for _, r := range l.G.Rules {
		for _, name := range Refs(r.Expr) {
			if l.G.Lookup(name) == nil {
				problems = append(problems, fmt.Sprintf("%s (line %d) references %s, which is not defined", r.Name, r.Line, name))
			}
		}
		walk(r.Expr, func(e Expr) {
			if s, ok := e.(Special); ok && l.Specials[s.Text] == nil {
				problems = append(problems, fmt.Sprintf("%s (line %d): no matcher for the special sequence ? %s ?", r.Name, r.Line, s.Text))
			}
		})
	}
	for name := range l.Lexical {
		if l.G.Lookup(name) == nil {
			problems = append(problems, fmt.Sprintf("lexical rule %s is not defined", name))
		}
	}
	if l.Newline != "" && l.G.Lookup(l.Newline) == nil {
		problems = append(problems, fmt.Sprintf("newline rule %s is not defined", l.Newline))
	}
	if l.G.Lookup(l.start()) == nil {
		problems = append(problems, fmt.Sprintf("start rule %q is not defined", l.start()))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func walk(e Expr, f func(Expr)) {
	f(e)
	switch x := e.(type) {
	case Seq:
		for _, it := range x.Items {
			walk(it, f)
		}
	case Alt:
		for _, it := range x.Items {
			walk(it, f)
		}
	case Opt:
		walk(x.X, f)
	case Rep:
		walk(x.X, f)
	case Times:
		walk(x.X, f)
	case Except:
		walk(x.X, f)
		walk(x.Y, f)
	}
}

// Accepts reports whether text is a sentence of the named rule: the
// rule matches it whole, layout at either end included when the rule
// is syntactic. The grammar's first rule, when Cut is set, is matched
// piecewise: the text is cut at every element start (§line-structure-and-whitespace) and
// every piece is a sentence of the rule on its own.
func (l *Language) Accepts(start, text string) bool {
	m := &matcher{l: l, text: text, memo: map[memoKey][]int{}, active: map[memoKey]bool{}}
	if l.Cut && start == l.start() {
		return m.program()
	}
	ctx := context{lexical: l.Lexical[start], contiguous: l.Contiguous[start]}
	from := 0
	if !ctx.lexical {
		from = m.layout(0, true)
	}
	for _, end := range m.rule(start, from, ctx) {
		if !ctx.lexical {
			end = m.layout(end, true)
		}
		if end == len(text) {
			return true
		}
	}
	return false
}

func (l *Language) start() string {
	if l.Start != "" {
		return l.Start
	}
	if len(l.G.Rules) > 0 {
		return l.G.Rules[0].Name
	}
	return ""
}

func (m *matcher) program() bool {
	bounds := append(ElementStarts(m.text), len(m.text))
	start := m.l.G.Lookup(m.l.start())
	pos := 0
	for _, next := range bounds {
		ok := false
		for _, end := range m.expr(start.Expr, m.layout(pos, true), context{}) {
			if end <= next && m.layout(end, true) == next {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
		pos = next
	}
	return true
}

// ElementStarts lists the offsets at which a top-level element begins
// (§line-structure-and-whitespace): an identifier in the first column of a line,
// outside every string, expression literal, comment and brace, other
// than at offset 0.
func ElementStarts(text string) []int {
	var starts []int
	depth := 0
	col1 := true
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case c == '\n':
			col1 = true
			i++
			continue
		case strings.HasPrefix(text[i:], "//"):
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return starts
			}
			i += 2 + end + 2
			col1 = false
			continue
		case strings.HasPrefix(text[i:], `"""`):
			end := strings.Index(text[i+3:], `"""`)
			if end < 0 {
				return starts
			}
			i += 3 + end + 3
			col1 = false
			continue
		case c == '`':
			end := strings.IndexByte(text[i+1:], '`')
			if end < 0 {
				return starts
			}
			i += 1 + end + 1
			col1 = false
			continue
		case c == '"':
			j := i + 1
			for j < len(text) && text[j] != c && text[j] != '\n' {
				if text[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(text) && text[j] == c {
				j++
			}
			i = j
			col1 = false
			continue
		case c == '{':
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		default:
			if col1 && depth == 0 && i > 0 && identStart(text[i:]) {
				starts = append(starts, i)
			}
		}
		col1 = false
		i++
	}
	return starts
}

func isWordStart(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return isWordRune(r)
}

func identStart(text string) bool {
	if text == "" || text[0] < '0' || text[0] > '9' {
		return isWordStart(text)
	}
	digits := func(i int) int {
		for i < len(text) && '0' <= text[i] && text[i] <= '9' {
			i++
		}
		return i
	}
	i := digits(0)
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		j := i + 1
		if j < len(text) && (text[j] == '+' || text[j] == '-') {
			j++
		}
		if j < len(text) && '0' <= text[j] && text[j] <= '9' {
			i = digits(j)
		}
	}
	return i < len(text) && text[i] != '.' && isWordStart(text[i:])
}

func isWordRune(r rune) bool {
	return r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r >= 0x80
}

type context struct {
	lexical    bool
	contiguous bool
	top        bool
	line       bool
	depth      int
}

func (c context) free() bool { return !c.line || c.depth > 0 }

type memoKey struct {
	name       string
	pos        int
	contiguous bool
	line       bool
	depth      int
}

type matcher struct {
	l      *Language
	text   string
	memo   map[memoKey][]int
	active map[memoKey]bool
}

func (m *matcher) rule(name string, pos int, ctx context) []int {
	r := m.l.G.Lookup(name)
	if r == nil {
		return nil
	}
	inner := context{lexical: ctx.lexical || m.l.Lexical[name], contiguous: ctx.contiguous || m.l.Contiguous[name], line: ctx.line, depth: ctx.depth}
	key := memoKey{name, pos, inner.contiguous, inner.line, inner.depth}
	if ends, ok := m.memo[key]; ok {
		return ends
	}
	if m.active[key] {
		return nil
	}
	m.active[key] = true
	ends := m.expr(r.Expr, pos, inner)
	delete(m.active, key)
	m.memo[key] = ends
	return ends
}

func (m *matcher) layout(pos int, newlines bool) int {
	for {
		start := pos
		if m.l.Layout != nil {
			for _, e := range m.expr(m.l.Layout, pos, context{lexical: true}) {
				if e > pos {
					pos = e
				}
			}
		}
		for newlines && pos < len(m.text) && (m.text[pos] == '\n' || m.text[pos] == '\r') {
			pos++
		}
		if pos == start {
			return pos
		}
	}
}

func (m *matcher) expr(e Expr, pos int, ctx context) []int {
	switch x := e.(type) {
	case Seq:
		if !ctx.lexical && !ctx.line && m.l.lineOriented(x) {
			ctx.line, ctx.depth = true, 0
		}
		ends := []int{pos}
		for _, it := range x.Items {
			var next []int
			for _, p := range ends {
				next = union(next, m.expr(it, p, ctx))
			}
			if len(next) == 0 {
				return nil
			}
			ends = next
			if ctx.line {
				ctx.depth += m.l.delta(it)
			}
		}
		return ends
	case Alt:
		var ends []int
		for _, it := range x.Items {
			ends = union(ends, m.expr(it, pos, ctx))
		}
		return ends
	case Opt:
		return union([]int{pos}, m.expr(x.X, pos, ctx))
	case Rep:
		ends := []int{pos}
		frontier := []int{pos}
		for len(frontier) > 0 {
			var next []int
			for _, p := range frontier {
				for _, q := range m.expr(x.X, p, ctx) {
					if q > p && !contains(ends, q) && !contains(next, q) {
						next = append(next, q)
					}
				}
			}
			ends = union(ends, next)
			frontier = next
		}
		return ends
	case Times:
		ends := []int{pos}
		for i := 0; i < x.N; i++ {
			var next []int
			for _, p := range ends {
				next = union(next, m.expr(x.X, p, ctx))
			}
			ends = next
		}
		return ends
	case Except:
		var ends []int
		ys := m.expr(x.Y, pos, ctx)
		for _, e := range m.expr(x.X, pos, ctx) {
			if !contains(ys, e) {
				ends = append(ends, e)
			}
		}
		return ends
	case Term:
		p := m.before(pos, ctx, false)
		if !strings.HasPrefix(m.text[p:], x.Text) {
			return nil
		}
		end := p + len(x.Text)
		if !ctx.lexical && !m.boundary(p, end) {
			return nil
		}
		if !ctx.lexical && m.l.NoBreakAfter[x.Text] {
			q := end
			for q < len(m.text) && m.text[q] == '\r' {
				q++
			}
			if q < len(m.text) && m.text[q] == '\n' {
				return nil
			}
		}
		return []int{end}
	case Special:
		p := m.before(pos, ctx, false)
		if p >= len(m.text) {
			return nil
		}
		r, n := utf8.DecodeRuneInString(m.text[p:])
		if f := m.l.Specials[x.Text]; f != nil && f(r) {
			return []int{p + n}
		}
		return nil
	case Ref:
		if !ctx.lexical && x.Name == m.l.Newline {
			p := m.before(pos, ctx, true)
			if p == len(m.text) || m.text[p] == '}' {
				return []int{p}
			}
			return m.rule(x.Name, p, ctx)
		}
		if m.l.SameLine[x.Name] && !ctx.lexical && !ctx.contiguous && m.layout(pos, false) != m.layout(pos, true) {
			return nil
		}
		p := m.before(pos, ctx, false)
		ends := m.rule(x.Name, p, ctx)
		if !ctx.lexical && m.l.Lexical[x.Name] {
			var kept []int
			for _, e := range ends {
				if m.boundary(p, e) {
					kept = append(kept, e)
				}
			}
			if len(kept) == 0 {
				return nil
			}
			return kept[len(kept)-1:]
		}
		return ends
	default:
		panic(fmt.Sprintf("ebnf: unhandled expression %T", e))
	}
}

func (m *matcher) before(pos int, ctx context, sameLine bool) int {
	if ctx.lexical || ctx.contiguous {
		return pos
	}
	return m.layout(pos, !sameLine && ctx.free())
}

func (l *Language) lineOriented(s Seq) bool {
	if l.Newline == "" || len(s.Items) == 0 {
		return false
	}
	last, ok := s.Items[len(s.Items)-1].(Ref)
	return ok && last.Name == l.Newline
}

func (l *Language) delta(e Expr) int {
	switch x := e.(type) {
	case Term:
		switch x.Text {
		case "(", "[", "{":
			return 1
		case ")", "]", "}":
			return -1
		}
	case Seq:
		d := 0
		for _, it := range x.Items {
			d += l.delta(it)
		}
		return d
	}
	return 0
}

func (m *matcher) boundary(from, to int) bool {
	if to > from {
		last, _ := utf8.DecodeLastRuneInString(m.text[from:to])
		if isWordRune(last) && to < len(m.text) {
			if next, _ := utf8.DecodeRuneInString(m.text[to:]); isWordRune(next) {
				return false
			}
		}
	}
	tok := m.text[from:to]
	for _, t := range m.l.terminals() {
		if len(t) > len(tok) && strings.HasPrefix(t, tok) && strings.HasPrefix(m.text[from:], t) {
			return false
		}
	}
	return true
}

func (l *Language) terminals() []string {
	if l.puncts == nil {
		seen := map[string]bool{}
		l.puncts = []string{}
		for _, r := range l.G.Rules {
			walk(r.Expr, func(e Expr) {
				if t, ok := e.(Term); ok && t.Text != "" && !seen[t.Text] && !strings.ContainsFunc(t.Text, isWordRune) {
					seen[t.Text] = true
					l.puncts = append(l.puncts, t.Text)
				}
			})
		}
	}
	return l.puncts
}

func union(a, b []int) []int {
	for _, x := range b {
		if !contains(a, x) {
			a = append(a, x)
		}
	}
	sort.Ints(a)
	return a
}

func contains(a []int, x int) bool {
	for _, y := range a {
		if y == x {
			return true
		}
	}
	return false
}
