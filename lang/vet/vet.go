// Package vet reports Volt that is legal but suspicious — the analogue of
// go vet. Where the check package answers "is this valid Volt?", vet
// answers "is this what you meant?": redundancy, silent overrides, likely
// modeling mistakes.
//
// Each check is an Analyzer with a name and a doc string, judged
// declaration by declaration (a Rule: a pure function of one
// declaration's syntax and the facts the checker resolved for it),
// or over the whole file (a Fold), or both.
package vet

import (
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
)

// Analyzer is one vet check, named for its diagnostic code and
// documented for docs/lint.md. Every analyzer is a Rule, a Fold, or
// both.
type Analyzer interface {
	Name() string // short lower-case identifier, used in diagnostic codes
	Doc() string  // one-sentence description of what it reports
}

// Rule is a check on one declaration: a pure function of the
// declaration, its nodes in source order (itself first) and the facts
// the checker resolved for it. It returns its warnings in the
// order it found them.
type Rule interface {
	Analyzer
	Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic
}

// Fold is a check over the whole file: the checker's model and its
// naming plan.
type Fold interface {
	Analyzer
	File(p *FilePass)
}

// Facts is what the checker resolved for one declaration: the checked
// table of a Table, the partial of a TablePartial, and the
// relationships that start at the declaration's own columns. Nil and
// empty for a declaration the checker refused or one it does not
// model.
type Facts struct {
	Table   *check.TableInfo
	Partial *check.PartialInfo
	Refs    []*check.RefInfo
}

// FilePass carries everything a Fold may inspect.
type FilePass struct {
	File *ast.File
	Info *check.Info

	plan  **naming.Plan
	fold  Fold
	diags []diag.Diagnostic
}

// Plan is the file's naming plan: the checker's own when the caller
// handed one to RunWithPlan, else built once for all folds of this
// run.
func (p *FilePass) Plan() *naming.Plan {
	if *p.plan == nil {
		*p.plan = naming.PlanBuild(p.File, p.Info)
	}
	return *p.plan
}

// Reportf records a warning attributed to the running fold.
func (p *FilePass) Reportf(pos token.Position, format string, args ...any) {
	p.diags = append(p.diags, diag.Warningf(pos, "vet/"+p.fold.Name(), format, args...))
}

type meta struct{ name, doc string }

func (m *meta) Name() string { return m.name }
func (m *meta) Doc() string  { return m.doc }

func (m *meta) warnf(pos token.Position, format string, args ...any) diag.Diagnostic {
	return diag.Warningf(pos, "vet/"+m.name, format, args...)
}

// All returns the registered analyzers in a stable order.
func All() []Analyzer {
	out := make([]Analyzer, len(registry))
	copy(out, registry)
	return out
}

