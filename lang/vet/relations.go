// Relationship and modeling analyzers: legal schemas that are likely to
// misbehave as databases — missing primary keys, foreign keys onto
// non-unique columns, type drift across a relationship, dependency cycles.
package vet

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

func init() {
	register(missingPK)
	register(refTypeMismatch)
	register(refTargetNotUnique)
	register(selfRef)
	register(refCycle)
	register(caseCollision)
}

type missingPKRule struct{ meta }

var missingPK = &missingPKRule{meta{
	name: "missingpk",
	doc:  "reports tables with no primary key (column setting or pk index)",
}}

func (r *missingPKRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	if ti := f.Table; ti != nil && !tableHasPK(ti) {
		out = append(out, r.warnf(ti.Decl.Pos(), "table %q has no primary key", ti.Decl.Name.String()))
	}
	return out
}

func tableHasPK(ti *check.TableInfo) bool {
	for _, cd := range ti.Columns {
		if columnHas(cd, "pk") {
			return true
		}
	}
	for _, ix := range ti.Indexes {
		if ix.Settings.Get("pk") != nil {
			return true
		}
	}
	return false
}

// fkSides returns the foreign-key endpoint and the referenced endpoint of a
// relationship, following spec §6.7.3, or ok=false for many-to-many.
func fkSides(r *check.RefInfo) (fk, target check.EndpointInfo, ok bool) {
	switch r.Op {
	case token.GT: // many-to-one: left is the FK
		return r.Left, r.Right, true
	case token.LT: // one-to-many: right is the FK
		return r.Right, r.Left, true
	case token.MINUS: // one-to-one: the second endpoint is the FK
		return r.Right, r.Left, true
	default: // <> has no FK side
		return fk, target, false
	}
}

type refTypeRule struct{ meta }

var refTypeMismatch = &refTypeRule{meta{
	name: "reftype",
	doc:  "reports relationships whose two columns have different declared types",
}}

func (r *refTypeRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		if ref.Left.Table == nil || ref.Right.Table == nil {
			continue
		}
		n := min(len(ref.Left.Columns), len(ref.Right.Columns))
		for i := 0; i < n; i++ {
			lc := ref.Left.Table.Column(ref.Left.Columns[i])
			rc := ref.Right.Table.Column(ref.Right.Columns[i])
			if lc == nil || rc == nil {
				continue
			}
			lt, rt := lc.Col.Type.String(), rc.Col.Type.String()
			if !strings.EqualFold(lt, rt) {
				out = append(out, r.warnf(ref.Node.Pos(), "relationship joins %s.%s (%s) to %s.%s (%s); the types differ",
					ref.Left.Table.Decl.Name.String(), ref.Left.Columns[i], lt,
					ref.Right.Table.Decl.Name.String(), ref.Right.Columns[i], rt))
			}
		}
	}
	return out
}

type refTargetRule struct{ meta }

var refTargetNotUnique = &refTargetRule{meta{
	name: "reftarget",
	doc:  "reports foreign keys referencing columns that are not pk or unique on the one side",
}}

func (r *refTargetRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		_, target, ok := fkSides(ref)
		if !ok || target.Table == nil {
			continue
		}
		if endpointUnique(target) {
			continue
		}
		out = append(out, r.warnf(ref.Node.Pos(), "referenced column(s) %s.(%s) are not pk or unique; the database cannot enforce this foreign key",
			target.Table.Decl.Name.String(), strings.Join(target.Columns, ", ")))
	}
	return out
}

// endpointUnique reports whether the endpoint's column set is guaranteed
// unique: a single pk/unique column, or a matching composite pk/unique index.
func endpointUnique(e check.EndpointInfo) bool {
	if len(e.Columns) == 1 {
		if cd := e.Table.Column(e.Columns[0]); cd != nil {
			if columnHas(cd, "pk") || columnHas(cd, "unique") {
				return true
			}
		}
	}
	for _, ix := range e.Table.Indexes {
		if ix.Settings.Get("pk") == nil && ix.Settings.Get("unique") == nil {
			continue
		}
		var got []string
		for _, a := range ix.Key {
			if id, ok := a.(*ast.Ident); ok {
				got = append(got, id.Name())
			}
		}
		if sameColumns(got, e.Columns) {
			return true
		}
	}
	return false
}

// sameColumns reports whether two column lists name the same columns
// the same number of times, in any order.
func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if count(a, x) != count(b, x) {
			return false
		}
	}
	return true
}

func count(list []string, x string) int {
	n := 0
	for _, y := range list {
		if y == x {
			n++
		}
	}
	return n
}

type selfRefRule struct{ meta }

var selfRef = &selfRefRule{meta{
	name: "selfref",
	doc:  "reports relationships whose two endpoints are the same column",
}}

