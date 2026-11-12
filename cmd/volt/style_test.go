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
	styleFunctionLinesMax   = 70
	styleLineColumnsMax     = 100
	styleTabColumns         = 4
	styleExpressionDepthMax = 64
	styleGraphVisitsMax     = 1 << 20
	styleLedgerPath         = "testdata/style.ledger"
	styleDocPath            = "../../docs/style.md"
)

const (
	ruleFunctionLength   = "function-length"
	ruleLineLength       = "line-length"
	ruleRecursion        = "recursion"
	ruleArgumentMutation = "argument-mutation"
	rulePackageWrite     = "package-write"
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
	rulePackageWrite:     0,
	ruleErrorDiscarded:   0,
	ruleLoopUnbounded:    0,
	ruleFileName:         0,
	ruleDependency:       0,
}

var styleExampleRules = []string{
	ruleRecursion,
	ruleArgumentMutation,
	rulePackageWrite,
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

var styleSkipDirectories = map[string]bool{
	".git":          true,
	"grammar":       true,
	"node_modules":  true,
	"zed-extension": true,
}

var styleCodeSuffixes = []string{".go", ".go.golden", ".volt", ".sql", ".sql.golden"}

var styleGoSuffixes = []string{".go", ".go.golden"}

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

var styleFiniteIterators = map[string]bool{
	"bytes.FieldsFuncSeq":   true,
	"bytes.FieldsSeq":       true,
	"bytes.Lines":           true,
	"bytes.SplitAfterSeq":   true,
	"bytes.SplitSeq":        true,
	"maps.All":              true,
	"maps.Keys":             true,
	"maps.Values":           true,
	"slices.All":            true,
	"slices.Backward":       true,
	"slices.Values":         true,
	"strings.FieldsFuncSeq": true,
	"strings.FieldsSeq":     true,
	"strings.Lines":         true,
	"strings.SplitAfterSeq": true,
	"strings.SplitSeq":      true,
}

type styleFinding struct {
	rule  string
	key   string
	value int
}

type styleRepository struct {
	root string
}

type styleSource struct {
	name string
	text []byte
}

type styleUnit struct {
	key        string
	name       *ast.Ident
	receiver   *ast.FieldList
	parameters *ast.FieldList
	body       *ast.BlockStmt
	start      token.Pos
	end        token.Pos
	init       bool
}

type styleTyped struct {
	name         string
	file         *ast.File
	information  *types.Info
	typesPackage *types.Package
	units        []styleUnit
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

type styleLedger struct {
	found    map[string]int
	recorded map[string]int
}

type styleExample struct {
	rule string
	bad  bool
	text string
	line int
}

func TestStyleLedger(t *testing.T) {
	repository := styleRepository{root: styleRootFind(t)}
	findings := styleSourceFindings(t, repository.sources(t))
	findings = append(findings, styleTypedFindingsAll(t, repository, gatePackages(t))...)
	findings = append(findings, styleDependencyFindings(repository.requirements(t))...)
	ledger := styleLedger{found: styleFindingsFold(findings), recorded: styleLedgerRead(t)}
	grown, shrunk := styleLedgerDiff(ledger)
	for _, entry := range grown {
		t.Errorf("style debt grew, bring the code back to docs/style.md: %s", entry)
	}
	if !*update {
		for _, entry := range shrunk {
			t.Errorf("style debt shrank, lower the ledger with "+
				"go test ./cmd/volt -run TestStyleLedger -update: %s", entry)
		}
		return
	}
	if len(grown) == 0 {
		styleLedgerWrite(t, ledger.found)
	}
}

func TestStyleDoc(t *testing.T) {
	text, err := os.ReadFile(styleDocPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(text)
	styleDocLimitsCheck(t, doc)
	styleDocDependenciesCheck(t, doc)
	styleDocExamplesCheck(t, doc)
}

func styleRootFind(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func (repository styleRepository) relative(t *testing.T, path string) string {
	t.Helper()
	relative, err := filepath.Rel(repository.root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(relative)
}

func (repository styleRepository) sources(t *testing.T) []styleSource {
	t.Helper()
	var sources []styleSource
	err := filepath.WalkDir(repository.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if styleSkipDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !styleSuffixed(path, styleCodeSuffixes) {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources = append(sources, styleSource{repository.relative(t, path), text})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].name < sources[j].name })
	return sources
}

func (repository styleRepository) requirements(t *testing.T) map[string][]string {
	t.Helper()
	requirements := map[string][]string{}
	for gomod := range styleDependencies {
		text, err := os.ReadFile(filepath.Join(repository.root, gomod))
		if err != nil {
			t.Fatal(err)
		}
		requirements[gomod] = styleRequirements(text)
	}
	return requirements
}

func styleSuffixed(path string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func styleFindingsFold(findings []styleFinding) map[string]int {
	folded := map[string]int{}
	for _, finding := range findings {
		folded[finding.rule+" "+finding.key] += finding.value
	}
	return folded
}

func styleFindingNonZero(finding styleFinding) []styleFinding {
	if finding.value == 0 {
		return nil
	}
	return []styleFinding{finding}
}

func styleSourceFindings(t *testing.T, sources []styleSource) []styleFinding {
	t.Helper()
	var findings []styleFinding
	for _, source := range sources {
		findings = append(findings, styleLineFindings(source)...)
		if !styleSuffixed(source.name, styleGoSuffixes) {
			continue
		}
		if !styleFileNamePattern.MatchString(filepath.Base(source.name)) {
			findings = append(findings, styleFinding{ruleFileName, source.name, 1})
		}
		fileSet := token.NewFileSet()
		mode := parser.SkipObjectResolution
		file, err := parser.ParseFile(fileSet, source.name, source.text, mode)
		if err != nil {
			t.Fatalf("%s: %v", source.name, err)
		}
		findings = append(findings, styleUnitFindings(fileSet, styleUnits(source.name, file))...)
	}
	return findings
}

func styleLineFindings(source styleSource) []styleFinding {
	count := 0
	for _, line := range strings.Split(string(source.text), "\n") {
		columns := utf8.RuneCountInString(line) + strings.Count(line, "\t")*(styleTabColumns-1)
		if columns > styleLineColumnsMax {
			count++
		}
	}
	return styleFindingNonZero(styleFinding{ruleLineLength, source.name, count})
}

func styleUnits(name string, file *ast.File) []styleUnit {
	var units []styleUnit
	seen := map[string]int{}
	for _, declaration := range file.Decls {
		switch declared := declaration.(type) {
		case *ast.FuncDecl:
			if declared.Body == nil {
				continue
			}
			key := name + ":" + styleFunctionName(declared)
			seen[key]++
			if seen[key] > 1 {
				key += "#" + strconv.Itoa(seen[key])
			}
			units = append(units, styleUnit{
				key:        key,
				name:       declared.Name,
				receiver:   declared.Recv,
				parameters: declared.Type.Params,
				body:       declared.Body,
				start:      declared.Pos(),
				end:        declared.End(),
				init:       declared.Recv == nil && declared.Name.Name == "init",
			})
		case *ast.GenDecl:
			units = append(units, styleLiteralUnits(name, declared)...)
		}
	}
	return units
}

func styleLiteralUnits(name string, declaration *ast.GenDecl) []styleUnit {
	if declaration.Tok != token.VAR {
		return nil
	}
	var units []styleUnit
	for _, specification := range declaration.Specs {
		value, ok := specification.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for index, expression := range value.Values {
			literal, ok := ast.Unparen(expression).(*ast.FuncLit)
			if !ok {
				continue
			}
			units = append(units, styleUnit{
				key:        name + ":" + value.Names[index].Name,
				parameters: literal.Type.Params,
				body:       literal.Body,
				start:      literal.Pos(),
				end:        literal.End(),
			})
		}
	}
	return units
}

func styleFunctionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	star := ""
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		star, receiver = "*", pointer.X
	}
	switch generic := receiver.(type) {
	case *ast.IndexExpr:
		receiver = generic.X
	case *ast.IndexListExpr:
		receiver = generic.X
	}
	typeName := "?"
	if identifier, ok := receiver.(*ast.Ident); ok {
		typeName = identifier.Name
	}
	return "(" + star + typeName + ")." + function.Name.Name
}

func styleUnitFindings(fileSet *token.FileSet, units []styleUnit) []styleFinding {
	var findings []styleFinding
	for _, unit := range units {
		lines := fileSet.Position(unit.end).Line - fileSet.Position(unit.start).Line + 1
		if lines > styleFunctionLinesMax {
			findings = append(findings, styleFinding{ruleFunctionLength, unit.key, lines})
		}
		loops := styleFinding{ruleLoopUnbounded, unit.key, styleForLoopsUnboundedCount(unit.body)}
		findings = append(findings, styleFindingNonZero(loops)...)
	}
	return findings
}

func styleForLoopsUnboundedCount(body *ast.BlockStmt) int {
	labels := map[*ast.ForStmt]string{}
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if labeled, ok := node.(*ast.LabeledStmt); ok {
			if loop, ok := labeled.Stmt.(*ast.ForStmt); ok {
				labels[loop] = labeled.Label.Name
			}
		}
		loop, ok := node.(*ast.ForStmt)
		if !ok {
			return true
		}
		if !styleLoopBounded(loop, labels[loop]) {
			count++
		}
		return true
	})
	return count
}

func styleLoopBounded(loop *ast.ForStmt, label string) bool {
	if loop.Cond == nil {
		return styleLoopEndless(loop.Body, label)
	}
	counter, step := styleLoopCounter(loop.Post)
	if counter == "" {
		return false
	}
	if styleWritten(loop.Body, []string{counter}) {
		return false
	}
	return styleConditionBounds(loop, counter, step)
}

func styleLoopCounter(post ast.Stmt) (string, int) {
	var target ast.Expr
	step := 0
	switch statement := post.(type) {
	case *ast.IncDecStmt:
		target, step = statement.X, 1
		if statement.Tok == token.DEC {
			step = -1
		}
	case *ast.AssignStmt:
		if len(statement.Lhs) != 1 {
			return "", 0
		}
		switch statement.Tok {
		case token.ADD_ASSIGN:
			target, step = statement.Lhs[0], 1
		case token.SUB_ASSIGN:
			target, step = statement.Lhs[0], -1
		}
	}
	if identifier, ok := ast.Unparen(target).(*ast.Ident); ok {
		return identifier.Name, step
	}
	return "", 0
}

func styleConditionBounds(loop *ast.ForStmt, counter string, step int) bool {
	pending := []ast.Expr{loop.Cond}
	index := 0
	for ; index < len(pending) && index < styleExpressionDepthMax; index++ {
		binary, ok := ast.Unparen(pending[index]).(*ast.BinaryExpr)
		if !ok {
			continue
		}
		if binary.Op == token.LAND {
			pending = append(pending, binary.X, binary.Y)
			continue
		}
		bound := styleComparisonBound(binary, counter, step)
		if bound == nil {
			continue
		}
		if !styleWritten(loop.Body, styleExpressionPaths(bound)) {
			return true
		}
	}
	if index < len(pending) {
		panic("styleConditionBounds: a condition deeper than styleExpressionDepthMax")
	}
	return false
}

func styleComparisonBound(binary *ast.BinaryExpr, counter string, step int) ast.Expr {
	rising := binary.Op == token.LSS || binary.Op == token.LEQ
	falling := binary.Op == token.GTR || binary.Op == token.GEQ
	if styleIdentifierNamed(binary.X, counter) {
		if (rising && step > 0) || (falling && step < 0) {
			return binary.Y
		}
		return nil
	}
	if styleIdentifierNamed(binary.Y, counter) {
		if (falling && step > 0) || (rising && step < 0) {
			return binary.X
		}
	}
	return nil
}

func styleExpressionPaths(expression ast.Expr) []string {
	var paths []string
	ast.Inspect(expression, func(node ast.Node) bool {
		if path := styleExpressionPath(node); path != "" {
			paths = append(paths, path)
		}
		return true
	})
	return paths
}

func styleExpressionPath(node ast.Node) string {
	current := node
	path := ""
	for depth := 0; depth < styleExpressionDepthMax; depth++ {
		switch typed := current.(type) {
		case *ast.Ident:
			return strings.TrimSuffix(typed.Name+"."+path, ".")
		case *ast.SelectorExpr:
			path = strings.TrimSuffix(typed.Sel.Name+"."+path, ".")
			current = ast.Unparen(typed.X)
		case *ast.ParenExpr:
			current = typed.X
		default:
			return ""
		}
	}
	panic("styleExpressionPath: an expression deeper than styleExpressionDepthMax")
}

func styleWritten(body *ast.BlockStmt, paths []string) bool {
	written := false
	ast.Inspect(body, func(node ast.Node) bool {
		for _, target := range styleAssignedPaths(node) {
			if slices.Contains(paths, target) {
				written = true
			}
		}
		return !written
	})
	return written
}

func styleAssignedPaths(node ast.Node) []string {
	var targets []ast.Expr
	switch statement := node.(type) {
	case *ast.AssignStmt:
		if statement.Tok != token.DEFINE {
			targets = statement.Lhs
		}
	case *ast.IncDecStmt:
		targets = []ast.Expr{statement.X}
	case *ast.RangeStmt:
		if statement.Tok == token.ASSIGN {
			targets = styleExpressionsPresent(statement.Key, statement.Value)
		}
	}
	var paths []string
	for _, target := range targets {
		if path := styleExpressionPath(ast.Unparen(target)); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func styleExpressionsPresent(expressions ...ast.Expr) []ast.Expr {
	var present []ast.Expr
	for _, expression := range expressions {
		if expression != nil {
			present = append(present, expression)
		}
	}
	return present
}

func styleLoopEndless(body *ast.BlockStmt, label string) bool {
	inner := styleInnerLabels(body)
	exits := 0
	var stack []ast.Node
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		if branch, ok := node.(*ast.BranchStmt); ok {
			if styleBranchExits(branch, stack, inner, label) {
				exits++
			}
		}
		if _, ok := node.(*ast.ReturnStmt); ok {
			exits++
		}
		stack = append(stack, node)
		return true
	})
	return exits == 0
}

func styleInnerLabels(body *ast.BlockStmt) map[string]bool {
	labels := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		if labeled, ok := node.(*ast.LabeledStmt); ok {
			labels[labeled.Label.Name] = true
		}
		return true
	})
	return labels
}

