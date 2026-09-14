package main

// The purity gate (D86) and the immutability gate (D92): every
// computation a session memo answers must be a function of its
// explicit inputs, and what it answers must stay what it was. This
// test loads the three modules with type information and walks
// function bodies with a taint: in the purity walk, over each memoized
// function and transitively everything it calls, a tainted value is
// one derived from an input, and the walk refuses reading mutable
// package state, writing through an input, and calling into a package
// that keeps state. In the immutability walk, over every function of
// the modules, a tainted value is one that came from outside the
// function (a parameter, package state, a memoized result), and the
// walk records every write through one to a memoized result type or
// to a node or token; a write is refused unless the function it was
// reached from is the result's own producer, or the value's own maker.
// Both walks are conservative: a value that comes out of a call whose
// inputs were tainted is tainted, a closure's parameters are assumed
// to carry inputs, and a standard function that writes its argument
// (sort, slices, maps) is a write. A write to a field of a struct held
// by value is the variable's own and counts for nothing.

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/Piechutowski/volt"

// pureTargets are the memoized computations (D86), by package path and
// name; each must be a top-level function.
var pureTargets = []struct{ pkg, name string }{
	{module + "/lang/check", "tableCheck"},
	{module + "/nao/gen/golang", "tableBuild"},
	{module + "/lang", "tableSpecs"},
	{module + "/lang", "selectCheck"},
	{module + "/lang", "itemLower"},
	{module + "/lsp", "tableOccurrences"},
}

// immutableVars are package-level variables a pure function may read:
// lookup tables written once at initialization. The gate also proves
// none of them is assigned anywhere but its declaration.
var immutableVars = map[string]bool{
	module + "/lang.crudMethodSuffixes":           true,
	module + "/lang.compareSQL":                   true,
	module + "/lang.goKeywords":                   true,
	module + "/lang.reservedParamNames":           true,
	module + "/lang.resourceActions":              true,
	module + "/lang.actionByLower":                true,
	module + "/lang.queryValueTypes":              true,
	module + "/lang/check.tableSettings":          true,
	module + "/lang/check.columnSettings":         true,
	module + "/lang/check.indexSettings":          true,
	module + "/lang/check.settingsCheck":          true,
	module + "/lang/check.refSettings":            true,
	module + "/lang/check.integerTypes":           true,
	module + "/lang/check.requiredKinds":          true,
	module + "/nao/gen/golang.commonInitialisms":  true,
	module + "/nao/gen/golang.typeMap":            true,
	module + "/nao/gen/golang.goKeywords":         true,
	module + "/nao/gen/golang.dynWrapperSuffixes": true,
	module + "/nao/gen/sqlite.sqliteKeywords":     true,
	module + "/nao/gen/sqlite.plainIdent":         true,
	module + "/nao/inflect.irregular":             true,
}

// pureStdlib are the standard packages whose functions keep no state:
// the trust boundary with Go (D97), kept exactly as wide as the memoized
// computations reach, since the walk fails on an entry none of them
// calls into.
var pureStdlib = map[string]bool{
	"strings": true, "strconv": true, "unicode": true, // functions of their arguments and of constant tables
	"fmt":    true, // formatting: what it pools internally never reaches an answer
	"sort":   true, // writes only what it is handed, which the gate counts as a write
	"regexp": true, // a compiled expression answers the same question every time
}

// pureStdlibFuncs are the single standard functions trusted outside
// pureStdlib's packages (D97): each names why. A position's offset and
// line read the file's base through an atomic, whose Load is a read of
// its receiver and nothing else; the package it lives in is not
// trusted as a whole.
var pureStdlibFuncs = map[string]bool{
	"(*sync/atomic.Int32).Load": true,
}

// stdlibMutators are the standard functions that write through an
// argument, by the index of the argument written: a call to one is a
// write to it (D92). Every other function of pureStdlib only reads
// its arguments.
var stdlibMutators = map[string]int{
	"sort.Sort": 0, "sort.Stable": 0, "sort.Slice": 0, "sort.SliceStable": 0,
	"sort.Strings": 0, "sort.Ints": 0, "sort.Float64s": 0,
	"slices.Sort": 0, "slices.SortFunc": 0, "slices.SortStableFunc": 0, "slices.Reverse": 0,
	"slices.Delete": 0, "slices.DeleteFunc": 0, "slices.Insert": 0, "slices.Replace": 0,
	"slices.Compact": 0, "slices.CompactFunc": 0,
	"maps.Copy": 0, "maps.DeleteFunc": 0,
}

// valuePackages declare the front end's values: nodes, tokens and
// positions. Their types are written only by the packages that make
// them (D92); everyone else holds them as read-only inputs.
var valuePackages = map[string][]string{
	module + "/lang/ast":   {module + "/lang/ast", module + "/lang/parser"},
	module + "/lang/token": {module + "/lang/token", module + "/lang/scanner", module + "/lang/parser"},
}

// closureStops are packages whose types a memoized result may hold
// without their being results: diagnostics are plain values every
// phase makes and sorts.
var closureStops = map[string]bool{module + "/lang/diag": true}

const fixtureImpure = module + "/cmd/volt/testdata/impure"
const fixtureMutable = module + "/cmd/volt/testdata/mutable"

var gateOnce struct {
	sync.Once
	pkgs []*packages.Package
	err  error
}

