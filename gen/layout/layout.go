package layout

import (
	"strings"
	"unicode/utf8"
)

const (
	LineColumnsMax = 100
	TabColumns     = 4
	specPassesMax  = 64
	tabsMax        = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"
)

func Width(line string) int {
	tabs := strings.Count(line, "\t")
	return len(line) - tabs + tabs*TabColumns
}

func Fits(line string) bool { return Width(line) <= LineColumnsMax }

func Tabs(indent int) string {
	if indent <= len(tabsMax) {
		return tabsMax[:indent]
	}
	return strings.Repeat("\t", indent)
}

type At struct {
	Indent  int
	Lead    int
	Trail   int
	OneLine bool
	Wrap    bool
	Deep    bool
}

type Shape struct {
	Multi       bool
	First, Last int
}

func (at At) Fits(s Shape) bool {
	lead := at.Indent*TabColumns + at.Lead
	if !s.Multi {
		return lead+s.First+at.Trail <= LineColumnsMax
	}
	return lead+s.First <= LineColumnsMax && s.Last+at.Trail <= LineColumnsMax
}

type Elem interface {
	Shape(at At) Shape
	Write(b *strings.Builder, at At)
}

func Render(e Elem, at At) string {
	var b strings.Builder
	e.Write(&b, at)
	return b.String()
}

func Line(indent int, before string, e Elem, after string) string {
	var b strings.Builder
	b.WriteString(before)
	e.Write(&b, At{Indent: indent, Lead: Width(before), Trail: Width(strings.TrimRight(after, "\n"))})
	b.WriteString(after)
	return b.String()
}

func WriteLine(b *strings.Builder, indent int, before string, e Elem, after string) {
	b.WriteString(Tabs(indent))
	b.WriteString(before)
	e.Write(b, At{Indent: indent, Lead: Width(before), Trail: Width(strings.TrimRight(after, "\n"))})
	b.WriteString(after)
}

type text string

func Text(s string) Elem { return text(s) }

func (t text) Shape(At) Shape {
	s := string(t)
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return Shape{First: Width(s)}
	}
	return Shape{Multi: true, First: Width(s[:i]), Last: Width(s[strings.LastIndexByte(s, '\n')+1:])}
}

func (t text) Write(b *strings.Builder, _ At) { b.WriteString(string(t)) }

func Texts(ss []string) []Elem {
	out := make([]Elem, len(ss))
	for i, s := range ss {
		out[i] = text(s)
	}
	return out
}

type prefixed struct {
	before string
	e      Elem
}

func Prefixed(before string, e Elem) Elem { return prefixed{before, e} }

func (p prefixed) Shape(at At) Shape {
	width := Width(p.before)
	at.Lead += width
	s := p.e.Shape(at)
	s.First += width
	return s
}

func (p prefixed) Write(b *strings.Builder, at At) {
	b.WriteString(p.before)
	at.Lead += Width(p.before)
	p.e.Write(b, at)
}

type list struct {
	head, tail string
	elems      []Elem
	strs       []string
	call       bool
	paren      bool
}

func List(head string, elems []Elem, tail string) Elem {
	return list{head: head, tail: tail, elems: elems}
}

func ListStrings(head string, strs []string, tail string) Elem {
	return list{head: head, tail: tail, strs: strs}
}

func Call(fn string, args []Elem) Elem {
	return list{head: fn, paren: true, tail: ")", elems: args, call: true}
}

func CallStrings(fn string, args []string) Elem {
	return list{head: fn, paren: true, tail: ")", strs: args, call: true}
}

func (l list) headWidth() int {
	n := Width(l.head)
	if l.paren {
		n++
	}
	return n
}

func (l list) count() int { return len(l.elems) + len(l.strs) }

func (l list) shapeOf(i int, at At) Shape {
	if l.strs != nil {
		return text(l.strs[i]).Shape(at)
	}
	return l.elems[i].Shape(at)
}

func (l list) writeOf(i int, b *strings.Builder, at At) {
	if l.strs != nil {
		b.WriteString(l.strs[i])
		return
	}
	l.elems[i].Write(b, at)
}