func styleBranchExits(
	branch *ast.BranchStmt, stack []ast.Node, inner map[string]bool, label string,
) bool {
	if branch.Tok == token.GOTO {
		return true
	}
	if branch.Label != nil {
		if inner[branch.Label.Name] {
			return false
		}
		return branch.Tok != token.CONTINUE || branch.Label.Name != label
	}
	if branch.Tok != token.BREAK {
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

func styleIdentifierNamed(expression ast.Expr, name string) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && identifier.Name == name
}

func styleTypedScope(path string) bool {
	if !strings.HasPrefix(path, module) {
		return false
	}
	if strings.Contains(path, "/testdata/") {
		return false
	}
	return !strings.HasPrefix(path, module+"/lang/conformance/snippets/")
}

func styleTypedFiles(
	t *testing.T, repository styleRepository, loaded []*packages.Package,
) []styleTyped {
	t.Helper()
	var files []styleTyped
	packages.Visit(loaded, nil, func(candidate *packages.Package) {
		if !styleTypedScope(candidate.PkgPath) {
			return
		}
		if len(candidate.Errors) > 0 {
			t.Fatalf("%s: %v", candidate.PkgPath, candidate.Errors[0])
		}
		for _, file := range candidate.Syntax {
			name := repository.relative(t, candidate.Fset.File(file.Pos()).Name())
			typed := styleTyped{
				name:         name,
				file:         file,
				information:  candidate.TypesInfo,
				typesPackage: candidate.Types,
				units:        styleUnits(name, file),
			}
			files = append(files, typed)
		}
	})
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files
}

func styleTypedFindingsAll(
	t *testing.T, repository styleRepository, loaded []*packages.Package,
) []styleFinding {
	t.Helper()
	typed := styleTypedFiles(t, repository, loaded)
	var findings []styleFinding
	for _, file := range typed {
		findings = append(findings, styleTypedFindings(file)...)
	}
	return append(findings, styleRecursionFindings(styleGraphBuild(typed))...)
}

func styleTypedFindings(file styleTyped) []styleFinding {
	var findings []styleFinding
	information := file.information
	for _, unit := range file.units {
		writes := styleWrites(information, unit.body)
		counted := []styleFinding{
			{ruleArgumentMutation, unit.key, styleArgumentWritesCount(information, unit, writes)},
			{ruleErrorDiscarded, unit.key, styleErrorsDiscardedCount(information, unit.body)},
			{ruleLoopUnbounded, unit.key, styleRangeLoopsUnboundedCount(information, unit.body)},
		}
		for _, finding := range counted {
			findings = append(findings, styleFindingNonZero(finding)...)
		}
		if !unit.init {
			findings = append(findings, stylePackageWrites(writes)...)
		}
	}
	return findings
}

func styleWrites(information *types.Info, body *ast.BlockStmt) []styleWrite {
	var writes []styleWrite
	ast.Inspect(body, func(node ast.Node) bool {
		writes = append(writes, styleNodeWrites(information, node)...)
		return true
	})
	return writes
}

func styleNodeWrites(information *types.Info, node ast.Node) []styleWrite {
	switch current := node.(type) {
	case *ast.AssignStmt:
		if current.Tok == token.DEFINE {
			return nil
		}
		return styleExpressionWrites(information, current.Lhs)
	case *ast.IncDecStmt:
		return styleExpressionWrites(information, []ast.Expr{current.X})
	case *ast.RangeStmt:
		if current.Tok != token.ASSIGN {
			return nil
		}
		present := styleExpressionsPresent(current.Key, current.Value)
		return styleExpressionWrites(information, present)
	case *ast.UnaryExpr:
		if current.Op == token.AND {
			return styleAddressWrites(information, current.X)
		}
	case *ast.CallExpr:
		return styleCallWrites(information, current)
	}
	return nil
}

func styleExpressionWrites(information *types.Info, expressions []ast.Expr) []styleWrite {
	var writes []styleWrite
	for _, expression := range expressions {
		if root, shared := styleWriteRoot(information, expression); root != nil {
			writes = append(writes, styleWrite{root, shared, false})
		}
	}
	return writes
}

func styleAddressWrites(information *types.Info, expression ast.Expr) []styleWrite {
	root, _ := styleWriteRoot(information, expression)
	if root == nil {
		return nil
	}
	return []styleWrite{{root, true, true}}
}

func styleSharedWrites(information *types.Info, expression ast.Expr) []styleWrite {
	root, _ := styleWriteRoot(information, expression)
	if root == nil {
		return nil
	}
	return []styleWrite{{root, true, false}}
}

func styleCallWrites(information *types.Info, call *ast.CallExpr) []styleWrite {
	if identifier, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		builtin, ok := information.Uses[identifier].(*types.Builtin)
		if ok && styleBuiltinMutators[builtin.Name()] && len(call.Args) > 0 {
			return styleSharedWrites(information, call.Args[0])
		}
	}
	if callee := typeutil.StaticCallee(information, call); callee != nil {
		index, ok := stdlibMutators[callee.Origin().FullName()]
		if ok && index < len(call.Args) {
			return styleSharedWrites(information, call.Args[index])
		}
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection := information.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return nil
	}
	method := selection.Obj().(*types.Func)
	_, pointerReceiver := method.Type().(*types.Signature).Recv().Type().(*types.Pointer)
	_, pointerBase := styleUnderlying(information.TypeOf(selector.X)).(*types.Pointer)
	if !pointerReceiver || pointerBase || styleMethodAtomicLoad(method) {
		return nil
	}
	return styleAddressWrites(information, selector.X)
}