// gatePackages loads the three modules and the fixtures with type
// information, once per test binary.
func gatePackages(t *testing.T) []*packages.Package {
	t.Helper()
	gateOnce.Do(func() {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			gateOnce.err = err
			return
		}
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
				packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
			Dir: root,
		}
		gateOnce.pkgs, gateOnce.err = packages.Load(cfg, module+"/...", module+"/lsp/...", module+"/cmd/volt", fixtureImpure, fixtureMutable)
	})
	if gateOnce.err != nil {
		t.Fatal(gateOnce.err)
	}
	return gateOnce.pkgs
}

// newPurity indexes the loaded packages' function declarations for a
// walk; shared selects the immutability walk over the purity walk.
func newPurity(t *testing.T, pkgs []*packages.Package, shared bool) *purity {
	t.Helper()
	p := &purity{byPath: map[string]*packages.Package{}, decls: map[*types.Func]funcDecl{}, summaries: map[string]*summary{}, active: map[string]bool{}, shared: shared}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		if len(pkg.Errors) > 0 && strings.HasPrefix(pkg.PkgPath, module) {
			t.Fatalf("%s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		p.byPath[pkg.PkgPath] = pkg
		for _, f := range pkg.Syntax {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					if obj, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func); ok {
						p.decls[obj] = funcDecl{pkg, fd}
					}
				}
			}
		}
	})
	return p
}

// targetFunc resolves one memoized computation.
func (p *purity) targetFunc(t *testing.T, pkgPath, name string) *types.Func {
	t.Helper()
	pkg := p.byPath[pkgPath]
	if pkg == nil {
		t.Fatalf("package %s not loaded", pkgPath)
	}
	obj, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
	if !ok {
		t.Fatalf("%s.%s: no such function", pkgPath, name)
	}
	return obj
}

// allTainted is the taint of a function whose every parameter is an
// input.
func (p *purity) allTainted(fn *types.Func) []bool {
	fd := p.decls[fn]
	var taint []bool
	for _, field := range fd.decl.Type.Params.List {
		for range field.Names {
			taint = append(taint, true)
		}
		if len(field.Names) == 0 {
			taint = append(taint, true)
		}
	}
	return taint
}

func TestMemoizedComputationsArePure(t *testing.T) {
	p := newPurity(t, gatePackages(t), false)
	for _, target := range pureTargets {
		obj := p.targetFunc(t, target.pkg, target.name)
		if obj.Type().(*types.Signature).Recv() != nil {
			t.Errorf("%s.%s: a memoized computation is a top-level function, not a method", target.pkg, target.name)
			continue
		}
		p.analyze(obj, false, p.allTainted(obj))
	}
	p.immutableVarsNeverAssigned()
	// The trusted standard packages are exactly the ones reached: an
	// entry no target calls into is trust for nothing (D97).
	for name := range pureStdlibFuncs {
		if !p.stdlibFuncsUsed[name] {
			p.problems = append(p.problems, fmt.Sprintf("pureStdlibFuncs trusts %s, which no memoized computation reaches", name))
		}
	}
	for path := range pureStdlib {
		if !p.stdlibUsed[path] {
			p.problems = append(p.problems, fmt.Sprintf("pureStdlib trusts %s, which no memoized computation reaches", path))
		}
	}
	sort.Strings(p.problems)
	for _, problem := range p.problems {
		t.Error(problem)
	}
}

// TestMemoizedResultsAreImmutable proves what a memo answers stays
// what it was (D92): no function of the three modules writes, through
// a value that came from outside it, into a memoized result type
// unless the function is reachable from the result's producer, nor
// into a node or a token outside the packages that make them.
func TestMemoizedResultsAreImmutable(t *testing.T) {
	p := newPurity(t, gatePackages(t), true)
	var targets []*types.Func
	for _, target := range pureTargets {
		targets = append(targets, p.targetFunc(t, target.pkg, target.name))
	}
	p.producersSet(targets)
	reported := map[string]bool{}
	for _, fn := range p.startingPoints(func(path string) bool {
		return strings.HasPrefix(path, module) && !strings.HasPrefix(path, module+"/cmd/volt/testdata/")
	}) {
		for _, w := range p.writesOutsideProducers(fn) {
			if !reported[w.at] {
				reported[w.at] = true
				p.problems = append(p.problems, fmt.Sprintf("%s: %s.%s is written outside its producer (reached from %s)", w.at, w.typ.Pkg().Name(), w.typ.Name(), fn.FullName()))
			}
		}
	}
	sort.Strings(p.problems)
	for _, problem := range p.problems {
		t.Error(problem)
	}
}