func (r *selfRefRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		if ref.Left.Table == nil || ref.Right.Table == nil || ref.Left.Table != ref.Right.Table {
			continue
		}
		if strings.Join(ref.Left.Columns, ",") == strings.Join(ref.Right.Columns, ",") {
			out = append(out, r.warnf(ref.Node.Pos(), "relationship references column %s.%s on both sides",
				ref.Left.Table.Decl.Name.String(), strings.Join(ref.Left.Columns, ",")))
		}
	}
	return out
}

// refCycleFold walks the file's foreign-key graph.
type refCycleFold struct{ meta }

var refCycle = &refCycleFold{meta{
	name: "refcycle",
	doc:  "reports foreign-key dependency cycles between tables; cyclic FKs complicate inserts and deletes",
}}

func (fo *refCycleFold) File(p *FilePass) {
	// Build FK edges: table with the FK -> referenced table.
	edges := map[*check.TableInfo][]*check.TableInfo{}
	for _, r := range p.Info.Refs {
		fk, target, ok := fkSides(r)
		if !ok || fk.Table == nil || target.Table == nil || fk.Table == target.Table {
			continue
		}
		edges[fk.Table] = append(edges[fk.Table], target.Table)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	state := map[*check.TableInfo]int{}
	var stack []*check.TableInfo
	var visit func(t *check.TableInfo)
	reported := map[string]bool{}
	visit = func(t *check.TableInfo) {
		state[t] = gray
		stack = append(stack, t)
		for _, next := range edges[t] {
			switch state[next] {
			case white:
				visit(next)
			case gray:
				// found a cycle: slice it out of the stack
				var names []string
				for i := len(stack) - 1; i >= 0; i-- {
					names = append(names, stack[i].Decl.Name.String())
					if stack[i] == next {
						break
					}
				}
				sort.Strings(names)
				key := strings.Join(names, "->")
				if !reported[key] {
					reported[key] = true
					p.Reportf(t.Decl.Pos(), "foreign-key cycle between tables: %s", strings.Join(names, ", "))
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[t] = black
	}
	for _, ti := range p.Info.Tables {
		if state[ti] == white {
			visit(ti)
		}
	}
}

// caseCollisionCheck judges one table's columns as a rule and the
// file's tables and enums as a fold.
type caseCollisionCheck struct{ meta }

var caseCollision = &caseCollisionCheck{meta{
	name: "casecollision",
	doc:  "reports names within one namespace that differ only in letter case",
}}

func (c *caseCollisionCheck) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	ti := f.Table
	if ti == nil {
		return out
	}
	var names []string
	var poss []token.Position
	for _, cd := range ti.Columns {
		names = append(names, cd.Col.Name.Name())
		poss = append(poss, cd.Col.Pos())
	}
	return append(out, c.groups("column", names, poss)...)
}

func (c *caseCollisionCheck) File(p *FilePass) {
	var names []string
	var poss []token.Position
	for _, ti := range p.Info.Tables {
		names = append(names, ti.Key)
		poss = append(poss, ti.Decl.Pos())
	}
	for _, dg := range c.groups("table", names, poss) {
		p.diags = append(p.diags, dg)
	}
	names, poss = nil, nil
	for _, e := range p.Info.Enums {
		names = append(names, e.Key)
		poss = append(poss, e.Decl.Pos())
	}
	for _, dg := range c.groups("enum", names, poss) {
		p.diags = append(p.diags, dg)
	}
}

// groups reports every set of distinct names that differ only in
// case, at the last of them by position, listing them in source order.
// Names are distinct within a namespace; a repeated spelling counts
// once.
func (c *caseCollisionCheck) groups(kind string, names []string, poss []token.Position) []diag.Diagnostic {
	var out []diag.Diagnostic
	byLower := map[string][]int{}
	var order []string // lower-case keys in first appearance order
	spelled := map[string]bool{}
	for i, n := range names {
		if spelled[n] {
			continue
		}
		spelled[n] = true
		l := strings.ToLower(n)
		if _, seen := byLower[l]; !seen {
			order = append(order, l)
		}
		byLower[l] = append(byLower[l], i)
	}
	for _, l := range order {
		group := byLower[l]
		if len(group) < 2 {
			continue
		}
		// in source order: insert each index behind the ones before it
		var sorted []int
		for _, i := range group {
			var next []int
			placed := false
			for _, j := range sorted {
				if !placed && posBefore(poss[i], poss[j]) {
					next = append(next, i)
					placed = true
				}
				next = append(next, j)
			}
			if !placed {
				next = append(next, i)
			}
			sorted = next
		}
		var listed []string
		for _, i := range sorted {
			listed = append(listed, names[i])
		}
		out = append(out, c.warnf(poss[sorted[len(sorted)-1]], "%s names %s differ only in case", kind, strings.Join(listed, " and ")))
	}
	return out
}

// posBefore reports whether a comes before b by line, then column.
func posBefore(a, b token.Position) bool {
	if a.Line() != b.Line() {
		return a.Line() < b.Line()
	}
	return a.Column() < b.Column()
}
