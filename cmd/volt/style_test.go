package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	styleFunctionLinesMax = 70
	styleLineColumnsMax   = 100
	styleTabColumns       = 4
	styleExprDepthMax     = 64
	styleLedgerPath       = "testdata/style.ledger"
	styleDocPath          = "../../docs/style.md"
)

const (
	ruleFunctionLength   = "function-length"
	ruleLineLength       = "line-length"
	ruleRecursion        = "recursion"
	ruleArgumentMutation = "argument-mutation"
	rulePackageState     = "package-state"
	ruleErrorDiscarded   = "error-discarded"
	ruleLoopUnbounded    = "loop-unbounded"
	ruleFileName         = "file-name"
	ruleDependency       = "dependency"
)

var styleLimits = map[string]int{
	ruleFunctionLength:   styleFunctionLinesMax,
	ruleLineLength:       styleLineColumnsMax,
	ruleRecursion:        0,
	ruleArgumentMutation: 0,
	rulePackageState:     0,
	ruleErrorDiscarded:   0,
	ruleLoopUnbounded:    0,
	ruleFileName:         0,
	ruleDependency:       0,
}

var styleExampleRules = []string{
	ruleRecursion,
	ruleArgumentMutation,
	rulePackageState,
	ruleErrorDiscarded,
	ruleLoopUnbounded,
}

var styleDependencies = map[string][]string{
	"go.mod":     {"github.com/mattn/go-sqlite3"},
	"lsp/go.mod": {"github.com/Piechutowski/volt"},
	"cmd/volt/go.mod": {
		"github.com/Piechutowski/volt",
		"github.com/Piechutowski/volt/lsp",
		"github.com/mattn/go-sqlite3",
		"golang.org/x/mod",
		"golang.org/x/sync",
		"golang.org/x/tools",
	},
}

var styleInventory = map[string]bool{
	module + "/nao/run.readers":    true,
	module + "/nao/run.StmtCache":  true,
	module + "/nao/run.cacheEntry": true,
	module + ".Broker":             true,
}

var styleSkipDirs = map[string]bool{
	".git":          true,
	"grammar":       true,
	"node_modules":  true,
	"zed-extension": true,
}

var styleFileNamePattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*\.go(\.golden)?$`)

var styleRulePattern = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

var styleBuiltinMutators = map[string]bool{
	"append": true,
	"clear":  true,
	"copy":   true,
	"delete": true,
}

var styleNeverFails = map[string]bool{
	"(*bytes.Buffer).Write":          true,
	"(*bytes.Buffer).WriteByte":      true,
	"(*bytes.Buffer).WriteRune":      true,
	"(*bytes.Buffer).WriteString":    true,
	"(*strings.Builder).Write":       true,
	"(*strings.Builder).WriteByte":   true,
	"(*strings.Builder).WriteRune":   true,
	"(*strings.Builder).WriteString": true,
}

var styleNeverFailWriters = map[string]bool{
	"*bytes.Buffer":    true,
	"*strings.Builder": true,
}

var styleFprints = map[string]bool{
	"fmt.Fprint":   true,
	"fmt.Fprintf":  true,
	"fmt.Fprintln": true,
}

type styleFinding struct {
	rule  string
	key   string
	value int
}

type styleTyped struct {
	name string
	file *ast.File
	info *types.Info
	pkg  *types.Package
	keys map[*ast.FuncDecl]string
}

type styleWrite struct {
	root    *types.Var
	shared  bool
	through bool
}

type styleGraph struct {
	edges    map[*types.Func][]*types.Func
	keys     map[*types.Func]string
	closures map[*types.Func]bool
}

type styleExample struct {
	rule string
	bad  bool
	src  string
	line int
}

func TestStyleLedger(t *testing.T) {
	got := styleFold(styleFindingsAll(t))
	want, exists := styleLedgerRead(t)
	if *update && !exists {
		styleLedgerWrite(t, got)
		return
	}
	grown, shrunk := styleLedgerDiff(got, want)
	for _, entry := range grown {
		t.Errorf("style debt grew, bring the code back to docs/style.md: %s", entry)
	}
	if *update && len(grown) == 0 {
		styleLedgerWrite(t, got)
		return
	}
	for _, entry := range shrunk {
		t.Errorf("style debt shrank, lower the ledger with "+
			"go test ./cmd/volt -run TestStyleLedger -update: %s", entry)
	}
}

func TestStyleDocumentsTheGate(t *testing.T) {
	src, err := os.ReadFile(styleDocPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(src)
	styleDocLimitsCheck(t, doc)
	styleDocDependenciesCheck(t, doc)
	styleDocExamplesCheck(t, doc)
}

func styleFindingsAll(t *testing.T) []styleFinding {
	t.Helper()
	root := styleRoot(t)
	var findings []styleFinding
	for _, path := range styleSourcePaths(t, root) {
		findings = append(findings, styleSyntaxFindings(t, root, path)...)
	}
	typed := styleTypedFiles(t, root, gatePackages(t))
	for _, file := range typed {
		findings = append(findings, styleTypedFindings(file)...)
	}
	findings = append(findings, styleRecursive(styleGraphBuild(typed))...)
	return append(findings, styleDependencyFindings(t, root)...)
}

func styleFold(findings []styleFinding) map[string]int {
	folded := map[string]int{}
	for _, finding := range findings {
		folded[finding.rule+" "+finding.key] += finding.value
	}
	return folded
}

func styleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func styleRel(t *testing.T, root string, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func styleSourcePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && styleSkipDirs[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".go.golden") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func styleSyntaxFindings(t *testing.T, root string, path string) []styleFinding {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name := styleRel(t, root, path)
	findings := styleLineFindings(name, src)
	if !styleFileNamePattern.MatchString(filepath.Base(name)) {
		findings = append(findings, styleFinding{ruleFileName, name, 1})
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return append(findings, styleDeclFindings(fset, name, file)...)
}

func styleLineFindings(name string, src []byte) []styleFinding {
	count := 0
	for _, line := range strings.Split(string(src), "\n") {
		columns := utf8.RuneCountInString(line) + strings.Count(line, "\t")*(styleTabColumns-1)
		if columns > styleLineColumnsMax {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	return []styleFinding{{ruleLineLength, name, count}}
}

func styleFuncKeys(name string, file *ast.File) map[*ast.FuncDecl]string {
	keys := map[*ast.FuncDecl]string{}
	seen := map[string]int{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		key := name + ":" + styleFuncName(fn)
		seen[key]++
		if seen[key] > 1 {
			key += "#" + strconv.Itoa(seen[key])
		}
		keys[fn] = key
	}
	return keys
}

func styleFuncName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	star := ""
	if pointer, ok := recv.(*ast.StarExpr); ok {
		star, recv = "*", pointer.X
	}
	switch generic := recv.(type) {
	case *ast.IndexExpr:
		recv = generic.X
	case *ast.IndexListExpr:
		recv = generic.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return "(" + star + id.Name + ")." + fn.Name.Name
	}
	return "(" + star + "?)." + fn.Name.Name
}

func styleDeclFindings(fset *token.FileSet, name string, file *ast.File) []styleFinding {
	keys := styleFuncKeys(name, file)
	var findings []styleFinding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lines := fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
		if lines > styleFunctionLinesMax {
			findings = append(findings, styleFinding{ruleFunctionLength, keys[fn], lines})
		}
		if count := styleLoopsUnbounded(fn.Body); count > 0 {
			findings = append(findings, styleFinding{ruleLoopUnbounded, keys[fn], count})
		}
	}
	return findings
}

func styleLoopsUnbounded(body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if loop, ok := node.(*ast.ForStmt); ok && !styleLoopBounded(loop) {
			count++
		}
		return true
	})
	return count
}

func styleLoopBounded(loop *ast.ForStmt) bool {
	if loop.Cond == nil {
		return styleLoopEndless(loop.Body)
	}
	counter := styleLoopCounter(loop.Post)
	return counter != "" && styleCondBounds(loop.Cond, counter)
}

func styleLoopCounter(post ast.Stmt) string {
	var target ast.Expr
	switch x := post.(type) {
	case *ast.IncDecStmt:
		target = x.X
	case *ast.AssignStmt:
		if len(x.Lhs) == 1 && (x.Tok == token.ADD_ASSIGN || x.Tok == token.SUB_ASSIGN) {
			target = x.Lhs[0]
		}
	}
	if id, ok := ast.Unparen(target).(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func styleLoopEndless(body *ast.BlockStmt) bool {
	exits := 0
	var stack []ast.Node
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if styleLoopExit(node, stack) {
			exits++
		}
		stack = append(stack, node)
		return true
	})
	return exits == 0
}

func styleLoopExit(node ast.Node, stack []ast.Node) bool {
	switch x := node.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		if x.Tok == token.GOTO || x.Label != nil {
			return true
		}
		if x.Tok != token.BREAK {
			return false
		}
		for _, outer := range stack {
			switch outer.(type) {
			case *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt:
				return false
			case *ast.SwitchStmt, *ast.TypeSwitchStmt:
				return false
			}
		}
		return true
	}
	return false
}

func styleCondBounds(cond ast.Expr, counter string) bool {
	pending := []ast.Expr{cond}
	for index := 0; index < len(pending); index++ {
		binary, ok := ast.Unparen(pending[index]).(*ast.BinaryExpr)
		if !ok {
			continue
		}
		switch binary.Op {
		case token.LAND:
			pending = append(pending, binary.X, binary.Y)
		case token.LSS, token.LEQ, token.GTR, token.GEQ:
			if styleIsIdent(binary.X, counter) || styleIsIdent(binary.Y, counter) {
				return true
			}
		}
	}
	return false
}

func styleIsIdent(expr ast.Expr, name string) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && id.Name == name
}

func styleTypedScope(path string) bool {
	if !strings.HasPrefix(path, module) || strings.Contains(path, "/testdata/") {
		return false
	}
	return !strings.HasPrefix(path, module+"/lang/conformance/snippets/")
}

func styleTypedFiles(t *testing.T, root string, pkgs []*packages.Package) []styleTyped {
	t.Helper()
	var files []styleTyped
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		if !styleTypedScope(pkg.PkgPath) {
			return
		}
		for _, file := range pkg.Syntax {
			name := styleRel(t, root, pkg.Fset.File(file.Pos()).Name())
			keys := styleFuncKeys(name, file)
			files = append(files, styleTyped{name, file, pkg.TypesInfo, pkg.Types, keys})
		}
	})
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files
}

func styleTypedFindings(file styleTyped) []styleFinding {
	var findings []styleFinding
	for _, decl := range file.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		key := file.keys[fn]
		writes := styleWrites(file.info, fn.Body)
		if count := styleArgumentWrites(file.info, fn, writes); count > 0 {
			findings = append(findings, styleFinding{ruleArgumentMutation, key, count})
		}
		if count := styleErrorsDiscarded(file.info, fn.Body); count > 0 {
			findings = append(findings, styleFinding{ruleErrorDiscarded, key, count})
		}
		if count := styleChannelLoops(file.info, fn.Body); count > 0 {
			findings = append(findings, styleFinding{ruleLoopUnbounded, key, count})
		}
		if fn.Recv != nil || fn.Name.Name != "init" {
			findings = append(findings, stylePackageWrites(writes)...)
		}
	}
	return findings
}

func styleWrites(info *types.Info, body *ast.BlockStmt) []styleWrite {
	var writes []styleWrite
	ast.Inspect(body, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				writes = append(writes, styleExprWrites(info, x.Lhs)...)
			}
		case *ast.IncDecStmt:
			writes = append(writes, styleExprWrites(info, []ast.Expr{x.X})...)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				writes = append(writes, styleThrough(info, x.X)...)
			}
		case *ast.CallExpr:
			writes = append(writes, styleCallWrites(info, x)...)
		}
		return true
	})
	return writes
}

func styleExprWrites(info *types.Info, exprs []ast.Expr) []styleWrite {
	var writes []styleWrite
	for _, expr := range exprs {
		if root, shared := styleWriteRoot(info, expr); root != nil {
			writes = append(writes, styleWrite{root, shared, false})
		}
	}
	return writes
}

func styleThrough(info *types.Info, expr ast.Expr) []styleWrite {
	root, _ := styleWriteRoot(info, expr)
	if root == nil {
		return nil
	}
	return []styleWrite{{root, true, true}}
}

func styleCallWrites(info *types.Info, call *ast.CallExpr) []styleWrite {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		builtin, ok := info.Uses[id].(*types.Builtin)
		if ok && styleBuiltinMutators[builtin.Name()] && len(call.Args) > 0 {
			return styleShared(info, call.Args[0])
		}
	}
	if callee := typeutil.StaticCallee(info, call); callee != nil {
		index, ok := stdlibMutators[callee.Origin().FullName()]
		if ok && index < len(call.Args) {
			return styleShared(info, call.Args[index])
		}
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection := info.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return nil
	}
	method := selection.Obj().(*types.Func)
	_, pointerRecv := method.Type().(*types.Signature).Recv().Type().(*types.Pointer)
	_, pointerBase := styleUnderlying(info.TypeOf(selector.X)).(*types.Pointer)
	if !pointerRecv || pointerBase || styleAtomicLoad(method) {
		return nil
	}
	return styleThrough(info, selector.X)
}

func styleShared(info *types.Info, expr ast.Expr) []styleWrite {
	root, _ := styleWriteRoot(info, expr)
	if root == nil {
		return nil
	}
	return []styleWrite{{root, true, false}}
}

func styleAtomicLoad(method *types.Func) bool {
	if method.Pkg() == nil || method.Pkg().Path() != "sync/atomic" {
		return false
	}
	return strings.HasPrefix(method.Name(), "Load")
}

func styleUnderlying(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return t.Underlying()
}

func styleWriteRoot(info *types.Info, expr ast.Expr) (*types.Var, bool) {
	current, shared := expr, false
	for depth := 0; depth < styleExprDepthMax; depth++ {
		switch x := ast.Unparen(current).(type) {
		case *ast.Ident:
			root, _ := info.Uses[x].(*types.Var)
			return root, shared
		case *ast.StarExpr:
			current, shared = x.X, true
		case *ast.IndexExpr:
			current, shared = x.X, shared || styleReference(info.TypeOf(x.X))
		case *ast.SelectorExpr:
			selection := info.Selections[x]
			if selection == nil {
				root, _ := info.Uses[x.Sel].(*types.Var)
				return root, shared
			}
			current, shared = x.X, shared || selection.Indirect()
		default:
			return nil, false
		}
	}
	return nil, false
}

func styleReference(t types.Type) bool {
	if _, ok := t.(*types.TypeParam); ok {
		return true
	}
	switch styleUnderlying(t).(type) {
	case *types.Slice, *types.Map, *types.Pointer:
		return true
	}
	return false
}

func styleArgumentWrites(info *types.Info, fn *ast.FuncDecl, writes []styleWrite) int {
	lists := []*ast.FieldList{fn.Recv, fn.Type.Params}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.FuncLit); ok {
			lists = append(lists, lit.Type.Params)
		}
		return true
	})
	params := map[*types.Var]bool{}
	for _, list := range lists {
		for _, field := range styleFields(list) {
			for _, name := range field.Names {
				param, ok := info.Defs[name].(*types.Var)
				if ok && !styleInventoried(param.Type()) {
					params[param] = true
				}
			}
		}
	}
	count := 0
	for _, write := range writes {
		if params[write.root] && write.shared && !write.through {
			count++
		}
	}
	return count
}

func styleFields(list *ast.FieldList) []*ast.Field {
	if list == nil {
		return nil
	}
	return list.List
}

func styleInventoried(t types.Type) bool {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return styleInventory[named.Obj().Pkg().Path()+"."+named.Obj().Name()]
}

func stylePackageWrites(writes []styleWrite) []styleFinding {
	var findings []styleFinding
	for _, write := range writes {
		root := write.root
		if root.Pkg() == nil || root.Parent() != root.Pkg().Scope() {
			continue
		}
		key := root.Pkg().Path() + "." + root.Name()
		findings = append(findings, styleFinding{rulePackageState, key, 1})
	}
	return findings
}

func styleErrorsDiscarded(info *types.Info, body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		var call *ast.CallExpr
		switch x := node.(type) {
		case *ast.ExprStmt:
			call, _ = ast.Unparen(x.X).(*ast.CallExpr)
		case *ast.DeferStmt:
			call = x.Call
		case *ast.GoStmt:
			call = x.Call
		}
		if call != nil && styleReturnsError(info, call) && !styleErrorNeverFails(info, call) {
			count++
		}
		return true
	})
	return count
}

func styleReturnsError(info *types.Info, call *ast.CallExpr) bool {
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	result := info.TypeOf(call)
	if result == nil {
		return false
	}
	tuple, ok := result.(*types.Tuple)
	if !ok {
		return types.Implements(result, errorType)
	}
	for index := 0; index < tuple.Len(); index++ {
		if types.Implements(tuple.At(index).Type(), errorType) {
			return true
		}
	}
	return false
}

func styleErrorNeverFails(info *types.Info, call *ast.CallExpr) bool {
	callee, ok := typeutil.Callee(info, call).(*types.Func)
	if !ok {
		return false
	}
	if styleNeverFails[callee.FullName()] {
		return true
	}
	if !styleFprints[callee.FullName()] || len(call.Args) == 0 {
		return false
	}
	writer := info.TypeOf(call.Args[0])
	return writer != nil && styleNeverFailWriters[writer.String()]
}

func styleChannelLoops(info *types.Info, body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if _, channel := styleUnderlying(info.TypeOf(loop.X)).(*types.Chan); channel {
			count++
		}
		return true
	})
	return count
}

func styleGraphBuild(files []styleTyped) styleGraph {
	graph := styleGraph{
		edges:    map[*types.Func][]*types.Func{},
		keys:     map[*types.Func]string{},
		closures: map[*types.Func]bool{},
	}
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			obj, ok := file.info.Defs[fn.Name].(*types.Func)
			if !ok {
				continue
			}
			graph.keys[obj] = file.keys[fn]
			graph.edges[obj] = styleFuncRefs(file.info, fn.Body)
			graph.closures[obj] = styleClosureRecursive(file.info, fn.Body)
		}
	}
	return graph
}

func styleFuncRefs(info *types.Info, body *ast.BlockStmt) []*types.Func {
	seen := map[*types.Func]bool{}
	var refs []*types.Func
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, expr := range append([]ast.Expr{call.Fun}, call.Args...) {
			fn := styleFuncNamed(info, expr)
			if fn != nil && !seen[fn] {
				seen[fn] = true
				refs = append(refs, fn)
			}
		}
		return true
	})
	return refs
}

func styleFuncNamed(info *types.Info, expr ast.Expr) *types.Func {
	current := ast.Unparen(expr)
	for depth := 0; depth < styleExprDepthMax; depth++ {
		switch x := current.(type) {
		case *ast.Ident:
			fn, _ := info.Uses[x].(*types.Func)
			if fn == nil {
				return nil
			}
			return fn.Origin()
		case *ast.SelectorExpr:
			current = x.Sel
		case *ast.IndexExpr:
			current = ast.Unparen(x.X)
		case *ast.IndexListExpr:
			current = ast.Unparen(x.X)
		default:
			return nil
		}
	}
	return nil
}

func styleClosureRecursive(info *types.Info, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != len(assign.Rhs) {
			return !found
		}
		for index, rhs := range assign.Rhs {
			lit, isLit := ast.Unparen(rhs).(*ast.FuncLit)
			id, isIdent := assign.Lhs[index].(*ast.Ident)
			if isLit && isIdent && styleUses(info, lit.Body, info.ObjectOf(id)) {
				found = true
			}
		}
		return !found
	})
	return found
}

func styleUses(info *types.Info, body *ast.BlockStmt, obj types.Object) bool {
	used := false
	ast.Inspect(body, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && obj != nil && info.Uses[id] == obj {
			used = true
		}
		return !used
	})
	return used
}

func styleRecursive(graph styleGraph) []styleFinding {
	var findings []styleFinding
	for fn, key := range graph.keys {
		if graph.closures[fn] || styleReaches(graph.edges, fn) {
			findings = append(findings, styleFinding{ruleRecursion, key, 1})
		}
	}
	return findings
}

func styleReaches(edges map[*types.Func][]*types.Func, start *types.Func) bool {
	seen := map[*types.Func]bool{}
	queue := slices.Clone(edges[start])
	for head := 0; head < len(queue); head++ {
		next := queue[head]
		if next == start {
			return true
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		queue = append(queue, edges[next]...)
	}
	return false
}

func styleDependencyFindings(t *testing.T, root string) []styleFinding {
	t.Helper()
	var findings []styleFinding
	for gomod, allowed := range styleDependencies {
		for _, required := range styleRequires(t, filepath.Join(root, gomod)) {
			if !slices.Contains(allowed, required) {
				findings = append(findings, styleFinding{ruleDependency, gomod + ":" + required, 1})
			}
		}
	}
	return findings
}

func styleRequires(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var required []string
	block := false
	for _, line := range strings.Split(string(src), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") {
			continue
		}
		if block {
			block = fields[0] != ")"
			if block {
				required = append(required, fields[0])
			}
			continue
		}
		if fields[0] == "require" && len(fields) > 1 {
			block = fields[1] == "("
			if !block {
				required = append(required, fields[1])
			}
		}
	}
	return required
}

func styleLedgerRead(t *testing.T) (map[string]int, bool) {
	t.Helper()
	src, err := os.ReadFile(styleLedgerPath)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]int{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	ledger := map[string]int{}
	for number, line := range strings.Split(strings.TrimSuffix(string(src), "\n"), "\n") {
		cut := strings.LastIndexByte(line, ' ')
		if cut < 0 {
			t.Fatalf("%s:%d: want <rule> <key> <value>", styleLedgerPath, number+1)
		}
		value, err := strconv.Atoi(line[cut+1:])
		if err != nil {
			t.Fatalf("%s:%d: %v", styleLedgerPath, number+1, err)
		}
		ledger[line[:cut]] = value
	}
	return ledger, true
}

func styleLedgerWrite(t *testing.T, ledger map[string]int) {
	t.Helper()
	keys := make([]string, 0, len(ledger))
	for key := range ledger {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s %d\n", key, ledger[key])
	}
	if err := os.WriteFile(styleLedgerPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func styleLedgerDiff(got map[string]int, want map[string]int) ([]string, []string) {
	var grown, shrunk []string
	for key, value := range got {
		if value > want[key] {
			grown = append(grown, fmt.Sprintf("%s %d, ledger %d", key, value, want[key]))
		}
	}
	for key, value := range want {
		if got[key] < value {
			shrunk = append(shrunk, fmt.Sprintf("%s %d, ledger %d", key, got[key], value))
		}
	}
	sort.Strings(grown)
	sort.Strings(shrunk)
	return grown, shrunk
}

func styleDocTableRows(doc string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(line[1:len(line)-1], "|")
		for index, cell := range cells {
			cells[index] = strings.TrimSpace(cell)
		}
		rows = append(rows, cells)
	}
	return rows
}

func styleDocCodes(cell string) []string {
	var codes []string
	for _, part := range strings.Split(cell, ",") {
		part = strings.TrimSpace(part)
		if len(part) > 2 && strings.HasPrefix(part, "`") && strings.HasSuffix(part, "`") {
			codes = append(codes, part[1:len(part)-1])
		}
	}
	sort.Strings(codes)
	return codes
}

func styleDocLimitsCheck(t *testing.T, doc string) {
	t.Helper()
	documented := map[string]string{}
	for _, cells := range styleDocTableRows(doc) {
		codes := styleDocCodes(cells[min(1, len(cells)-1)])
		if len(cells) == 4 && len(codes) == 1 && styleRulePattern.MatchString(codes[0]) {
			documented[codes[0]] = cells[3]
		}
	}
	for rule, limit := range styleLimits {
		want := "none"
		if limit > 0 {
			want = strconv.Itoa(limit)
		}
		if documented[rule] != want {
			t.Errorf("docs/style.md gives %s the limit %q, the gate %q",
				rule, documented[rule], want)
		}
	}
	for rule := range documented {
		if _, ok := styleLimits[rule]; !ok {
			t.Errorf("docs/style.md lists the check %s, which the gate does not run", rule)
		}
	}
}

func styleDocDependenciesCheck(t *testing.T, doc string) {
	t.Helper()
	documented := map[string][]string{}
	for _, cells := range styleDocTableRows(doc) {
		gomod := styleDocCodes(cells[0])
		if len(cells) == 2 && len(gomod) == 1 && strings.HasSuffix(gomod[0], "go.mod") {
			documented[gomod[0]] = styleDocCodes(cells[1])
		}
	}
	for gomod, allowed := range styleDependencies {
		want := slices.Sorted(slices.Values(allowed))
		if !slices.Equal(documented[gomod], want) {
			t.Errorf("docs/style.md allows %s %v, the gate %v", gomod, documented[gomod], want)
		}
	}
	if len(documented) != len(styleDependencies) {
		t.Errorf("docs/style.md lists %d go.mod files, the gate %d",
			len(documented), len(styleDependencies))
	}
}

func styleDocExamples(doc string) []styleExample {
	var examples []styleExample
	var current *styleExample
	for number, line := range strings.Split(doc, "\n") {
		fields := strings.Fields(line)
		opens := len(fields) == 3 && fields[0] == "```go"
		opens = opens && (fields[1] == "bad" || fields[1] == "good")
		switch {
		case current == nil && opens:
			current = &styleExample{rule: fields[2], bad: fields[1] == "bad", line: number + 1}
		case current != nil && strings.TrimSpace(line) == "```":
			examples = append(examples, *current)
			current = nil
		case current != nil:
			current.src += line + "\n"
		}
	}
	return examples
}