// producersSet records the memoized result types each target's results
// reach, the value packages' types, and what each producer can reach.
func (p *purity) producersSet(targets []*types.Func) {
	p.closures = map[*types.TypeName][]int{}
	p.targets = map[*types.Func]int{}
	p.reach = make([]map[*types.Func]bool, len(targets))
	for i, fn := range targets {
		p.targets[fn] = i
		p.reach[i] = p.reachable(fn)
		seen := map[*types.TypeName]bool{}
		var add func(t types.Type)
		add = func(t types.Type) {
			switch t := types.Unalias(t).(type) {
			case *types.Named:
				obj := t.Obj()
				if obj.Pkg() == nil || !strings.HasPrefix(obj.Pkg().Path(), module) || seen[obj] {
					return
				}
				if valuePackages[obj.Pkg().Path()] != nil || closureStops[obj.Pkg().Path()] {
					return
				}
				seen[obj] = true
				p.closures[obj] = append(p.closures[obj], i)
				add(t.Underlying())
			case *types.Pointer:
				add(t.Elem())
			case *types.Slice:
				add(t.Elem())
			case *types.Array:
				add(t.Elem())
			case *types.Map:
				add(t.Key())
				add(t.Elem())
			case *types.Chan:
				add(t.Elem())
			case *types.Struct:
				for j := 0; j < t.NumFields(); j++ {
					add(t.Field(j).Type())
				}
			}
		}
		sig := fn.Type().(*types.Signature)
		for j := 0; j < sig.Results().Len(); j++ {
			add(sig.Results().At(j).Type())
		}
	}
	for path := range valuePackages {
		pkg := p.byPath[path]
		if pkg == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			if tn, ok := scope.Lookup(name).(*types.TypeName); ok {
				p.closures[tn] = append(p.closures[tn], -1)
			}
		}
	}
}

// reachable is every function a call from fn can reach by static
// resolution, fn itself included: every function its body names.
func (p *purity) reachable(fn *types.Func) map[*types.Func]bool {
	out := map[*types.Func]bool{}
	var visit func(f *types.Func)
	visit = func(f *types.Func) {
		if out[f] {
			return
		}
		out[f] = true
		fd, ok := p.decls[f]
		if !ok {
			return
		}
		ast.Inspect(fd.decl.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if g, ok := fd.pkg.TypesInfo.Uses[x].(*types.Func); ok {
					visit(g)
				}
			case *ast.SelectorExpr:
				if sel, ok := fd.pkg.TypesInfo.Selections[x]; ok {
					if g, ok := sel.Obj().(*types.Func); ok {
						visit(g)
						if recv := g.Type().(*types.Signature).Recv(); recv != nil && types.IsInterface(recv.Type()) {
							for _, impl := range p.implementations(g) {
								visit(impl)
							}
						}
					}
				}
			}
			return true
		})
	}
	visit(fn)
	return out
}

// startingPoints are the declared functions of the packages accepted
// by keep, in a fixed order.
func (p *purity) startingPoints(keep func(path string) bool) []*types.Func {
	var out []*types.Func
	for fn, fd := range p.decls {
		if keep(fd.pkg.PkgPath) {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName() < out[j].FullName() })
	return out
}

// writesOutsideProducers walks fn with everything it was handed as
// tainted and returns the writes to memoized result types, nodes and
// tokens that fn may not make: it is neither reachable from the
// result's producer nor in the value's own package.
func (p *purity) writesOutsideProducers(fn *types.Func) []taintedWrite {
	var out []taintedWrite
	for _, w := range p.analyze(fn, true, p.allTainted(fn)).writes {
		allowed := false
		for _, i := range p.closures[w.typ] {
			if i < 0 {
				allowed = allowed || slices.Contains(valuePackages[w.typ.Pkg().Path()], fn.Pkg().Path())
			} else {
				allowed = allowed || p.reach[i][fn]
			}
		}
		if !allowed {
			out = append(out, w)
		}
	}
	return out
}

type funcDecl struct {
	pkg  *packages.Package
	decl *ast.FuncDecl
}

// summary is what a function does with a given taint of its receiver
// and parameters: which results carry a tainted value, and, in the
// immutability walk, the writes it makes through one.
type summary struct {
	results []bool
	writes  []taintedWrite
}

// taintedWrite is a write through a tainted value to an object of a
// named type, at a position.
type taintedWrite struct {
	typ *types.TypeName
	at  string
}

type purity struct {
	byPath    map[string]*packages.Package
	decls     map[*types.Func]funcDecl
	summaries map[string]*summary
	active    map[string]bool
	problems  []string

	// The immutability walk (D92): every value from outside a function
	// is tainted; writes to these types are recorded, and a target's
	// results are tainted the moment it returns them.
	shared   bool
	closures map[*types.TypeName][]int // type -> the targets whose results reach it; -1 for a value package's type
	targets  map[*types.Func]int
	reach    []map[*types.Func]bool // per target, the functions it can reach

	// impls are the module's implementations of each interface method
	// a walk meets (D93), found once.
	impls map[*types.Func][]*types.Func
	// stdlibUsed are the standard packages the walk called into (D97),
	// stdlibFuncsUsed the single trusted functions.
	stdlibUsed      map[string]bool
	stdlibFuncsUsed map[string]bool
}