func styleMethodAtomicLoad(method *types.Func) bool {
	if method.Pkg() == nil || method.Pkg().Path() != "sync/atomic" {
		return false
	}
	return strings.HasPrefix(method.Name(), "Load")
}

func styleUnderlying(valueType types.Type) types.Type {
	if valueType == nil {
		return nil
	}
	return valueType.Underlying()
}

func styleWriteRoot(information *types.Info, expression ast.Expr) (*types.Var, bool) {
	current, shared := expression, false
	for depth := 0; depth < styleExpressionDepthMax; depth++ {
		switch typed := ast.Unparen(current).(type) {
		case *ast.Ident:
			root, _ := information.Uses[typed].(*types.Var)
			return root, shared
		case *ast.StarExpr:
			current, shared = typed.X, true
		case *ast.IndexExpr:
			current, shared = typed.X, shared || styleTypeShares(information.TypeOf(typed.X))
		case *ast.SliceExpr:
			current, shared = typed.X, shared || styleTypeShares(information.TypeOf(typed.X))
		case *ast.SelectorExpr:
			selection := information.Selections[typed]
			if selection == nil {
				root, _ := information.Uses[typed.Sel].(*types.Var)
				return root, shared
			}
			current, shared = typed.X, shared || selection.Indirect()
		default:
			return nil, false
		}
	}
	panic("styleWriteRoot: an expression deeper than styleExpressionDepthMax")
}

