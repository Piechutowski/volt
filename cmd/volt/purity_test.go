package main

// The purity gate (D86): every computation a session memo answers must
// be a function of its explicit inputs, so that "same inputs, same
// answer" is a property of the code and not of an enumeration. This
// test loads the library with type information and checks each
// memoized function, and transitively everything it calls, for the
// three things purity forbids: reading mutable package state, writing
// through an input, and calling into a package that keeps state. What
// it allows is what a pure function may do: read its inputs, build and
// mutate values it created, return them, and call other pure code.
// The check is conservative: a value that comes out of a call whose
// inputs were tainted is tainted, so a write to it is refused, and a
// closure's parameters are assumed to carry inputs.

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
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
	{module + "/lsp", "tableOccurrences"},
}

// immutableVars are package-level variables a pure function may read:
// lookup tables written once at initialization. The gate also proves
// none of them is assigned anywhere but its declaration.
var immutableVars = map[string]bool{
	module + "/lang.crudMethodSuffixes":          true,
	module + "/lang.compareSQL":                  true,
	module + "/lang.goKeywords":                  true,
	module + "/lang.reservedParamNames":          true,
	module + "/lang/check.tableSettings":         true,
	module + "/lang/check.columnSettings":        true,
	module + "/lang/check.indexSettings":         true,
	module + "/lang/check.settingsCheck":         true,
	module + "/lang/check.refSettings":           true,
	module + "/lang/check.integerTypes":          true,
	module + "/lang/check.requiredKinds":         true,
	module + "/nao/gen/golang.commonInitialisms": true,
	module + "/nao/gen/golang.typeMap":           true,
	module + "/nao/gen/golang.goKeywords":        true,
	module + "/nao/gen/sqlite.sqliteKeywords":    true,
	module + "/nao/gen/sqlite.plainIdent":        true,
	module + "/nao/inflect.irregular":            true,
}

// pureStdlib are the standard packages whose functions keep no state.
var pureStdlib = map[string]bool{
	"strings": true, "strconv": true, "fmt": true, "sort": true, "slices": true, "maps": true,
	"errors": true, "unicode": true, "unicode/utf8": true, "bytes": true, "cmp": true,
	"regexp": true, // a compiled expression answers the same question every time
}

func TestMemoizedComputationsArePure(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir: root,
	}
	pkgs, err := packages.Load(cfg, module+"/...", module+"/lsp/...")
	if err != nil {
		t.Fatal(err)
	}
	p := &purity{byPath: map[string]*packages.Package{}, decls: map[*types.Func]funcDecl{}, summaries: map[string]*summary{}, active: map[string]bool{}}
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
	for _, target := range pureTargets {
		pkg := p.byPath[target.pkg]
		if pkg == nil {
			t.Fatalf("package %s not loaded", target.pkg)
		}
		obj, ok := pkg.Types.Scope().Lookup(target.name).(*types.Func)
		if !ok {
			t.Fatalf("%s.%s: no such function", target.pkg, target.name)
		}
		if obj.Type().(*types.Signature).Recv() != nil {
			t.Errorf("%s.%s: a memoized computation is a top-level function, not a method", target.pkg, target.name)
			continue
		}
		fd := p.decls[obj]
		n := fd.decl.Type.Params.NumFields()
		taint := make([]bool, 0, n)
		for _, field := range fd.decl.Type.Params.List {
			for range field.Names {
				taint = append(taint, true)
			}
			if len(field.Names) == 0 {
				taint = append(taint, true)
			}
		}
		p.analyze(obj, false, taint)
	}
	p.immutableVarsNeverAssigned()
	sort.Strings(p.problems)
	for _, problem := range p.problems {
		t.Error(problem)
	}
}

type funcDecl struct {
	pkg  *packages.Package
	decl *ast.FuncDecl
}

// summary is what a function does with a given taint of its receiver
// and parameters: which results carry an input.
type summary struct {
	results []bool
}

type purity struct {
	byPath    map[string]*packages.Package
	decls     map[*types.Func]funcDecl
	summaries map[string]*summary
	active    map[string]bool
	problems  []string
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
	s := &summary{results: fa.results}
	p.summaries[key] = s
	return s
}