// implementations are the methods of the module's named types that
// stand behind an interface method: what a call through the interface
// may run. Types outside the module are not walked; their results are
// judged by the receiver's taint instead.
func (p *purity) implementations(abstract *types.Func) []*types.Func {
	if p.impls == nil {
		p.impls = map[*types.Func][]*types.Func{}
	}
	if impls, done := p.impls[abstract]; done {
		return impls
	}
	var impls []*types.Func
	iface, _ := abstract.Type().(*types.Signature).Recv().Type().Underlying().(*types.Interface)
	if iface != nil {
		for _, pkg := range p.byPath {
			if !strings.HasPrefix(pkg.PkgPath, module) {
				continue
			}
			scope := pkg.Types.Scope()
			for _, name := range scope.Names() {
				tn, ok := scope.Lookup(name).(*types.TypeName)
				if !ok || tn.IsAlias() || types.IsInterface(tn.Type()) {
					continue
				}
				ptr := types.NewPointer(tn.Type())
				if !types.Implements(ptr, iface) {
					continue
				}
				if m, _, _ := types.LookupFieldOrMethod(ptr, true, abstract.Pkg(), abstract.Name()); m != nil {
					if fn, ok := m.(*types.Func); ok {
						if _, has := p.decls[fn]; has {
							impls = append(impls, fn)
						}
					}
				}
			}
		}
	}
	sort.Slice(impls, func(i, j int) bool { return impls[i].FullName() < impls[j].FullName() })
	p.impls[abstract] = impls
	return impls
}

func (p *purity) problem(pkg *packages.Package, pos ast.Node, format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf("%s: %s", pkg.Fset.Position(pos.Pos()), fmt.Sprintf(format, args...)))
}

// analyze checks fn under the given taint (receiver first when there
// is one, then the parameters in order) and returns its summary.
func (p *purity) analyze(fn *types.Func, recvTaint bool, paramTaint []bool) *summary {
	key := fn.FullName() + "/" + fmt.Sprint(recvTaint, paramTaint)
	if s := p.summaries[key]; s != nil {
		return s
	}
	if p.active[key] {
		return &summary{} // a recursive call: the results are judged by the outer analysis
	}
	fd, ok := p.decls[fn]
	if !ok {
		return &summary{}
	}
	p.active[key] = true
	defer delete(p.active, key)
	fa := &funcAnalysis{p: p, pkg: fd.pkg, info: fd.pkg.TypesInfo, taint: map[types.Object]bool{}}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() != nil && fd.decl.Recv != nil {
		for _, field := range fd.decl.Recv.List {
			for _, name := range field.Names {
				if obj := fd.pkg.TypesInfo.Defs[name]; obj != nil {
					fa.taint[obj] = recvTaint
				}
			}
		}
	}
	i := 0
	for _, field := range fd.decl.Type.Params.List {
		if len(field.Names) == 0 {
			i++
			continue
		}
		for _, name := range field.Names {
			if obj := fd.pkg.TypesInfo.Defs[name]; obj != nil && i < len(paramTaint) {
				fa.taint[obj] = paramTaint[i]
			}
			i++
		}
	}
	fa.results = make([]bool, sig.Results().Len())
	fa.stmts(fd.decl.Body.List)
	s := &summary{results: fa.results, writes: fa.writes}
	p.summaries[key] = s
	return s
}

// funcAnalysis walks one function body, tracking which locals carry a
// tainted value and judging every read, write and call.
type funcAnalysis struct {
	p       *purity
	pkg     *packages.Package
	info    *types.Info
	taint   map[types.Object]bool
	results []bool
	writes  []taintedWrite
}

func (fa *funcAnalysis) problem(node ast.Node, format string, args ...any) {
	fa.p.problems = append(fa.p.problems, fmt.Sprintf("%s: %s", fa.pkg.Fset.Position(node.Pos()), fmt.Sprintf(format, args...)))
}

func (fa *funcAnalysis) stmts(list []ast.Stmt) {
	for _, s := range list {
		fa.stmt(s)
	}
}