func styleTypeShares(valueType types.Type) bool {
	if _, ok := valueType.(*types.TypeParam); ok {
		return true
	}
	switch styleUnderlying(valueType).(type) {
	case *types.Slice, *types.Map, *types.Pointer:
		return true
	}
	return false
}

func styleArgumentWritesCount(information *types.Info, unit styleUnit, writes []styleWrite) int {
	parameters := styleParameters(information, unit)
	count := 0
	for _, write := range writes {
		if !parameters[write.root] {
			continue
		}
		if write.shared && !write.through {
			count++
		}
	}
	return count
}

func styleParameters(information *types.Info, unit styleUnit) map[*types.Var]bool {
	lists := []*ast.FieldList{unit.receiver, unit.parameters}
	ast.Inspect(unit.body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok {
			lists = append(lists, literal.Type.Params)
		}
		return true
	})
	inventoried := styleReceiverInventoried(information, unit.receiver)
	parameters := map[*types.Var]bool{}
	for _, list := range lists {
		for _, field := range styleFields(list) {
			for _, name := range field.Names {
				parameter, ok := information.Defs[name].(*types.Var)
				if !ok {
					continue
				}
				if inventoried && styleInventoried(parameter.Type()) {
					continue
				}
				parameters[parameter] = true
			}
		}
	}
	return parameters
}

