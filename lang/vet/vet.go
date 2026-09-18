// Package vet reports DBML that is legal but suspicious — the analogue of
// go vet. Where the check package answers "is this valid DBML?", vet
// answers "is this what you meant?": redundancy, silent overrides, likely
// modeling mistakes.
//
// The design follows golang.org/x/tools/go/analysis in spirit: each
// check is an Analyzer with a name and a doc string, registered once,
// independent of every other and of the CLI. A check is judged
// declaration by declaration (a Rule: a pure function of one
// declaration's syntax and the facts the checker resolved for it,
// D104), or over the whole file (a Fold: the checker's model plus what
// every declaration's rules summarized), or both. Nothing is computed
// twice: the rules see the check package's resolved Info, and a memo
// answers a declaration whose inputs are the objects they were.
package vet

import (
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	golang "github.com/Piechutowski/volt/nao/gen/golang"
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
// the checker resolved for it (D104). It returns its warnings in the
// order it found them.
type Rule interface {
	Analyzer
	Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic
}

// Fold is a check over the whole file: the checker's model and the
// summaries of every declaration.
type Fold interface {
	Analyzer
	File(p *FilePass)
}

// Facts is what the checker resolved for one declaration: the checked
// table of a Table, the partial of a TablePartial, and the
// relationships that start at the declaration's own columns — a Ref
// element's, or the inline refs of a table's columns, a partial's
// inline ref reaching every table that injects it. Nil and empty for a
// declaration the checker refused (a duplicate table) or one it does
// not model.
type Facts struct {
	Table   *check.TableInfo
	Partial *check.PartialInfo
	Refs    []*check.RefInfo
}

// Summary is what the folds need of one declaration: the enum keys its
// columns name, in both spellings the file-wide rules look up, and the
// unqualified table spellings it uses.
type Summary struct {
	EnumsUsed []string
	NamesUsed []string
}

// FilePass carries everything a Fold may inspect.
type FilePass struct {
	File      *ast.File
	Info      *check.Info
	Summaries []Summary // one per File.Decls entry, in order

	plan  **golang.Plan // shared by the folds of one run
	fold  Fold
	diags []diag.Diagnostic
}

// Plan is the file's naming plan: the checker's own when the caller
// handed one to RunWithPlan, else built once for all folds of this
// run. A fold that reasons about generated Go names asks here instead
// of planning again (D81).
func (p *FilePass) Plan() *golang.Plan {
	if *p.plan == nil {
		*p.plan = golang.PlanBuild(p.File, p.Info)
	}
	return *p.plan
}

// Reportf records a warning attributed to the running fold.
func (p *FilePass) Reportf(pos token.Position, format string, args ...any) {
	p.diags = append(p.diags, diag.Warningf(pos, "vet/"+p.fold.Name(), format, args...))
}

// meta names an analyzer.
type meta struct{ name, doc string }

func (m *meta) Name() string { return m.name }
func (m *meta) Doc() string  { return m.doc }

// warnf is one warning attributed to the analyzer.
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

var registry []Analyzer

func register(a Analyzer) { registry = append(registry, a) }

// Run executes the given analyzers (all registered ones if none are named)
// over a checked file and returns their warnings, sorted by position.
func Run(f *ast.File, info *check.Info, analyzers ...Analyzer) []diag.Diagnostic {
	return RunWithPlan(f, info, nil, analyzers...)
}

// RunWithPlan is Run with the file's naming plan already built, as the
// checker has it, so no analyzer builds it again.
func RunWithPlan(f *ast.File, info *check.Info, plan *golang.Plan, analyzers ...Analyzer) []diag.Diagnostic {
	return RunMemo(f, info, plan, nil, analyzers...)
}

// Memo remembers a file's vetted declarations across its versions
// (D104): a declaration that is the node it was, with the facts it
// had, under the rules it had, warns what it warned. The zero value is
// ready; a Memo belongs to one file and one goroutine at a time.
type Memo struct {
	prev, next map[ast.Decl]*memoDecl
	// Hits and Misses count the declarations answered from the memo
	// and judged afresh by the last RunMemo.
	Hits, Misses int
}

// memoDecl is one call of DeclVet: its inputs (the declaration is the
// map key) and its outputs.
type memoDecl struct {
	facts   factsKey
	rules   []Rule
	diags   [][]diag.Diagnostic
	summary Summary
}

// factsKey is the identity of a declaration's facts (D87): the checked
// table, the partial's declaration and use count, and each
// relationship's node, operator, endpoint tables and columns — every
// value a rule reads through Facts, as the object or the value it is.
type factsKey struct {
	table   *check.TableInfo
	partial *ast.TablePartial
	uses    int
	refs    []refKey
}

type refKey struct {
	node                ast.Node
	op                  token.Kind
	inline              bool
	left, right         *check.TableInfo
	leftCols, rightCols []string
}

func factsKeyOf(f Facts) factsKey {
	k := factsKey{table: f.Table}
	if f.Partial != nil {
		k.partial, k.uses = f.Partial.Decl, f.Partial.Uses
	}
	for _, r := range f.Refs {
		k.refs = append(k.refs, refKey{
			node: r.Node, op: r.Op, inline: r.Inline,
			left: r.Left.Table, right: r.Right.Table,
			leftCols: r.Left.Columns, rightCols: r.Right.Columns,
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
		if r.node != s.node || r.op != s.op || r.inline != s.inline || r.left != s.left || r.right != s.right ||
			!stringsEqual(r.leftCols, s.leftCols) || !stringsEqual(r.rightCols, s.rightCols) {
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

// declResult is one declaration's verdict: each rule's warnings, in
// rule order, and the summary the folds read.
type declResult struct {
	diags   [][]diag.Diagnostic
	summary Summary
	reused  bool
}

// RunMemo is RunWithPlan through a memo of the file's earlier runs; a
// nil memo judges every declaration. The rules judge every declaration
// of the file, then the folds judge the file; the warnings come
// analyzer by analyzer in registration order, each analyzer's in
// declaration order, then sorted by position, so two warnings at one
// position stand in the order the analyzers were registered.
func RunMemo(f *ast.File, info *check.Info, plan *golang.Plan, memo *Memo, analyzers ...Analyzer) []diag.Diagnostic {
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
				results[i] = declResult{diags: e.diags, summary: e.summary, reused: true}
				return
			}
		}
		diags, sum := DeclVet(d, facts[i], rules)
		results[i] = declResult{diags: diags, summary: sum}
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
				memo.next[d] = &memoDecl{facts: factsKeyOf(facts[i]), rules: rules, diags: results[i].diags, summary: results[i].summary}
			}
		}
		memo.prev, memo.next = memo.next, nil
	}

	summaries := make([]Summary, len(f.Decls))
	for i := range results {
		summaries[i] = results[i].summary
	}
	foldDiags := make([][]diag.Diagnostic, len(folds))
	for j, fo := range folds {
		p := &FilePass{File: f, Info: info, Summaries: summaries, plan: &plan, fold: fo}
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

// factsByDecl resolves each declaration's facts from the checker's
// model, aligned with f.Decls: a relationship belongs to the
// declaration it starts at, the Ref element or the table whose column
// carries the inline ref.
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
		} else if r.Left.Table != nil {
			refs[r.Left.Table.Decl] = append(refs[r.Left.Table.Decl], r)
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
// for it and the rules (D86, D104). It answers each rule's warnings,
// in rule order, and the declaration's summary for the folds.
func DeclVet(d ast.Decl, f Facts, rules []Rule) ([][]diag.Diagnostic, Summary) {
	nodes := nodesOf(d)
	out := make([][]diag.Diagnostic, len(rules))
	for i, r := range rules {
		out[i] = r.Decl(d, nodes, f)
	}
	return out, summaryOf(nodes, f)
}

// nodesOf lists a declaration's nodes in source order, itself first:
// the order ast.Inspect visits them, produced without a function value
// (D102) from ast.Children.
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

// summaryOf is what the folds need of one declaration.
func summaryOf(nodes []ast.Node, f Facts) Summary {
	var s Summary
	if f.Table != nil {
		seen := map[string]bool{}
		for _, cd := range f.Table.Columns {
			t := cd.Col.Type.Name
			if t.Schema() == "" {
				s.EnumsUsed, seen = enumUse(s.EnumsUsed, seen, "public."+t.Base())
			}
			s.EnumsUsed, seen = enumUse(s.EnumsUsed, seen, t.String())
		}
	}
	for _, r := range f.Refs {
		// endpoint tables were written as names; the syntax holds the
		// spelling actually used
		switch n := r.Node.(type) {
		case *ast.Ref:
			s.NamesUsed = nameUse(s.NamesUsed, n.Left.Table)
			s.NamesUsed = nameUse(s.NamesUsed, n.Right.Table)
		case *ast.Setting:
			if rv, ok := n.Value.(*ast.RefValue); ok {
				s.NamesUsed = nameUse(s.NamesUsed, rv.Endpoint.Table)
			}
		}
	}
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.TableGroup:
			for _, m := range n.Members {
				s.NamesUsed = nameUse(s.NamesUsed, m)
			}
		case *ast.ViewCategory:
			for _, m := range n.Names {
				s.NamesUsed = nameUse(s.NamesUsed, m)
			}
		case *ast.Records:
			if n.Table != nil {
				s.NamesUsed = nameUse(s.NamesUsed, n.Table)
			}
		}
	}
	return s
}

func enumUse(out []string, seen map[string]bool, key string) ([]string, map[string]bool) {
	if !seen[key] {
		seen[key] = true
		out = append(out, key)
	}
	return out, seen
}

func nameUse(out []string, q *ast.QualName) []string {
	if q.Schema() == "" {
		out = append(out, q.Base())
	}
	return out
}