func (fa *funcAnalysis) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case nil:
	case *ast.ExprStmt:
		fa.expr(s.X)
	case *ast.AssignStmt:
		taints := fa.rhsTaints(s.Rhs, len(s.Lhs))
		for i, lhs := range s.Lhs {
			fa.write(lhs, taints[i], s.Tok.String() == ":=")
		}
	case *ast.IncDecStmt:
		fa.write(s.X, false, false)
	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					taints := fa.rhsTaints(vs.Values, len(vs.Names))
					for i, name := range vs.Names {
						if obj := fa.info.Defs[name]; obj != nil {
							fa.taint[obj] = len(vs.Values) > 0 && taints[i]
						}
					}
				}
			}
		}
	case *ast.ReturnStmt:
		if len(s.Results) == len(fa.results) {
			for i, r := range s.Results {
				if fa.expr(r) {
					fa.results[i] = true
				}
			}
		} else if len(s.Results) == 1 {
			// a call returning several values
			if fa.expr(s.Results[0]) {
				for i := range fa.results {
					fa.results[i] = true
				}
			}
		}
	case *ast.BlockStmt:
		fa.stmts(s.List)
	case *ast.IfStmt:
		fa.stmt(s.Init)
		fa.expr(s.Cond)
		fa.stmt(s.Body)
		fa.stmt(s.Else)
	case *ast.ForStmt:
		fa.stmt(s.Init)
		if s.Cond != nil {
			fa.expr(s.Cond)
		}
		fa.stmt(s.Post)
		fa.stmt(s.Body)
	case *ast.RangeStmt:
		tainted := fa.expr(s.X)
		for _, v := range []ast.Expr{s.Key, s.Value} {
			if id, ok := v.(*ast.Ident); ok && s.Tok.String() == ":=" {
				if obj := fa.info.Defs[id]; obj != nil {
					fa.taint[obj] = tainted
				}
			} else if v != nil {
				fa.write(v, tainted, false)
			}
		}
		fa.stmt(s.Body)
	case *ast.SwitchStmt:
		fa.stmt(s.Init)
		if s.Tag != nil {
			fa.expr(s.Tag)
		}
		fa.stmt(s.Body)
	case *ast.TypeSwitchStmt:
		fa.stmt(s.Init)
		var tainted bool
		var bound *ast.Ident
		switch a := s.Assign.(type) {
		case *ast.ExprStmt:
			tainted = fa.expr(a.X)
		case *ast.AssignStmt:
			tainted = fa.expr(a.Rhs[0])
			bound = a.Lhs[0].(*ast.Ident)
		}
		for _, c := range s.Body.List {
			cc := c.(*ast.CaseClause)
			if bound != nil {
				if obj := fa.info.Implicits[cc]; obj != nil {
					fa.taint[obj] = tainted
				}
			}
			fa.stmts(cc.Body)
		}
	case *ast.CaseClause:
		for _, e := range s.List {
			fa.expr(e)
		}
		fa.stmts(s.Body)
	case *ast.CommClause:
		fa.stmt(s.Comm)
		fa.stmts(s.Body)
	case *ast.BranchStmt, *ast.EmptyStmt:
	case *ast.LabeledStmt:
		fa.stmt(s.Stmt)
	case *ast.DeferStmt:
		fa.expr(s.Call)
	case *ast.GoStmt:
		if fa.p.shared {
			fa.expr(s.Call)
		} else {
			fa.problem(s, "a pure function starts no goroutine")
		}
	case *ast.SendStmt:
		if fa.p.shared {
			fa.expr(s.Chan)
			fa.expr(s.Value)
		} else {
			fa.problem(s, "a pure function sends on no channel")
		}
	case *ast.SelectStmt:
		if fa.p.shared {
			fa.stmt(s.Body)
		} else {
			fa.problem(s, "a pure function selects on no channel")
		}
	default:
		fa.problem(s, "purity gate: unhandled statement %T", s)
	}
}

// rhsTaints is the taint of each value on the right-hand side of an
// assignment of n variables: one expression per variable, or one call
// yielding them all.
func (fa *funcAnalysis) rhsTaints(rhs []ast.Expr, n int) []bool {
	out := make([]bool, n)
	if len(rhs) == n {
		for i, e := range rhs {
			out[i] = fa.expr(e)
		}
		return out
	}
	if len(rhs) == 1 {
		if call, ok := rhs[0].(*ast.CallExpr); ok {
			results := fa.call(call)
			for i := range out {
				if i < len(results) {
					out[i] = results[i]
				}
			}
			return out
		}
		t := fa.expr(rhs[0])
		for i := range out {
			out[i] = t
		}
	}
	return out
}

// write judges an assignment to lhs: a bare local is rebound (and
// carries the new taint); a write that reaches memory beyond the root
// variable's own value, through a pointer, a slice or a map, is a
// write to the object there, refused or recorded when it is reached
// through a tainted value.
func (fa *funcAnalysis) write(lhs ast.Expr, tainted bool, define bool) {
	if id, ok := lhs.(*ast.Ident); ok {
		if id.Name == "_" {
			return
		}
		var obj types.Object
		if define {
			obj = fa.info.Defs[id]
		} else {
			obj = fa.info.Uses[id]
		}
		if obj == nil {
			return
		}
		if v, ok := obj.(*types.Var); ok && v.Parent() == fa.pkg.Types.Scope() {
			if !fa.p.shared {
				fa.problem(lhs, "a pure function writes no package variable (%s)", id.Name)
			}
			return
		}
		fa.taint[obj] = tainted
		return
	}
	written, shared := fa.sharedTarget(lhs)
	if !shared {
		fa.expr(lhs) // a field of a value the variable holds itself
		return
	}
	viaTainted := false
	if root := rootIdent(lhs); root != nil {
		if obj := fa.info.Uses[root]; obj != nil {
			if v, ok := obj.(*types.Var); ok && v.Parent() == fa.pkg.Types.Scope() {
				if !fa.p.shared {
					fa.problem(lhs, "a pure function writes no package variable (%s)", root.Name)
					return
				}
				viaTainted = true
			} else {
				viaTainted = fa.taint[obj]
			}
		}
		fa.expr(lhs)
	} else {
		// The write reaches its object through an expression with no
		// identifier at its root, a call for instance: tainted if that
		// expression is.
		viaTainted = fa.expr(baseExpr(lhs))
	}
	if viaTainted {
		fa.taintedWrite(lhs, written)
	}
}

// taintedWrite judges a write through a tainted value to an object of
// the named type written (nil when unnamed): a purity walk refuses it,
// an immutability walk records it when the type is a memoized result's
// or a value package's.
func (fa *funcAnalysis) taintedWrite(node ast.Node, written *types.TypeName) {
	if !fa.p.shared {
		fa.problem(node, "a pure function writes through no input")
		return
	}
	if written != nil && fa.p.closures[written] != nil {
		fa.writes = append(fa.writes, taintedWrite{typ: written, at: fa.pkg.Fset.Position(node.Pos()).String()})
	}
}