// funcAnalysis walks one function body, tracking which locals carry an
// input (taint) and judging every read, write and call.
type funcAnalysis struct {
	p       *purity
	pkg     *packages.Package
	info    *types.Info
	taint   map[types.Object]bool
	results []bool
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
	case *ast.BranchStmt, *ast.EmptyStmt:
	case *ast.LabeledStmt:
		fa.stmt(s.Stmt)
	case *ast.DeferStmt:
		fa.expr(s.Call)
	case *ast.GoStmt:
		fa.problem(s, "a pure function starts no goroutine")
	case *ast.SendStmt:
		fa.problem(s, "a pure function sends on no channel")
	case *ast.SelectStmt:
		fa.problem(s, "a pure function selects on no channel")
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
// carries the new taint); anything reached through a tainted root is
// an input, and an input is never written.
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
			fa.problem(lhs, "a pure function writes no package variable (%s)", id.Name)
			return
		}
		fa.taint[obj] = tainted
		return
	}
	if root := rootIdent(lhs); root != nil {
		if obj := fa.info.Uses[root]; obj != nil {
			if v, ok := obj.(*types.Var); ok && v.Parent() == fa.pkg.Types.Scope() {
				fa.problem(lhs, "a pure function writes no package variable (%s)", root.Name)
				return
			}
			if fa.taint[obj] {
				fa.problem(lhs, "a pure function writes through no input (%s)", root.Name)
			}
		}
		fa.expr(lhs)
		return
	}
	// The write reaches its object through an expression with no
	// identifier at its root, a call for instance: tainted if that
	// expression is.
	if fa.expr(baseExpr(lhs)) {
		fa.problem(lhs, "a pure function writes through no input (a value derived from one)")
	}
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

// expr judges an expression and reports whether its value carries an
// input.
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
				if !immutableVars[v.Pkg().Path()+"."+v.Name()] {
					fa.problem(x, "a pure function reads no package variable (%s.%s)", v.Pkg().Path(), v.Name())
				}
				return false
			}
			return fa.taint[obj]
		}
		return false
	case *ast.SelectorExpr:
		if sel, ok := fa.info.Selections[x]; ok {
			_ = sel
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
		if x.Op.String() == "<-" {
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
	// A conversion keeps what it converts.
	if tv, ok := fa.info.Types[x.Fun]; ok && tv.IsType() {
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
	if fn.Pkg() == nil {
		return make([]bool, n) // universe: error.Error
	}
	path := fn.Pkg().Path()
	if !strings.HasPrefix(path, module) {
		if !pureStdlib[path] {
			fa.problem(x, "a pure function calls into no package that keeps state (%s)", fn.FullName())
		}
		return make([]bool, n)
	}
	if _, has := fa.p.decls[fn]; !has {
		// An interface method: a read of the value it is called on,
		// answered by whatever implements it. Only the module's own
		// interfaces are data (ast, token, an oracle passed as input).
		if recv := sig.Recv(); recv != nil && types.IsInterface(recv.Type()) {
			out := make([]bool, n)
			for i := range out {
				out[i] = recvTaint
			}
			return out
		}
		return make([]bool, n)
	}
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
	return fa.p.analyze(fn, recvTaint, params).results
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
		if len(x.Args) > 0 && argTaint[0] {
			fa.problem(x, "a pure function writes through no input (%s)", name)
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
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir: root,
	}
	const fixture = module + "/cmd/volt/testdata/impure"
	pkgs, err := packages.Load(cfg, fixture)
	if err != nil {
		t.Fatal(err)
	}
	p := &purity{byPath: map[string]*packages.Package{}, decls: map[*types.Func]funcDecl{}, summaries: map[string]*summary{}, active: map[string]bool{}}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
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
	pkg := p.byPath[fixture]
	if pkg == nil {
		t.Fatal("fixture not loaded")
	}
	for name, want := range map[string]string{
		"ReadsGlobal":         "reads no package variable",
		"WritesInput":         "writes through no input",
		"WritesInputByCallee": "writes through no input",
		"CallsOS":             "calls into no package that keeps state",
		"WritesResultOfInput": "writes through no input",
		"Pure":                "",
	} {
		p.problems = nil
		fn := pkg.Types.Scope().Lookup(name).(*types.Func)
		n := fn.Type().(*types.Signature).Params().Len()
		taint := make([]bool, n)
		for i := range taint {
			taint[i] = true
		}
		p.analyze(fn, false, taint)
		got := strings.Join(p.problems, "\n")
		switch {
		case want == "" && got != "":
			t.Errorf("%s: pure, yet refused:\n%s", name, got)
		case want != "" && !strings.Contains(got, want):
			t.Errorf("%s: want a refusal saying %q, got:\n%s", name, want, got)
		}
	}
}