func (l list) callAt(i int) bool {
	if l.strs != nil {
		return false
	}
	return isCall(l.elems[i])
}

const unwrapDepthMax = 64

func isCall(e Elem) bool {
	for depth := 0; depth < unwrapDepthMax; depth++ {
		switch inner := e.(type) {
		case list:
			return inner.call
		case prefixed:
			e = inner.e
		case fields:
			if len(inner.keys) != 1 {
				return false
			}
			e = inner.values[0]
		default:
			return false
		}
	}
	return false
}

func (l list) deep(at At) bool { return l.call && (at.Deep || l.count() > 1) }

func (l list) elemAt(at At, i, width int) At {
	elemAt := At{Indent: at.Indent, OneLine: true, Deep: l.deep(at)}
	if i == l.count()-1 {
		elemAt.Lead, elemAt.Trail = at.Lead+width, Width(l.tail)+at.Trail
	}
	return elemAt
}

func (l list) flatShape(at At) (Shape, bool) {
	width := l.headWidth()
	n := l.count()
	for i := range n {
		if i > 0 {
			width += 2
		}
		s := l.shapeOf(i, l.elemAt(at, i, width))
		if s.Multi {
			if i < n-1 || l.callAt(i) {
				return Shape{}, false
			}
			return Shape{Multi: true, First: width + s.First, Last: s.Last + Width(l.tail)}, true
		}
		width += s.First
	}
	return Shape{First: width + Width(l.tail)}, true
}

func (l list) flat(at At) bool {
	if l.count() == 0 {
		return true
	}
	s, ok := l.flatShape(at)
	return ok && !at.Wrap && (at.OneLine || at.Fits(s))
}

func (l list) Shape(at At) Shape {
	if l.flat(at) {
		s, _ := l.flatShape(at)
		return s
	}
	return Shape{Multi: true, First: l.headWidth(), Last: at.Indent*TabColumns + Width(l.tail)}
}

func (l list) Write(b *strings.Builder, at At) {
	b.WriteString(l.head)
	if l.paren {
		b.WriteByte('(')
	}
	n := l.count()
	if l.flat(at) {
		width := l.headWidth()
		for i := range n {
			if i > 0 {
				b.WriteString(", ")
				width += 2
			}
			elemAt := l.elemAt(at, i, width)
			if i < n-1 {
				width += l.shapeOf(i, elemAt).First
			}
			l.writeOf(i, b, elemAt)
		}
		b.WriteString(l.tail)
		return
	}
	b.WriteByte('\n')
	for i := range n {
		b.WriteString(Tabs(at.Indent + 1))
		l.writeOf(i, b, At{Indent: at.Indent + 1, Trail: 1, Deep: l.deep(at)})
		b.WriteString(",\n")
	}
	b.WriteString(Tabs(at.Indent))
	b.WriteString(l.tail)
}

type signature struct {
	params       list
	results      []string
	tail         string
	resultsWidth int
}

func Signature(head string, params, results []string, tail string) Elem {
	return signatureOf(head, params, results, tail)
}

func signatureOf(head string, params, results []string, tail string) signature {
	s := signature{params: list{head: head, paren: true, tail: ")", strs: params}, results: results, tail: tail}
	s.resultsWidth = Width(tail)
	for i, r := range results {
		s.resultsWidth += Width(r) + 2
		if i == 0 && len(results) == 1 {
			s.resultsWidth--
		}
	}
	if len(results) > 1 {
		s.resultsWidth++
	}
	return s
}

func (s signature) resultsList() list {
	return list{head: " (", tail: ")" + s.tail, strs: s.results}
}

func (s signature) resultsWrite(b *strings.Builder) {
	switch len(s.results) {
	case 0:
	case 1:
		b.WriteByte(' ')
		b.WriteString(s.results[0])
	default:
		b.WriteString(" (")
		for i, r := range s.results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(r)
		}
		b.WriteByte(')')
	}
	b.WriteString(s.tail)
}

func (s signature) paramsAt(at At) At {
	at.Trail += s.resultsWidth
	return at
}

