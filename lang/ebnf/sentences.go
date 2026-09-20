package ebnf

import (
	"fmt"
	"sort"
	"strings"
)

// Piece is one token of a rendered sentence, with what the join
// between it and its predecessor needs: whether it is glued to the
// previous token (contiguous rules), whether it is layout itself,
// whether it is a required line break, or the break between two
// elements, or the hole of a context.
type Piece struct {
	Text   string
	Glued  bool
	Layout bool
	NL     bool
	Sep    bool
	Hole   bool
}

// Text joins pieces into the sentence's text: one space between two
// tokens unless the second is glued, a tab after a required line
// break so no token lands in the first column (§3.2 rule 5).
func Text(pieces []Piece) string {
	var b strings.Builder
	for i, p := range pieces {
		if i > 0 {
			prev := pieces[i-1]
			switch {
			case prev.NL && !p.Sep:
				b.WriteString("\t")
			case p.Glued || p.Layout || p.NL || p.Sep || prev.Layout || prev.Sep:
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString(p.Text)
	}
	return b.String()
}

// Sentence is a rendered sentence of the start rule that exercises
// one choice of one rule.
type Sentence struct {
	Rule   string  // the rule exercised
	Pieces []Piece // the whole sentence, tokens of the start rule
}

// Witness must map every special sequence of the grammar to one
// character its matcher accepts; Check verifies it.
func (l *Language) witnessCheck() error {
	var problems []string
	for _, r := range l.G.Rules {
		walk(r.Expr, func(e Expr) {
			s, ok := e.(Special)
			if !ok {
				return
			}
			w, has := l.Witness[s.Text]
			switch {
			case !has:
				problems = append(problems, fmt.Sprintf("%s (line %d): no witness for the special sequence ? %s ?", r.Name, r.Line, s.Text))
			case len([]rune(w)) != 1 || l.Specials[s.Text] == nil || !l.Specials[s.Text]([]rune(w)[0]):
				problems = append(problems, fmt.Sprintf("%s (line %d): the witness %q of ? %s ? is not one character its matcher accepts", r.Name, r.Line, w, s.Text))
			}
		})
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

// Sentences derives, for every rule reachable from the start rule,
// the sentences that exercise each choice made inside that rule once
// (each alternative, each option taken and left, each repetition
// zero, one and two times), every other choice minimal, each embedded
// in the shortest sentence of the start rule that reaches the rule.
// An unreachable rule yields nothing and is reported.
func (l *Language) Sentences() (sentences []Sentence, unreachable []string, err error) {
	if err := l.Check(); err != nil {
		return nil, nil, err
	}
	if err := l.witnessCheck(); err != nil {
		return nil, nil, err
	}
	d := &deriver{l: l, mins: map[string]minEntry{}}
	start := l.start()
	// contexts by breadth-first search from the start rule
	type ctxEntry struct {
		pieces []Piece
		ctx    context
	}
	// A context through the left side of an exception could be
	// filled with a sentence the exception excludes, so contexts
	// avoid exceptions while any path does, then take them.
	contexts := map[string]ctxEntry{start: {[]Piece{{Hole: true}}, context{top: true}}}
	for _, throughExcept := range []bool{false, true} {
		queue := []string{}
		for name := range contexts {
			queue = append(queue, name)
		}
		sort.Strings(queue)
		for len(queue) > 0 {
			name := queue[0]
			queue = queue[1:]
			r := l.G.Lookup(name)
			c := contexts[name]
			inner := context{lexical: c.ctx.lexical || l.Lexical[name], contiguous: c.ctx.contiguous || l.Contiguous[name], top: name == start}
			for _, ref := range Refs(r.Expr) {
				if _, seen := contexts[ref]; seen {
					continue
				}
				h, hctx, ok := d.hole(r.Expr, inner, ref, throughExcept)
				if !ok {
					continue
				}
				contexts[ref] = ctxEntry{substitute(c.pieces, h), hctx}
				queue = append(queue, ref)
			}
		}
	}
	for _, r := range l.G.Rules {
		c, ok := contexts[r.Name]
		if !ok {
			unreachable = append(unreachable, r.Name)
			continue
		}
		inner := context{lexical: c.ctx.lexical || l.Lexical[r.Name], contiguous: c.ctx.contiguous || l.Contiguous[r.Name], top: r.Name == start}
		seen := map[string]bool{}
		for _, v := range d.variants(r.Expr, inner) {
			pieces := d.tokens(v, inner, c.ctx, r.Name)
			full := substitute(c.pieces, pieces)
			text := Text(full)
			if seen[text] {
				continue
			}
			seen[text] = true
			sentences = append(sentences, Sentence{Rule: r.Name, Pieces: full})
		}
	}
	return sentences, unreachable, nil
}

// substitute replaces the hole of a context with pieces, which take
// the hole's join flags.
func substitute(ctx []Piece, fill []Piece) []Piece {
	out := make([]Piece, 0, len(ctx)+len(fill))
	for _, p := range ctx {
		if !p.Hole {
			out = append(out, p)
			continue
		}
		for i, f := range fill {
			if i == 0 {
				f.Glued = p.Glued
			}
			out = append(out, f)
		}
	}
	return out
}

/* ===== derivation ===== */

type minEntry struct {
	pieces []Piece
	ok     bool
}

type deriver struct {
	l      *Language
	mins   map[string]minEntry
	active map[string]bool
}

// tokens turns a rendering made in the rule's own context into pieces
// for the enclosing context: a lexical rule seen from a syntactic
// context is one token; a contiguous rule's first token is not glued
// to what precedes it.
func (d *deriver) tokens(pieces []Piece, inner, outer context, name string) []Piece {
	if inner.lexical && !outer.lexical {
		text := Text(pieces)
		return []Piece{{Text: text, Glued: outer.contiguous, Layout: strings.TrimSpace(text) == ""}}
	}
	if inner.contiguous && !outer.contiguous && len(pieces) > 0 {
		out := append([]Piece{}, pieces...)
		out[0].Glued = false
		return out
	}
	return pieces
}

// ref renders the named rule in its own context and returns pieces
// for the referring context.
func (d *deriver) ref(name string, outer context, render func(Expr, context) ([]Piece, bool)) ([]Piece, bool) {
	inner := context{lexical: outer.lexical || d.l.Lexical[name], contiguous: outer.contiguous || d.l.Contiguous[name]}
	r := d.l.G.Lookup(name)
	if r == nil {
		return nil, false
	}
	if !outer.lexical && name == d.l.Newline {
		return []Piece{{Text: "\n", NL: true}}, true
	}
	pieces, ok := render(r.Expr, inner)
	if !ok {
		return nil, false
	}
	return d.tokens(pieces, inner, outer, name), true
}

// min is the shortest sentence of e, by text length; false when e
// derives nothing finite (a cycle with no way out).
func (d *deriver) min(e Expr, ctx context) ([]Piece, bool) {
	switch x := e.(type) {
	case Seq:
		var out []Piece
		for _, it := range x.Items {
			p, ok := d.min(it, ctx)
			if !ok {
				return nil, false
			}
			out = append(out, p...)
		}
		return out, true
	case Alt:
		var best []Piece
		found := false
		for _, it := range x.Items {
			p, ok := d.min(it, ctx)
			if ok && (!found || len(Text(p)) < len(Text(best))) {
				best, found = p, true
			}
		}
		return best, found
	case Opt, Rep:
		return []Piece{}, true
	case Times:
		var out []Piece
		for i := 0; i < x.N; i++ {
			p, ok := d.min(x.X, ctx)
			if !ok {
				return nil, false
			}
			out = append(out, p...)
		}
		return out, true
	case Except:
		for _, v := range d.variants(x.X, ctx) {
			if !d.matches(x.Y, Text(v), ctx) {
				return v, true
			}
		}
		return nil, false
	case Term:
		return []Piece{{Text: x.Text, Glued: ctx.contiguous || ctx.lexical}}, true
	case Special:
		return []Piece{{Text: d.l.Witness[x.Text], Glued: ctx.contiguous || ctx.lexical}}, true
	case Ref:
		key := x.Name
		if m, ok := d.mins[key]; ok && ctx.lexical == d.l.Lexical[key] {
			if !m.ok {
				return nil, false
			}
			return d.tokens(m.pieces, context{lexical: d.l.Lexical[key], contiguous: ctx.contiguous || d.l.Contiguous[key]}, ctx, key), true
		}
		if d.active == nil {
			d.active = map[string]bool{}
		}
		if d.active[key] {
			return nil, false
		}
		d.active[key] = true
		p, ok := d.ref(x.Name, ctx, d.min)
		delete(d.active, key)
		return p, ok
	}
	panic(fmt.Sprintf("ebnf: unhandled expression %T", e))
}

// variants are the sentences of e that exercise each choice inside
// e once, sub-rules rendered minimal; the first is the minimum.
func (d *deriver) variants(e Expr, ctx context) [][]Piece {
	m, ok := d.min(e, ctx)
	if !ok {
		return nil
	}
	out := [][]Piece{m}
	add := func(p []Piece) {
		t := Text(p)
		for _, q := range out {
			if Text(q) == t {
				return
			}
		}
		out = append(out, p)
	}
	switch x := e.(type) {
	case Seq:
		mins := make([][]Piece, len(x.Items))
		for i, it := range x.Items {
			mins[i], _ = d.min(it, ctx)
		}
		for i, it := range x.Items {
			for _, v := range d.variants(it, ctx) {
				var p []Piece
				for j := range x.Items {
					if j == i {
						p = append(p, v...)
					} else {
						p = append(p, mins[j]...)
					}
				}
				add(p)
			}
		}
	case Alt:
		for _, it := range x.Items {
			for _, v := range d.variants(it, ctx) {
				add(v)
			}
		}
	case Opt:
		for _, v := range d.variants(x.X, ctx) {
			add(v)
		}
	case Rep:
		for _, v := range d.variants(x.X, ctx) {
			add(v)
		}
		if one, ok := d.min(x.X, ctx); ok && len(one) > 0 {
			two := append([]Piece{}, one...)
			if ctx.top {
				two = append(two, Piece{Text: "\n", Sep: true})
			}
			add(append(two, one...))
		}
	case Times:
		one, _ := d.min(x.X, ctx)
		for _, v := range d.variants(x.X, ctx) {
			var p []Piece
			for i := 0; i < x.N-1; i++ {
				p = append(p, one...)
			}
			add(append(p, v...))
		}
	case Except:
		out = nil
		for _, v := range d.variants(x.X, ctx) {
			if !d.matches(x.Y, Text(v), ctx) {
				out = append(out, v)
			}
		}
	}
	return out
}

// hole is the shortest sentence of e that passes through a reference
// to name, that reference rendered as a hole; the context the hole
// sits in comes back with it. Unless throughExcept, the left side of
// an exception is not entered.
func (d *deriver) hole(e Expr, ctx context, name string, throughExcept bool) ([]Piece, context, bool) {
	switch x := e.(type) {
	case Seq:
		for i, it := range x.Items {
			h, hctx, ok := d.hole(it, ctx, name, throughExcept)
			if !ok {
				continue
			}
			var out []Piece
			for j, other := range x.Items {
				if j == i {
					out = append(out, h...)
					continue
				}
				p, ok := d.min(other, ctx)
				if !ok {
					return nil, ctx, false
				}
				out = append(out, p...)
			}
			return out, hctx, true
		}
		return nil, ctx, false
	case Alt:
		var best []Piece
		var bestCtx context
		found := false
		for _, it := range x.Items {
			h, hctx, ok := d.hole(it, ctx, name, throughExcept)
			if ok && (!found || len(Text(h)) < len(Text(best))) {
				best, bestCtx, found = h, hctx, true
			}
		}
		return best, bestCtx, found
	case Opt:
		return d.hole(x.X, ctx, name, throughExcept)
	case Rep:
		return d.hole(x.X, ctx, name, throughExcept)
	case Times:
		h, hctx, ok := d.hole(x.X, ctx, name, throughExcept)
		if !ok {
			return nil, ctx, false
		}
		var out []Piece
		for i := 0; i < x.N-1; i++ {
			p, ok := d.min(x.X, ctx)
			if !ok {
				return nil, ctx, false
			}
			out = append(out, p...)
		}
		return append(out, h...), hctx, true
	case Except:
		if !throughExcept {
			return nil, ctx, false
		}
		return d.hole(x.X, ctx, name, throughExcept)
	case Ref:
		if x.Name != name {
			return nil, ctx, false
		}
		if !ctx.lexical && name == d.l.Newline {
			return nil, ctx, false // rendered as a line break, never a hole
		}
		return []Piece{{Hole: true, Glued: ctx.contiguous || ctx.lexical}}, ctx, true
	}
	return nil, ctx, false
}

// matches reports whether text is a sentence of e in ctx.
func (d *deriver) matches(e Expr, text string, ctx context) bool {
	m := &matcher{l: d.l, text: text, memo: map[memoKey][]int{}, active: map[memoKey]bool{}}
	for _, end := range m.expr(e, 0, ctx) {
		if end == len(text) {
			return true
		}
	}
	return false
}

// Deletions are the texts with one token removed, each once: the
// sentences one keystroke away.
func Deletions(pieces []Piece) []string {
	var out []string
	seen := map[string]bool{}
	for i := range pieces {
		if pieces[i].NL || pieces[i].Sep {
			continue
		}
		rest := append(append([]Piece{}, pieces[:i]...), pieces[i+1:]...)
		if i < len(rest) && i > 0 {
			rest[i].Glued = pieces[i].Glued
		}
		t := Text(rest)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// Insertions are the texts with one of the given tokens inserted at
// each position, each once.
func Insertions(pieces []Piece, tokens []Piece) []string {
	var out []string
	seen := map[string]bool{}
	for i := 0; i <= len(pieces); i++ {
		for _, tok := range tokens {
			with := append(append(append([]Piece{}, pieces[:i]...), tok), pieces[i:]...)
			t := Text(with)
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Swaps are the texts with two adjacent tokens exchanged, each once.
func Swaps(pieces []Piece) []string {
	var out []string
	seen := map[string]bool{}
	for i := 0; i+1 < len(pieces); i++ {
		with := append([]Piece{}, pieces...)
		with[i], with[i+1] = with[i+1], with[i]
		with[i].Glued, with[i+1].Glued = pieces[i].Glued, pieces[i+1].Glued
		t := Text(with)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