// sharedTarget walks an assignment's left side from the written
// location toward its root and reports whether the write reaches
// memory beyond the root variable's own value: it does when the path
// passes a pointer, a slice or a map. The object written is the
// innermost struct behind such a step: a map of ints, or a slice of
// pointers that a store replaces one of, is its holder's.
func (fa *funcAnalysis) sharedTarget(lhs ast.Expr) (written *types.TypeName, shared bool) {
	step := func(t types.Type) {
		shared = true
		if written == nil {
			written = structOf(t)
		}
	}
	for {
		switch x := lhs.(type) {
		case *ast.ParenExpr:
			lhs = x.X
		case *ast.SelectorExpr:
			if t := fa.info.TypeOf(x.X); t != nil {
				if ptr, ok := t.Underlying().(*types.Pointer); ok {
					step(ptr.Elem())
				}
			}
			lhs = x.X
		case *ast.IndexExpr:
			if t := fa.info.TypeOf(x.X); t != nil {
				switch u := t.Underlying().(type) {
				case *types.Slice:
					step(u.Elem())
				case *types.Map:
					step(u.Elem())
				case *types.Pointer:
					step(u.Elem())
				}
			}
			lhs = x.X
		case *ast.StarExpr:
			if t := fa.info.TypeOf(x.X); t != nil {
				if ptr, ok := t.Underlying().(*types.Pointer); ok {
					step(ptr.Elem())
				} else {
					shared = true
				}
			}
			lhs = x.X
		default:
			return written, shared
		}
	}
}

// structOf is the named struct type t is; nil otherwise. A store into
// a container of anything else (pointers, slices, maps, basics)
// replaces a value in the container and writes nothing behind it,
// so the object written is the container's holder.
func structOf(t types.Type) *types.TypeName {
	if x, ok := types.Unalias(t).(*types.Named); ok {
		if _, ok := x.Underlying().(*types.Struct); ok {
			return x.Obj()
		}
	}
	return nil
}

// elemOf is what a slice, an array or a map holds; t itself otherwise.
func elemOf(t types.Type) types.Type {
	switch x := types.Unalias(t).Underlying().(type) {
	case *types.Slice:
		return x.Elem()
	case *types.Array:
		return x.Elem()
	case *types.Map:
		return x.Elem()
	}
	return t
}

// mutated judges a call that writes the container it is handed as an
// argument: the object written is the container's elements when they
// are structs, its holder otherwise.
func (fa *funcAnalysis) mutated(arg ast.Expr) {
	if !fa.expr(arg) {
		return
	}
	written := structOf(elemOf(fa.info.TypeOf(arg)))
	if written == nil {
		written, _ = fa.sharedTarget(arg)
	}
	fa.taintedWrite(arg, written)
}

// baseExpr is the expression a write reaches its object through, with
// the selectors, indexes and dereferences peeled.
func baseExpr(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		default:
			return e
		}
	}
}

// rootIdent is the identifier an expression reaches its object
// through: x in x.f, x[i], *x, x.f[i].g.
func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// expr judges an expression and reports whether its value carries a
// tainted value.
func (fa *funcAnalysis) expr(e ast.Expr) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.Ident:
		obj := fa.info.Uses[x]
		if obj == nil {
			return false
		}
		if v, ok := obj.(*types.Var); ok {
			if v.Parent() == fa.pkg.Types.Scope() || (v.Pkg() != nil && v.Pkg().Scope().Lookup(v.Name()) == v) {
				if immutableVars[v.Pkg().Path()+"."+v.Name()] {
					return false
				}
				if fa.p.shared {
					return true // package state: shared with everyone
				}
				fa.problem(x, "a pure function reads no package variable (%s.%s)", v.Pkg().Path(), v.Name())
				return false
			}
			return fa.taint[obj]
		}
		return false
	case *ast.SelectorExpr:
		if _, ok := fa.info.Selections[x]; ok {
			return fa.expr(x.X)
		}
		// a package-qualified name
		return fa.expr(x.Sel)
	case *ast.IndexExpr:
		fa.expr(x.Index)
		return fa.expr(x.X)
	case *ast.IndexListExpr:
		return fa.expr(x.X)
	case *ast.SliceExpr:
		for _, i := range []ast.Expr{x.Low, x.High, x.Max} {
			if i != nil {
				fa.expr(i)
			}
		}
		return fa.expr(x.X)
	case *ast.StarExpr:
		return fa.expr(x.X)
	case *ast.ParenExpr:
		return fa.expr(x.X)
	case *ast.TypeAssertExpr:
		return fa.expr(x.X)
	case *ast.UnaryExpr:
		if x.Op.String() == "<-" && !fa.p.shared {
			fa.problem(x, "a pure function receives on no channel")
		}
		return fa.expr(x.X)
	case *ast.BinaryExpr:
		fa.expr(x.X)
		fa.expr(x.Y)
		return false
	case *ast.BasicLit:
		return false
	case *ast.CompositeLit:
		for _, elt := range x.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				fa.expr(kv.Value)
			} else {
				fa.expr(elt)
			}
		}
		return false // a value the function created
	case *ast.KeyValueExpr:
		fa.expr(x.Key)
		return fa.expr(x.Value)
	case *ast.FuncLit:
		// A closure runs in this scope; its parameters may be handed inputs.
		for _, field := range x.Type.Params.List {
			for _, name := range field.Names {
				if obj := fa.info.Defs[name]; obj != nil {
					fa.taint[obj] = true
				}
			}
		}
		saved := fa.results
		fa.results = make([]bool, x.Type.Results.NumFields())
		fa.stmts(x.Body.List)
		fa.results = saved
		return false
	case *ast.CallExpr:
		results := fa.call(x)
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	case *ast.ArrayType, *ast.MapType, *ast.StructType, *ast.InterfaceType, *ast.FuncType, *ast.ChanType, *ast.Ellipsis:
		return false
	default:
		fa.problem(e, "purity gate: unhandled expression %T", e)
		return false
	}
}