func (s signature) flat(at At) bool { return s.params.flat(s.paramsAt(at)) }

func (s signature) resultsWrapped(at At) bool {
	return len(s.results) > 1 && at.Indent*TabColumns+1+s.resultsWidth > LineColumnsMax
}

func (s signature) Shape(at At) Shape {
	if s.flat(at) {
		first := s.params.Shape(s.paramsAt(at)).First
		return Shape{First: first + s.resultsWidth}
	}
	last := at.Indent*TabColumns + 1 + s.resultsWidth
	if s.resultsWrapped(at) {
		last = at.Indent*TabColumns + 1 + Width(s.tail)
	}
	return Shape{Multi: true, First: s.params.headWidth(), Last: last}
}

func (s signature) Write(b *strings.Builder, at At) {
	if s.flat(at) {
		s.params.Write(b, s.paramsAt(at))
		s.resultsWrite(b)
		return
	}
	s.params.Write(b, At{Indent: at.Indent, Wrap: true})
	if !s.resultsWrapped(at) {
		s.resultsWrite(b)
		return
	}
	s.resultsList().Write(b, At{Indent: at.Indent, Wrap: true})
}

type fields struct {
	head, tail string
	keys       []string
	values     []Elem
}

func Fields(head string, keys []string, values []Elem, tail string) Elem {
	return fields{head, tail, keys, values}
}

func (f fields) single() list {
	return list{head: f.head, tail: f.tail, elems: []Elem{Prefixed(f.keys[0]+": ", f.values[0])}}
}

func (f fields) flat(at At) bool { return len(f.keys) == 1 && !at.Wrap && f.single().flat(at) }

func (f fields) Shape(at At) Shape {
	if f.flat(at) {
		return f.single().Shape(at)
	}
	return Shape{Multi: true, First: Width(f.head), Last: at.Indent*TabColumns + Width(f.tail)}
}

func (f fields) Write(b *strings.Builder, at At) {
	if f.flat(at) {
		f.single().Write(b, at)
		return
	}
	keyMax := 0
	for _, key := range f.keys {
		keyMax = max(keyMax, utf8.RuneCountInString(key))
	}
	valueAt := At{Indent: at.Indent + 1, Lead: keyMax + 2, Trail: 1}
	widths := fieldWidths(f.keys, f.values, valueAt)
	b.WriteString(f.head)
	b.WriteByte('\n')
	for i, key := range f.keys {
		b.WriteString(Tabs(at.Indent + 1))
		b.WriteString(key)
		b.WriteByte(':')
		for range widths[i] - utf8.RuneCountInString(key) {
			b.WriteByte(' ')
		}
		f.values[i].Write(b, valueAt)
		b.WriteString(",\n")
	}
	b.WriteString(Tabs(at.Indent))
	b.WriteString(f.tail)
}

func fieldWidths(keys []string, values []Elem, valueAt At) []int {
	widths := make([]int, len(keys))
	start, width := 0, 0
	for i, key := range keys {
		single := !values[i].Shape(valueAt).Multi
		end := i
		if single {
			width, end = max(width, utf8.RuneCountInString(key)+1), i+1
		}
		if single && i < len(keys)-1 {
			continue
		}
		for k := start; k < end; k++ {
			widths[k] = width
		}
		if !single {
			widths[i] = utf8.RuneCountInString(key) + 1
		}
		start, width = i+1, 0
	}
	return widths
}

type funcLit struct {
	sig  signature
	body Elem
}

func Func(params []string, results string, body Elem) Elem {
	var rs []string
	if results != "" {
		rs = []string{results}
	}
	return funcLit{sig: signatureOf("func", params, rs, ""), body: body}
}

func (f funcLit) oneLine(at At) (int, bool) {
	if at.Wrap {
		return 0, false
	}
	sig := f.sig.Shape(At{OneLine: true})
	s := f.body.Shape(At{OneLine: true})
	width := sig.First + len(" {  }") + s.First
	return width, !s.Multi && at.Fits(Shape{First: width})
}

