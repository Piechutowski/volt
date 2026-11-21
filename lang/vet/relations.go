// Relationship and modeling analyzers: legal schemas that are likely to
// misbehave as databases.
package vet

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

type missingPKRule struct{ meta }

var missingPK = &missingPKRule{meta{
	name: "missingpk",
	doc:  "reports tables with no primary key, no column carrying pk",
}}

func (r *missingPKRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	if ti := f.Table; ti != nil && len(ti.PK()) == 0 {
		out = append(out, r.warnf(ti.Decl.Pos(), "table %q has no primary key", ti.Decl.Name.Name()))
	}
	return out
}

type refTypeRule struct{ meta }

var refTypeMismatch = &refTypeRule{meta{
	name: "reftype",
	doc:  "reports relationships whose two columns have different declared types",
}}

func (r *refTypeRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		if ref.FK.Table == nil || ref.Target.Table == nil {
			continue
		}
		n := min(len(ref.FK.Columns), len(ref.Target.Columns))
		for i := 0; i < n; i++ {
			lc := ref.FK.Table.Column(ref.FK.Columns[i])
			rc := ref.Target.Table.Column(ref.Target.Columns[i])
			if lc == nil || rc == nil || !lc.Type.Known() || !rc.Type.Known() {
				continue
			}
			lt, rt := lc.Col.Type.String(), rc.Col.Type.String()
			if !strings.EqualFold(lt, rt) {
				out = append(out, r.warnf(ref.Node.Pos(), "relationship joins %s.%s (%s) to %s.%s (%s); the types differ",
					ref.FK.Table.Decl.Name.Name(), ref.FK.Columns[i], lt,
					ref.Target.Table.Decl.Name.Name(), ref.Target.Columns[i], rt))
			}
		}
	}
	return out
}

type refTargetRule struct{ meta }

var refTargetNotUnique = &refTargetRule{meta{
	name: "reftarget",
	doc:  "reports foreign keys referencing columns that are not unique on the one side: not the primary key, a unique column or the key of a unique index without where:",
}}

func (r *refTargetRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		target := ref.Target
		if target.Table == nil {
			continue
		}
		if target.Table.ColumnsUnique(target.Columns) {
			continue
		}
		out = append(out, r.warnf(ref.Node.Pos(), "referenced column(s) %s.(%s) are not unique (not the primary key, a unique column or the key of a unique index without where:), so the database cannot enforce this foreign key",
			target.Table.Decl.Name.Name(), strings.Join(target.Columns, ", ")))
	}
	return out
}

type selfRefRule struct{ meta }

var selfRef = &selfRefRule{meta{
	name: "selfref",
	doc:  "reports relationships whose two endpoints are the same column",
}}

func (r *selfRefRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, ref := range f.Refs {
		if ref.FK.Table == nil || ref.Target.Table == nil || ref.FK.Table != ref.Target.Table {
			continue
		}
		if strings.Join(ref.FK.Columns, ",") == strings.Join(ref.Target.Columns, ",") {
			out = append(out, r.warnf(ref.Node.Pos(), "relationship references column %s.%s on both sides",
				ref.FK.Table.Decl.Name.Name(), strings.Join(ref.FK.Columns, ",")))
		}
	}
	return out
}

type refCycleFold struct{ meta }

var refCycle = &refCycleFold{meta{
	name: "refcycle",
	doc:  "reports foreign-key dependency cycles between tables; cyclic FKs complicate inserts and deletes",
}}

func (fo *refCycleFold) File(p *FilePass) {
	edges := map[*check.TableInfo][]*check.TableInfo{}
	for _, r := range p.Info.Refs {
		if r.FK.Table == nil || r.Target.Table == nil || r.FK.Table == r.Target.Table {
			continue
		}
		edges[r.FK.Table] = append(edges[r.FK.Table], r.Target.Table)
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
				var names []string
				for i := len(stack) - 1; i >= 0; i-- {
					names = append(names, stack[i].Decl.Name.Name())
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
}

func (c *caseCollisionCheck) groups(kind string, names []string, poss []token.Position) []diag.Diagnostic {
	var out []diag.Diagnostic
	byLower := map[string][]int{}
	var order []string
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

func posBefore(a, b token.Position) bool {
	if a.Line() != b.Line() {
		return a.Line() < b.Line()
	}
	return a.Column() < b.Column()
}