// isStringToSlice reports whether a conversion from the type from to
// the type to is one of the two the language specifies as copying:
// string to []byte and string to []rune.
func isStringToSlice(to, from types.Type) bool {
	if from == nil {
		return false
	}
	if b, ok := from.Underlying().(*types.Basic); !ok || b.Info()&types.IsString == 0 {
		return false
	}
	sl, ok := to.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	e, ok := sl.Elem().Underlying().(*types.Basic)
	return ok && (e.Kind() == types.Byte || e.Kind() == types.Rune || e.Kind() == types.Uint8 || e.Kind() == types.Int32)
}

// call judges a call and reports the taint of its results.
func (fa *funcAnalysis) call(x *ast.CallExpr) []bool {
	argTaint := make([]bool, len(x.Args))
	for i, a := range x.Args {
		argTaint[i] = fa.expr(a)
	}
	anyArg := false
	for _, t := range argTaint {
		anyArg = anyArg || t
	}
	// A conversion keeps what it converts, except one from a string
	// to a slice of bytes or runes, which the language defines as a
	// copy: the result is fresh memory nobody else holds.
	if tv, ok := fa.info.Types[x.Fun]; ok && tv.IsType() {
		if len(x.Args) == 1 && isStringToSlice(tv.Type, fa.info.Types[x.Args[0]].Type) {
			return []bool{false}
		}
		return []bool{anyArg}
	}
	var fn *types.Func
	var recvTaint bool
	switch f := ast.Unparen(x.Fun).(type) {
	case *ast.Ident:
		switch obj := fa.info.Uses[f].(type) {
		case *types.Builtin:
			return fa.builtin(obj.Name(), x, argTaint)
		case *types.Func:
			fn = obj
		case *types.Var:
			// a function value: a closure analyzed where it was made, or an input
			return []bool{anyArg || fa.taint[obj]}
		default:
			return []bool{anyArg}
		}
	case *ast.SelectorExpr:
		if sel, ok := fa.info.Selections[f]; ok {
			recvTaint = fa.expr(f.X)
			if m, ok := sel.Obj().(*types.Func); ok {
				fn = m
			} else {
				// a method value or a func-typed field
				return []bool{recvTaint || anyArg}
			}
		} else if obj, ok := fa.info.Uses[f.Sel].(*types.Func); ok {
			fn = obj // pkg.Func
		} else {
			return []bool{anyArg}
		}
	case *ast.FuncLit:
		fa.expr(f)
		return []bool{anyArg}
	default:
		fa.expr(x.Fun)
		return []bool{anyArg}
	}
	sig := fn.Type().(*types.Signature)
	n := sig.Results().Len()
	if recv := sig.Recv(); recv != nil && types.IsInterface(recv.Type()) {
		return fa.dispatch(fn, sig, recvTaint, argTaint)
	}
	if fn.Pkg() == nil {
		return make([]bool, n) // universe
	}
	path := fn.Pkg().Path()
	if !strings.HasPrefix(path, module) {
		if fa.p.stdlibUsed == nil {
			fa.p.stdlibUsed = map[string]bool{}
			fa.p.stdlibFuncsUsed = map[string]bool{}
		}
		fa.p.stdlibUsed[path] = true
		if idx, ok := stdlibMutators[fn.FullName()]; ok && idx < len(x.Args) {
			fa.mutated(x.Args[idx])
		} else if pureStdlibFuncs[fn.FullName()] {
			fa.p.stdlibFuncsUsed[fn.FullName()] = true
		} else if !pureStdlib[path] && !fa.p.shared {
			fa.problem(x, "a pure function calls into no package that keeps state (%s)", fn.FullName())
		}
		return make([]bool, n)
	}
	if _, has := fa.p.decls[fn]; !has {
		return make([]bool, n) // declared without a body: assembly, or generated
	}
	params := paramTaints(sig, argTaint)
	s := fa.p.analyze(fn, recvTaint, params)
	fa.writes = append(fa.writes, s.writes...)
	if _, memoized := fa.p.targets[fn]; memoized && fa.p.shared {
		// A memoized computation's results are its memo's the moment
		// they return: nobody writes them.
		out := make([]bool, n)
		for i := range out {
			out[i] = true
		}
		return out
	}
	return s.results
}

// paramTaints spreads the arguments' taint over a signature's
// parameters, the variadic one gathering its arguments'.
func paramTaints(sig *types.Signature, argTaint []bool) []bool {
	params := make([]bool, sig.Params().Len())
	for i := range params {
		if sig.Variadic() && i == len(params)-1 {
			for j := i; j < len(argTaint); j++ {
				params[i] = params[i] || argTaint[j]
			}
		} else if i < len(argTaint) {
			params[i] = argTaint[i]
		}
	}
	return params
}