func styleFields(list *ast.FieldList) []*ast.Field {
	if list == nil {
		return nil
	}
	return list.List
}

func styleReceiverInventoried(information *types.Info, receiver *ast.FieldList) bool {
	if receiver == nil || len(receiver.List) == 0 {
		return false
	}
	return styleInventoried(information.TypeOf(receiver.List[0].Type))
}

func styleInventoried(valueType types.Type) bool {
	if pointer, ok := valueType.(*types.Pointer); ok {
		valueType = pointer.Elem()
	}
	named, ok := valueType.(*types.Named)
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
		findings = append(findings, styleFinding{rulePackageWrite, key, 1})
	}
	return findings
}

func styleErrorsDiscardedCount(information *types.Info, body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		count += styleNodeDiscards(information, node)
		return true
	})
	return count
}

func styleNodeDiscards(information *types.Info, node ast.Node) int {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		call, _ := ast.Unparen(statement.X).(*ast.CallExpr)
		return styleCallDiscards(information, call)
	case *ast.DeferStmt:
		return styleCallDiscards(information, statement.Call)
	case *ast.GoStmt:
		return styleCallDiscards(information, statement.Call)
	case *ast.AssignStmt:
		return styleBlankDiscards(information, statement)
	}
	return 0
}

func styleCallDiscards(information *types.Info, call *ast.CallExpr) int {
	if call == nil || !styleErrorResult(information, call) {
		return 0
	}
	if styleCallNeverFails(information, call) {
		return 0
	}
	return 1
}