func styleDocExamplesCheck(t *testing.T, doc string) {
	t.Helper()
	seen := map[string]bool{}
	for _, example := range styleDocExamples(doc) {
		if _, ok := styleLimits[example.rule]; !ok {
			t.Errorf("docs/style.md:%d: no check is named %s", example.line, example.rule)
			continue
		}
		found := false
		for _, finding := range styleSnippetFindings(t, example) {
			found = found || finding.rule == example.rule
		}
		if example.bad && !found {
			t.Errorf("docs/style.md:%d: the gate passes the bad %s example",
				example.line, example.rule)
		}
		if !example.bad && found {
			t.Errorf("docs/style.md:%d: the gate fails the good %s example",
				example.line, example.rule)
		}
		seen[fmt.Sprint(example.rule, example.bad)] = true
	}
	for _, rule := range styleExampleRules {
		if !seen[fmt.Sprint(rule, true)] || !seen[fmt.Sprint(rule, false)] {
			t.Errorf("docs/style.md needs a bad and a good example of %s", rule)
		}
	}
}

func styleSnippetFindings(t *testing.T, example styleExample) []styleFinding {
	t.Helper()
	fset := token.NewFileSet()
	src := "package example\n\n" + example.src
	file, err := parser.ParseFile(fset, "example.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("docs/style.md:%d: %v", example.line, err)
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("docs/style.md:%d: %v", example.line, err)
	}
	typed := styleTyped{"example.go", file, info, pkg, styleFuncKeys("example.go", file)}
	findings := styleDeclFindings(fset, typed.name, file)
	findings = append(findings, styleTypedFindings(typed)...)
	return append(findings, styleRecursive(styleGraphBuild([]styleTyped{typed}))...)
}
