package golang

import (
	"strings"

	"github.com/Piechutowski/volt/gen/layout"
)

var sqlClauses = []string{
	"SELECT", "FROM", "LEFT JOIN", "INNER JOIN", "CROSS JOIN", "JOIN", "WHERE", "GROUP BY",
	"HAVING", "ORDER BY", "LIMIT", "OFFSET", "INSERT INTO", "VALUES", "UPDATE", "SET",
	"DELETE FROM", "RETURNING", "ON CONFLICT", "UNION ALL", "UNION",
}

const (
	sqlDepthMax = 32
	sqlStepsMax = 1 << 16
)

const (
	sqlStatement = iota
	sqlClause
	sqlBody
	sqlLine
)

type sqlWork struct {
	kind   int
	text   string
	indent string
	depth  int
}

func sqlConst(b *strings.Builder, name, sql string) {
	b.WriteString("const " + name + " = `")
	for _, clause := range sqlClausesSplit(sql) {
		if layout.Fits(clause) {
			b.WriteByte('\n')
			b.WriteString(clause)
			continue
		}
		if sqlClauseColumnsWrite(b, clause) {
			continue
		}
		sqlWorkWrite(b, sqlWork{kind: sqlClause, text: clause})
	}
	b.WriteString("`\n\n")
}

type sqlFragmentElem string

func sqlFragment(fragment string) layout.Elem { return sqlFragmentElem(fragment) }

func (f sqlFragmentElem) flat(at layout.At) bool {
	return at.OneLine || at.Fits(layout.Shape{First: layout.Width(string(f)) + 2})
}

func (f sqlFragmentElem) Shape(at layout.At) layout.Shape {
	if f.flat(at) {
		return layout.Shape{First: layout.Width(string(f)) + 2}
	}
	last := 0
	sqlWorkEach(sqlWork{kind: sqlBody, text: string(f), indent: "\t"}, func(indent, text string) {
		last = layout.Width(indent) + layout.Width(text)
	})
	return layout.Shape{Multi: true, First: 1, Last: last + 1}
}

func (f sqlFragmentElem) Write(b *strings.Builder, at layout.At) {
	b.WriteByte('`')
	if f.flat(at) {
		b.WriteString(string(f))
	} else {
		sqlWorkWrite(b, sqlWork{kind: sqlBody, text: string(f), indent: "\t"})
	}
	b.WriteByte('`')
}

func sqlWorkWrite(b *strings.Builder, first sqlWork) {
	sqlWorkEach(first, func(indent, text string) {
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteString(text)
	})
}

func sqlWorkEach(first sqlWork, line func(indent, text string)) {
	stack := []sqlWork{first}
	for step := 0; step < sqlStepsMax && len(stack) > 0; step++ {
		w := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if w.kind == sqlLine {
			line(w.indent, w.text)
			continue
		}
		next := sqlWorkSplit(w)
		for i := len(next) - 1; i >= 0; i-- {
			stack = append(stack, next[i])
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		line(stack[i].indent, stack[i].text)
	}
}

func sqlWorkSplit(w sqlWork) []sqlWork {
	at := func(kind int, text, indent string) sqlWork {
		return sqlWork{kind: kind, text: text, indent: indent, depth: w.depth + 1}
	}
	switch w.kind {
	case sqlStatement:
		clauses := sqlClausesSplit(w.text)
		out := make([]sqlWork, len(clauses))
		for i, c := range clauses {
			out[i] = at(sqlClause, c, w.indent)
		}
		return out
	case sqlClause:
		if layout.Fits(w.indent + w.text) {
			return []sqlWork{at(sqlLine, w.text, w.indent)}
		}
		for _, kw := range sqlClauses {
			if body, ok := strings.CutPrefix(w.text, kw+" "); ok {
				return []sqlWork{at(sqlLine, kw, w.indent), at(sqlBody, body, w.indent+"\t")}
			}
		}
		return []sqlWork{at(sqlBody, w.text, w.indent)}
	}
	return sqlBodySplit(w, at)
}

func sqlBodySplit(w sqlWork, at func(int, string, string) sqlWork) []sqlWork {
	if layout.Fits(w.indent+w.text) || w.depth >= sqlDepthMax {
		return []sqlWork{at(sqlLine, w.text, w.indent)}
	}
	parts := sqlCommaSplit(w.text)
	if len(parts) == 1 {
		parts = sqlLogicSplit(w.text)
	}
	if len(parts) > 1 {
		out := make([]sqlWork, len(parts))
		for i, p := range parts {
			out[i] = at(sqlBody, p, w.indent)
		}
		return out
	}
	prefix, inner, ok := sqlParenSplit(w.text)
	if !ok {
		return []sqlWork{at(sqlLine, w.text, w.indent)}
	}
	kind := sqlBody
	if strings.HasPrefix(inner, "SELECT ") {
		kind = sqlStatement
	}
	return []sqlWork{
		at(sqlLine, prefix+"(", w.indent),
		at(kind, inner, w.indent+"\t"),
		at(sqlLine, ")", w.indent),
	}
}

func sqlClausesSplit(sql string) []string {
	var out []string
	start := 0
	sqlTopScan(sql, func(i int) {
		if i == 0 || sql[i-1] != ' ' {
			return
		}
		for _, kw := range sqlClauses {
			if sqlWordAt(sql, i, kw) {
				out = append(out, sql[start:i-1])
				start = i
				return
			}
		}
	})
	return append(out, sql[start:])
}

func sqlCommaSplit(body string) []string {
	var out []string
	start := 0
	sqlTopScan(body, func(i int) {
		if strings.HasPrefix(body[i:], ", ") {
			out = append(out, body[start:i+1])
			start = i + 2
		}
	})
	return append(out, body[start:])
}

func sqlLogicSplit(body string) []string {
	var out []string
	start := 0
	sqlTopScan(body, func(i int) {
		if i == 0 || body[i-1] != ' ' {
			return
		}
		if sqlWordAt(body, i, "AND") || sqlWordAt(body, i, "OR") {
			out = append(out, body[start:i-1])
			start = i
		}
	})
	return append(out, body[start:])
}

func sqlParenSplit(body string) (prefix, inner string, ok bool) {
	if !strings.HasSuffix(body, ")") {
		return "", "", false
	}
	open, opens := -1, 0
	sqlTopScan(body, func(i int) {
		if body[i] == '(' {
			opens++
			open = i
		}
	})
	if opens != 1 {
		return "", "", false
	}
	return body[:open], body[open+1 : len(body)-1], true
}

func sqlWordAt(s string, i int, word string) bool {
	if !strings.HasPrefix(s[i:], word) {
		return false
	}
	end := i + len(word)
	return end == len(s) || s[end] == ' '
}

func sqlTopScan(s string, at func(i int)) {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			if depth == 0 {
				at(i)
			}
			depth++
		case c == ')':
			depth--
		case depth == 0:
			at(i)
		}
	}
}

func sqlClauseColumnsWrite(b *strings.Builder, clause string) bool {
	for _, kw := range sqlClauses {
		body, ok := strings.CutPrefix(clause, kw+" ")
		if !ok {
			continue
		}
		parts := sqlCommaSplit(body)
		if len(parts) < 2 {
			return false
		}
		for _, p := range parts {
			if !layout.Fits("\t" + p) {
				return false
			}
		}
		b.WriteByte('\n')
		b.WriteString(kw)
		for _, p := range parts {
			b.WriteString("\n\t")
			b.WriteString(p)
		}
		return true
	}
	return false
}