func styleBlankDiscards(information *types.Info, assignment *ast.AssignStmt) int {
	if len(assignment.Rhs) != 1 || len(assignment.Lhs) < 2 {
		return 0
	}
	call, ok := ast.Unparen(assignment.Rhs[0]).(*ast.CallExpr)
	if !ok || styleCallNeverFails(information, call) {
		return 0
	}
	tuple, ok := information.TypeOf(call).(*types.Tuple)
	if !ok || tuple.Len() != len(assignment.Lhs) {
		return 0
	}
	count := 0
	for index, target := range assignment.Lhs {
		if !styleIdentifierNamed(target, "_") {
			continue
		}
		if styleImplementsError(tuple.At(index).Type()) {
			count++
		}
	}
	return count
}

func styleImplementsError(valueType types.Type) bool {
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return types.Implements(valueType, errorType)
}

func styleErrorResult(information *types.Info, call *ast.CallExpr) bool {
	result := information.TypeOf(call)
	if result == nil {
		return false
	}
	tuple, ok := result.(*types.Tuple)
	if !ok {
		return styleImplementsError(result)
	}
	for index := 0; index < tuple.Len(); index++ {
		if styleImplementsError(tuple.At(index).Type()) {
			return true
		}
	}
	return false
}

func styleCallNeverFails(information *types.Info, call *ast.CallExpr) bool {
	callee, ok := typeutil.Callee(information, call).(*types.Func)
	if !ok {
		return false
	}
	if styleNeverFails[callee.FullName()] {
		return true
	}
	if !styleFprints[callee.FullName()] || len(call.Args) == 0 {
		return false
	}
	writer := information.TypeOf(call.Args[0])
	return writer != nil && styleNeverFailWriters[writer.String()]
}