func (f funcLit) sigAt(at At) At { return At{Indent: at.Indent, Lead: at.Lead, Trail: len(" {")} }

func (f funcLit) Shape(at At) Shape {
	if width, ok := f.oneLine(at); ok {
		return Shape{First: width}
	}
	sig := f.sig.Shape(At{OneLine: true})
	return Shape{Multi: true, First: sig.First + len(" {"), Last: at.Indent*TabColumns + 1}
}

func (f funcLit) Write(b *strings.Builder, at At) {
	if _, ok := f.oneLine(at); ok {
		f.sig.Write(b, At{OneLine: true})
		b.WriteString(" { ")
		f.body.Write(b, At{OneLine: true})
		b.WriteString(" }")
		return
	}
	f.sig.Write(b, f.sigAt(at))
	b.WriteString(" {\n")
	b.WriteString(Tabs(at.Indent + 1))
	f.body.Write(b, At{Indent: at.Indent + 1})
	b.WriteByte('\n')
	b.WriteString(Tabs(at.Indent))
	b.WriteByte('}')
}

type concat []string

func Concat(parts []string) Elem { return concat(parts) }

func (c concat) sep(at At) string {
	if at.Deep {
		return "+"
	}
	return " + "
}

func (c concat) flatWidth(at At) int {
	width := 0
	for i, p := range c {
		if i > 0 {
			width += len(c.sep(at))
		}
		width += Width(p)
	}
	return width
}

func (c concat) flat(at At) bool {
	if len(c) < 2 {
		return true
	}
	return !at.Wrap && (at.OneLine || at.Fits(Shape{First: c.flatWidth(at)}))
}

func (c concat) Shape(at At) Shape {
	if c.flat(at) {
		return Shape{First: c.flatWidth(at)}
	}
	sep := strings.TrimRight(c.sep(at), " ")
	return Shape{Multi: true, First: Width(c[0]) + len(sep), Last: (at.Indent+1)*TabColumns + Width(c[len(c)-1])}
}

func (c concat) Write(b *strings.Builder, at At) {
	if c.flat(at) {
		for i, p := range c {
			if i > 0 {
				b.WriteString(c.sep(at))
			}
			b.WriteString(p)
		}
		return
	}
	sep := strings.TrimRight(c.sep(at), " ")
	for i, p := range c {
		if i > 0 {
			b.WriteString(sep)
			b.WriteByte('\n')
			b.WriteString(Tabs(at.Indent + 1))
		}
		b.WriteString(p)
	}
}

type Spec struct {
	Name  string
	Value Elem
}

func SpecsWrite(b *strings.Builder, indent int, specs []Spec) {
	multi := make([]bool, len(specs))
	widths := specWidths(specs, multi)
	for pass := 0; pass < specPassesMax; pass++ {
		changed := false
		for i, s := range specs {
			if !multi[i] && s.Value.Shape(At{Indent: indent, Lead: widths[i] + 3}).Multi {
				multi[i], changed = true, true
			}
		}
		if !changed {
			break
		}
		widths = specWidths(specs, multi)
	}
	for i, s := range specs {
		b.WriteString(Tabs(indent))
		b.WriteString(s.Name)
		for range widths[i] - len(s.Name) {
			b.WriteByte(' ')
		}
		b.WriteString(" = ")
		s.Value.Write(b, At{Indent: indent, Lead: widths[i] + 3, OneLine: !multi[i], Wrap: multi[i]})
		b.WriteByte('\n')
	}
}

func specWidths(specs []Spec, multi []bool) []int {
	widths := make([]int, len(specs))
	start, width := 0, 0
	for i, s := range specs {
		width = max(width, len(s.Name))
		if !multi[i] && i < len(specs)-1 {
			continue
		}
		for k := start; k <= i; k++ {
			widths[k] = width
		}
		start, width = i+1, 0
	}
	return widths
}

func Results(results string) []string {
	inner, tuple := strings.CutPrefix(results, "(")
	if !tuple {
		if results == "" {
			return nil
		}
		return []string{results}
	}
	inner = strings.TrimSuffix(inner, ")")
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(inner[start:]))
}