// ByName returns the named analyzer, or nil.
func ByName(name string) Analyzer {
	for _, a := range registry {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

var registry = []Analyzer{
	unusedPartial,
	redundantIndex, emptyBody, shadowedColumn,
	missingPK, refTypeMismatch, refTargetNotUnique, selfRef, refCycle, caseCollision,
	modelName, dynName,
}

// Run executes the given analyzers (all registered ones if none are named)
// over a checked file and returns their warnings, sorted by position.
func Run(f *ast.File, info *check.Info, analyzers ...Analyzer) []diag.Diagnostic {
	return RunWithPlan(f, info, nil, analyzers...)
}

// RunWithPlan is Run with the file's naming plan already built, as the
// checker has it, so no analyzer builds it again.
func RunWithPlan(f *ast.File, info *check.Info, plan *naming.Plan, analyzers ...Analyzer) []diag.Diagnostic {
	return RunMemo(f, info, plan, nil, analyzers...)
}

// Memo remembers a file's vetted declarations across its versions
// : a declaration that is the node it was, with the facts it
// had, under the rules it had, warns what it warned. The zero value is
// ready; a Memo belongs to one file and one goroutine at a time.
type Memo struct {
	prev, next map[ast.Decl]*memoDecl
	// Hits and Misses count the declarations answered from the memo
	// and judged afresh by the last RunMemo.
	Hits, Misses int
}

type memoDecl struct {
	facts factsKey
	rules []Rule
	diags [][]diag.Diagnostic
}

type factsKey struct {
	table   *check.TableInfo
	partial *ast.TablePartial
	uses    int
	refs    []refKey
}

type refKey struct {
	node               ast.Node
	inline             bool
	fk, target         *check.TableInfo
	fkCols, targetCols []string
}

func factsKeyOf(f Facts) factsKey {
	k := factsKey{table: f.Table}
	if f.Partial != nil {
		k.partial, k.uses = f.Partial.Decl, f.Partial.Uses
	}
	for _, r := range f.Refs {
		k.refs = append(k.refs, refKey{
			node: r.Node, inline: r.Inline,
			fk: r.FK.Table, target: r.Target.Table,
			fkCols: r.FK.Columns, targetCols: r.Target.Columns,
		})
	}
	return k
}

func (k factsKey) equal(o factsKey) bool {
	if k.table != o.table || k.partial != o.partial || k.uses != o.uses || len(k.refs) != len(o.refs) {
		return false
	}
	for i, r := range k.refs {
		s := o.refs[i]
		if r.node != s.node || r.inline != s.inline || r.fk != s.fk || r.target != s.target ||
			!stringsEqual(r.fkCols, s.fkCols) || !stringsEqual(r.targetCols, s.targetCols) {
			return false
		}
	}
	return true
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rulesEqual(a, b []Rule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type declResult struct {
	diags  [][]diag.Diagnostic
	reused bool
}

// RunMemo is RunWithPlan through a memo of the file's earlier runs; a
// nil memo judges every declaration. The warnings come analyzer by
// analyzer in registration order, each analyzer's in declaration
// order, then sorted by position.
func RunMemo(f *ast.File, info *check.Info, plan *naming.Plan, memo *Memo, analyzers ...Analyzer) []diag.Diagnostic {
	if len(analyzers) == 0 {
		analyzers = All()
	}
	var rules []Rule
	var folds []Fold
	for _, a := range analyzers {
		if r, ok := a.(Rule); ok {
			rules = append(rules, r)
		}
		if fo, ok := a.(Fold); ok {
			folds = append(folds, fo)
		}
	}
	facts := factsByDecl(f, info)
	results := make([]declResult, len(f.Decls))
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
	}
	par.For(len(f.Decls), func(i int) {
		d := f.Decls[i]
		if d == nil {
			results[i] = declResult{diags: make([][]diag.Diagnostic, len(rules))}
			return
		}
		if memo != nil {
			if e := memo.prev[d]; e != nil && e.facts.equal(factsKeyOf(facts[i])) && rulesEqual(e.rules, rules) {
				results[i] = declResult{diags: e.diags, reused: true}
				return
			}
		}
		results[i] = declResult{diags: DeclVet(d, facts[i], rules)}
	})
	if memo != nil {
		memo.next = make(map[ast.Decl]*memoDecl, len(f.Decls))
		for i, d := range f.Decls {
			if d == nil {
				continue
			}
			if results[i].reused {
				memo.Hits++
				memo.next[d] = memo.prev[d]
			} else {
				memo.Misses++
				memo.next[d] = &memoDecl{facts: factsKeyOf(facts[i]), rules: rules, diags: results[i].diags}
			}
		}
		memo.prev, memo.next = memo.next, nil
	}

	foldDiags := make([][]diag.Diagnostic, len(folds))
	for j, fo := range folds {
		p := &FilePass{File: f, Info: info, plan: &plan, fold: fo}
		fo.File(p)
		foldDiags[j] = p.diags
	}

	var out []diag.Diagnostic
	ri, fi := 0, 0
	for _, a := range analyzers {
		if _, ok := a.(Rule); ok {
			for i := range results {
				out = append(out, results[i].diags[ri]...)
			}
			ri++
		}
		if _, ok := a.(Fold); ok {
			out = append(out, foldDiags[fi]...)
			fi++
		}
	}
	diag.Sort(out)
	return out
}

func factsByDecl(f *ast.File, info *check.Info) []Facts {
	tables := make(map[*ast.Table]*check.TableInfo, len(info.Tables))
	for _, ti := range info.Tables {
		tables[ti.Decl] = ti
	}
	partials := make(map[*ast.TablePartial]*check.PartialInfo, len(info.Partials))
	for _, pi := range info.Partials {
		partials[pi.Decl] = pi
	}
	refs := map[ast.Decl][]*check.RefInfo{}
	for _, r := range info.Refs {
		if n, ok := r.Node.(*ast.Ref); ok {
			refs[n] = append(refs[n], r)
		} else if r.FK.Table != nil {
			refs[r.FK.Table.Decl] = append(refs[r.FK.Table.Decl], r)
		}
	}
	out := make([]Facts, len(f.Decls))
	for i, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Table:
			out[i].Table = tables[d]
		case *ast.TablePartial:
			out[i].Partial = partials[d]
		}
		if d != nil {
			out[i].Refs = refs[d]
		}
	}
	return out
}

// DeclVet judges one declaration under the given rules: a pure
// function of the declaration's syntax, the facts the checker resolved
// for it and the rules. It answers each rule's warnings, in rule order.
func DeclVet(d ast.Decl, f Facts, rules []Rule) [][]diag.Diagnostic {
	nodes := nodesOf(d)
	out := make([][]diag.Diagnostic, len(rules))
	for i, r := range rules {
		out[i] = r.Decl(d, nodes, f)
	}
	return out
}

func nodesOf(root ast.Node) []ast.Node {
	var out []ast.Node
	stack := []ast.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, n)
		kids := ast.Children(n)
		for i := len(kids) - 1; i >= 0; i-- {
			stack = append(stack, kids[i])
		}
	}
	return out
}