func styleRangeLoopsUnboundedCount(information *types.Info, body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if !styleRangeBounded(information, loop) {
			count++
		}
		return true
	})
	return count
}

func styleRangeBounded(information *types.Info, loop *ast.RangeStmt) bool {
	switch styleUnderlying(information.TypeOf(loop.X)).(type) {
	case *types.Chan:
		return false
	case *types.Signature:
		return styleIteratorFinite(information, loop.X)
	}
	return true
}

func styleIteratorFinite(information *types.Info, expression ast.Expr) bool {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok {
		return false
	}
	callee := typeutil.StaticCallee(information, call)
	if callee == nil {
		return false
	}
	return styleFiniteIterators[callee.Origin().FullName()]
}

func styleGraphBuild(files []styleTyped) styleGraph {
	graph := styleGraph{
		edges:    map[*types.Func][]*types.Func{},
		keys:     map[*types.Func]string{},
		closures: map[*types.Func]bool{},
	}
	for _, file := range files {
		for _, unit := range file.units {
			if unit.name == nil {
				continue
			}
			node, ok := file.information.Defs[unit.name].(*types.Func)
			if !ok {
				continue
			}
			graph.keys[node] = unit.key
			graph.edges[node] = styleCallees(file.information, unit.body)
			graph.closures[node] = styleClosureRecursive(file.information, unit.body)
		}
	}
	return graph
}

func styleCallees(information *types.Info, body *ast.BlockStmt) []*types.Func {
	seen := map[*types.Func]bool{}
	var callees []*types.Func
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, expression := range append([]ast.Expr{call.Fun}, call.Args...) {
			callee := styleFunctionNamed(information, expression)
			if callee == nil || seen[callee] {
				continue
			}
			seen[callee] = true
			callees = append(callees, callee)
		}
		return true
	})
	return callees
}

func styleFunctionNamed(information *types.Info, expression ast.Expr) *types.Func {
	current := ast.Unparen(expression)
	for depth := 0; depth < styleExpressionDepthMax; depth++ {
		switch typed := current.(type) {
		case *ast.Ident:
			function, _ := information.Uses[typed].(*types.Func)
			if function == nil {
				return nil
			}
			return function.Origin()
		case *ast.SelectorExpr:
			current = typed.Sel
		case *ast.IndexExpr:
			current = ast.Unparen(typed.X)
		case *ast.IndexListExpr:
			current = ast.Unparen(typed.X)
		default:
			return nil
		}
	}
	panic("styleFunctionNamed: an expression deeper than styleExpressionDepthMax")
}

func styleClosureRecursive(information *types.Info, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
			return !found
		}
		for index, value := range assignment.Rhs {
			literal, isLiteral := ast.Unparen(value).(*ast.FuncLit)
			identifier, isIdentifier := assignment.Lhs[index].(*ast.Ident)
			if !isLiteral || !isIdentifier {
				continue
			}
			if styleObjectUsed(information, literal.Body, information.ObjectOf(identifier)) {
				found = true
			}
		}
		return !found
	})
	return found
}

func styleObjectUsed(information *types.Info, body *ast.BlockStmt, object types.Object) bool {
	if object == nil {
		return false
	}
	used := false
	ast.Inspect(body, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Ident); ok && information.Uses[identifier] == object {
			used = true
		}
		return !used
	})
	return used
}