// dispatch judges a call through an interface (D93): every method of
// the module's types that implements it is walked with the receiver's
// and the arguments' taint, and the results carry a tainted value
// when any implementation's do. An implementation outside the module
// is not seen; its results are judged by the receiver alone, which is
// what such a call reads.
func (fa *funcAnalysis) dispatch(abstract *types.Func, sig *types.Signature, recvTaint bool, argTaint []bool) []bool {
	out := make([]bool, sig.Results().Len())
	for i := range out {
		out[i] = recvTaint
	}
	params := paramTaints(sig, argTaint)
	for _, impl := range fa.p.implementations(abstract) {
		s := fa.p.analyze(impl, recvTaint, params)
		fa.writes = append(fa.writes, s.writes...)
		for i := range out {
			if i < len(s.results) && s.results[i] {
				out[i] = true
			}
		}
	}
	return out
}

func (fa *funcAnalysis) builtin(name string, x *ast.CallExpr, argTaint []bool) []bool {
	switch name {
	case "append":
		t := false
		for _, a := range argTaint {
			t = t || a
		}
		return []bool{t}
	case "copy", "delete", "clear":
		if len(x.Args) > 0 {
			fa.mutated(x.Args[0])
		}
		return []bool{false}
	case "len", "cap", "make", "new", "min", "max", "panic", "print", "println", "recover", "complex", "real", "imag":
		return []bool{false}
	}
	return []bool{false}
}

// immutableVarsNeverAssigned proves the allowlisted tables are written
// only by their declarations.
func (p *purity) immutableVarsNeverAssigned() {
	for _, pkg := range p.byPath {
		if !strings.HasPrefix(pkg.PkgPath, module) {
			continue
		}
		for _, f := range pkg.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				var lhs []ast.Expr
				switch s := n.(type) {
				case *ast.AssignStmt:
					lhs = s.Lhs
				case *ast.IncDecStmt:
					lhs = []ast.Expr{s.X}
				default:
					return true
				}
				for _, l := range lhs {
					root := rootIdent(l)
					if root == nil {
						continue
					}
					if v, ok := pkg.TypesInfo.Uses[root].(*types.Var); ok && v.Pkg() != nil && immutableVars[v.Pkg().Path()+"."+v.Name()] {
						p.problems = append(p.problems, fmt.Sprintf("%s: immutable table %s is assigned", pkg.Fset.Position(l.Pos()), v.Name()))
					}
				}
				return true
			})
		}
	}
}

// TestPurityGateBites proves the gate refuses each kind of impurity
// on a fixture package, and passes the pure function in it, so a
// green gate on the real targets means something.
func TestPurityGateBites(t *testing.T) {
	p := newPurity(t, gatePackages(t), false)
	pkg := p.byPath[fixtureImpure]
	if pkg == nil {
		t.Fatal("fixture not loaded")
	}
	for name, want := range map[string]string{
		"ReadsGlobal":         "reads no package variable",
		"WritesInput":         "writes through no input",
		"WritesInputByCallee": "writes through no input",
		"CallsOS":             "calls into no package that keeps state",
		"WritesResultOfInput": "writes through no input",
		"SortsInput":          "writes through no input",
		"CallsInterface":      "writes no package variable",
		"Pure":                "",
	} {
		p.problems = nil
		fn := pkg.Types.Scope().Lookup(name).(*types.Func)
		p.analyze(fn, false, p.allTainted(fn))
		got := strings.Join(p.problems, "\n")
		switch {
		case want == "" && got != "":
			t.Errorf("%s: pure, yet refused:\n%s", name, got)
		case want != "" && !strings.Contains(got, want):
			t.Errorf("%s: want a refusal saying %q, got:\n%s", name, want, got)
		}
	}
}

// TestImmutabilityGateBites proves the immutability walk refuses each
// way of writing a memoized result on a fixture package, and passes
// the functions that only read it, make their own, or write a copy.
func TestImmutabilityGateBites(t *testing.T) {
	p := newPurity(t, gatePackages(t), true)
	pkg := p.byPath[fixtureMutable]
	if pkg == nil {
		t.Fatal("fixture not loaded")
	}
	p.producersSet([]*types.Func{pkg.Types.Scope().Lookup("Produce").(*types.Func)})
	for name, want := range map[string]bool{
		"Tamper": true, "TamperByCallee": true, "touch": true, "Sorts": true, "Rebuilds": true,
		"Produce": false, "Fresh": false, "Copies": false, "Reads": false,
	} {
		fn := pkg.Types.Scope().Lookup(name).(*types.Func)
		if got := len(p.writesOutsideProducers(fn)) > 0; got != want {
			t.Errorf("%s: refused=%v, want %v", name, got, want)
		}
	}
}

// TestNoUnsafeImports proves no package of the three modules reaches
// for unsafe (D89): what the compiler proves about a Go program holds
// only while nothing steps outside the type system.
func TestNoUnsafeImports(t *testing.T) {
	pkgs := gatePackages(t)
	if len(pkgs) < 20 {
		t.Fatalf("only %d packages loaded", len(pkgs))
	}
	for _, pkg := range pkgs {
		if pkg.Imports["unsafe"] != nil {
			t.Errorf("%s imports unsafe", pkg.PkgPath)
		}
	}
}