func styleRecursionFindings(graph styleGraph) []styleFinding {
	var findings []styleFinding
	for node, key := range graph.keys {
		if graph.closures[node] || styleNodeReachesItself(graph.edges, node) {
			findings = append(findings, styleFinding{ruleRecursion, key, 1})
		}
	}
	return findings
}

func styleNodeReachesItself(edges map[*types.Func][]*types.Func, start *types.Func) bool {
	seen := map[*types.Func]bool{}
	queue := slices.Clone(edges[start])
	head := 0
	for ; head < len(queue) && head < styleGraphVisitsMax; head++ {
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
	if head < len(queue) {
		panic("styleNodeReachesItself: the call graph is larger than styleGraphVisitsMax")
	}
	return false
}

func styleDependencyFindings(requirements map[string][]string) []styleFinding {
	var findings []styleFinding
	for gomod, required := range requirements {
		for _, requirement := range required {
			if !slices.Contains(styleDependencies[gomod], requirement) {
				key := gomod + ":" + requirement
				findings = append(findings, styleFinding{ruleDependency, key, 1})
			}
		}
	}
	return findings
}

func styleRequirements(text []byte) []string {
	var required []string
	block := false
	for _, line := range strings.Split(string(text), "\n") {
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
		if fields[0] != "require" || len(fields) < 2 {
			continue
		}
		block = fields[1] == "("
		if !block {
			required = append(required, fields[1])
		}
	}
	return required
}

func styleLedgerRead(t *testing.T) map[string]int {
	t.Helper()
	text, err := os.ReadFile(styleLedgerPath)
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s is missing: it holds the style debt older than docs/style.md, "+
			"and -update never recreates it", styleLedgerPath)
	}
	if err != nil {
		t.Fatal(err)
	}
	recorded := map[string]int{}
	for number, line := range strings.Split(string(text), "\n") {
		if line == "" {
			continue
		}
		cut := strings.LastIndexByte(line, ' ')
		if cut < 0 {
			t.Fatalf("%s:%d: want <rule> <key> <value>", styleLedgerPath, number+1)
		}
		value, err := strconv.Atoi(line[cut+1:])
		if err != nil {
			t.Fatalf("%s:%d: %v", styleLedgerPath, number+1, err)
		}
		recorded[line[:cut]] = value
	}
	return recorded
}

func styleLedgerWrite(t *testing.T, found map[string]int) {
	t.Helper()
	keys := make([]string, 0, len(found))
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&builder, "%s %d\n", key, found[key])
	}
	if err := os.WriteFile(styleLedgerPath, []byte(builder.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func styleLedgerDiff(ledger styleLedger) ([]string, []string) {
	var grown, shrunk []string
	for key, value := range ledger.found {
		if value > ledger.recorded[key] {
			entry := fmt.Sprintf("%s %d, ledger %d", key, value, ledger.recorded[key])
			grown = append(grown, entry)
		}
	}
	for key, value := range ledger.recorded {
		if ledger.found[key] < value {
			entry := fmt.Sprintf("%s %d, ledger %d", key, ledger.found[key], value)
			shrunk = append(shrunk, entry)
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
		if len(cells) != 4 {
			continue
		}
		codes := styleDocCodes(cells[1])
		if len(codes) == 1 && styleRulePattern.MatchString(codes[0]) {
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
			current.text += line + "\n"
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
	fileSet := token.NewFileSet()
	text := "package example\n\n" + example.text
	file, err := parser.ParseFile(fileSet, "example.go", text, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("docs/style.md:%d: %v", example.line, err)
	}
	information := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	config := types.Config{Importer: importer.Default()}
	checked, err := config.Check("example", fileSet, []*ast.File{file}, information)
	if err != nil {
		t.Fatalf("docs/style.md:%d: %v", example.line, err)
	}
	units := styleUnits("example.go", file)
	typed := styleTyped{"example.go", file, information, checked, units}
	findings := styleUnitFindings(fileSet, units)
	findings = append(findings, styleTypedFindings(typed)...)
	return append(findings, styleRecursionFindings(styleGraphBuild([]styleTyped{typed}))...)
}
