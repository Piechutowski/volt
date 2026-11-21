package layout_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/layout"
	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

var projectFixtures = []struct {
	dir      string
	inModule bool
}{
	{"../router/testdata/blog", false},
	{"../router/testdata/shop", false},
	{"testdata/longnames", false},
	{"../../itest/blog/app", true},
	{"../../nao/itest", true},
}

var committedOutputs = []string{
	"../router/testdata",
	"../../nao/gen/golang/testdata",
	"../../itest/blog",
	"../../nao/itest",
}

func TestGeneratedCodeFollowsTheRules(t *testing.T) {
	outputs := map[string][]byte{}
	for _, fx := range projectFixtures {
		for name, code := range projectGenerate(t, fx.dir, fx.inModule) {
			outputs[name] = code
		}
	}
	for _, dir := range committedOutputs {
		for name, code := range committedRead(t, dir) {
			outputs[name] = code
		}
	}
	if len(outputs) == 0 {
		t.Fatal("no generated file to check")
	}
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, problem := range rulesBroken(outputs[name]) {
			t.Errorf("%s: %s", name, problem)
		}
	}
}

func projectGenerate(t *testing.T, dir string, inModule bool) map[string][]byte {
	t.Helper()
	pr, err := projectLoad(dir, inModule)
	if err != nil {
		t.Fatal(err)
	}
	root := dir
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("%s: fixture has errors: %v", root, diags)
	}
	out := map[string][]byte{}
	for path, pkg := range pr.Packages {
		at := filepath.Join(root, path)
		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{Source: "package " + pkg.Path})
			if err != nil {
				t.Fatalf("%s: %v", at, err)
			}
			for name, code := range files {
				out["generated "+filepath.Join(at, name)] = code
			}
		}
		if pkg.HasRouting() {
			files, err := router.Generate(pkg, router.Options{Source: "package " + pkg.Path})
			if err != nil {
				t.Fatalf("%s: %v", at, err)
			}
			for name, code := range files {
				out["generated "+filepath.Join(at, name)] = code
			}
		}
	}
	return out
}

func committedRead(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		golden := strings.HasSuffix(name, ".go.golden")
		generated := strings.HasPrefix(name, "volt_") && strings.HasSuffix(name, ".go")
		if !golden && !generated {
			return nil
		}
		code, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(code, []byte("// Code generated ")) {
			out[path] = code
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

var lowerVolt = regexp.MustCompile(`^volt([a-z_]|Out$)`)

func rulesBroken(src []byte) []string {
	var problems []string
	formatted, err := format.Source(src)
	if err != nil {
		return append(problems, "does not parse: "+err.Error())
	}
	if !bytes.Equal(formatted, src) {
		problems = append(problems, "is not gofmt clean")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return append(problems, "does not parse: "+err.Error())
	}
	at := func(p token.Pos) int { return fset.Position(p).Line }
	breakable := breakableLines(file, at)
	for i, line := range strings.Split(string(src), "\n") {
		if !layout.Fits(line) && breakable[i+1] {
			problem := lineProblem(i+1, "is %d columns wide, over %d, and could still be wrapped",
				layout.Width(line), layout.LineColumnsMax)
			problems = append(problems, problem)
		}
	}
	report := func(p token.Pos, format string, args ...any) {
		problems = append(problems, lineProblem(at(p), format, args...))
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			shapeCheck(report, at, "literal", n.Lbrace, n.Rbrace, exprs(n.Elts))
			if keyedMany(n) && at(n.Lbrace) == at(n.Rbrace) {
				report(n.Lbrace, "a struct literal of %d fields is on one line", len(n.Elts))
			}
		case *ast.CallExpr:
			shapeCheck(report, at, "call", n.Lparen, n.Rparen, exprs(n.Args))
			callCheck(report, fset, n)
		case *ast.FuncType:
			if n.Params != nil {
				shapeCheck(report, at, "parameter list", n.Params.Opening, n.Params.Closing, fields(n.Params.List))
			}
		case *ast.FuncDecl:
			if n.Body != nil {
				unusedCheck(report, n.Type, n.Recv, n.Body)
			}
		case *ast.FuncLit:
			unusedCheck(report, n.Type, nil, n.Body)
		case *ast.Ident:
			if lowerVolt.MatchString(n.Name) {
				report(n.Pos(), "the name %s is not volt and a descriptive name in Go initialism style", n.Name)
			}
		}
		return true
	})
	return problems
}

func lineProblem(line int, format string, args ...any) string {
	return fmt.Sprintf("line %d: ", line) + fmt.Sprintf(format, args...)
}

type reporter func(token.Pos, string, ...any)

func shapeCheck(report reporter, at func(token.Pos) int, what string, open, close token.Pos, elems []ast.Node) {
	if len(elems) == 0 {
		return
	}
	onOpen := 0
	for _, e := range elems {
		if at(e.Pos()) == at(open) {
			onOpen++
		}
	}
	lastEnd := at(elems[len(elems)-1].End())
	switch {
	case onOpen == len(elems):
	case onOpen == 0 && at(close) > lastEnd:
		for i := 1; i < len(elems); i++ {
			if at(elems[i].Pos()) == at(elems[i-1].End()) {
				report(elems[i].Pos(), "a wrapped %s puts two elements on one line", what)
			}
		}
	default:
		report(open, "a %s mixes the one-line and the wrapped style", what)
	}
}

var runtimeCalls = map[string]bool{"volt": true, "rt": true}

func callCheck(report reporter, fset *token.FileSet, call *ast.CallExpr) {
	at := func(p token.Pos) int { return fset.Position(p).Line }
	if len(call.Args) > 0 && at(call.Args[0].Pos()) != at(call.Lparen) {
		for _, arg := range call.Args {
			inner, isCall := arg.(*ast.CallExpr)
			wrapped := isCall && len(inner.Args) > 0 && at(inner.Args[0].Pos()) != at(inner.Lparen)
			if wrapped && oneLineFits(fset, inner) {
				report(inner.Pos(), "a wrapped call is the argument of a wrapped call although it fits on its line; "+
					"name it first")
			}
		}
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, _ := sel.X.(*ast.Ident)
	runtime := pkg != nil && runtimeCalls[pkg.Name] || sel.Sel.Name == "Do" || sel.Sel.Name == "Raw"
	if !runtime || sel.Sel.Name == "Some" {
		return
	}
	for _, arg := range call.Args {
		if id, isIdent := arg.(*ast.Ident); isIdent && (id.Name == "nil" || id.Name == "true" || id.Name == "false") {
			report(arg.Pos(), "%s passes a positional %s", sel.Sel.Name, id.Name)
		}
	}
	if pkg != nil && pkg.Name == "volt" && len(call.Args) > 0 {
		if lit, isLit := call.Args[0].(*ast.BasicLit); isLit && lit.Kind == token.INT {
			report(lit.Pos(), "%s passes the status %s as a number", sel.Sel.Name, lit.Value)
		}
	}
}

func unusedCheck(report reporter, typ *ast.FuncType, recv *ast.FieldList, body *ast.BlockStmt) {
	used := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			ast.Inspect(sel.X, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok {
					used[id.Name] = true
				}
				return true
			})
			return false
		}
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	for _, list := range []*ast.FieldList{recv, typ.Params} {
		if list == nil {
			continue
		}
		for _, f := range list.List {
			for _, name := range f.Names {
				if name.Name != "_" && !used[name.Name] {
					report(name.Pos(), "the parameter %s is unused; name it _", name.Name)
				}
			}
		}
	}
}

func keyedMany(n *ast.CompositeLit) bool {
	if _, isMap := n.Type.(*ast.MapType); isMap || len(n.Elts) < 2 {
		return false
	}
	for _, e := range n.Elts {
		kv, keyed := e.(*ast.KeyValueExpr)
		if !keyed {
			return false
		}
		if _, named := kv.Key.(*ast.Ident); !named {
			return false
		}
	}
	return true
}

func exprs(es []ast.Expr) []ast.Node {
	out := make([]ast.Node, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

func fields(fs []*ast.Field) []ast.Node {
	out := make([]ast.Node, len(fs))
	for i, f := range fs {
		out[i] = f
	}
	return out
}

func projectLoad(dir string, inModule bool) (*lang.Project, error) {
	if !inModule {
		return lang.Load(disk.Source{}, dir)
	}
	root, err := lang.FindRoot(disk.Source{}, dir)
	if err != nil {
		return nil, err
	}
	return lang.LoadDirs(disk.Source{}, root, []string{dir}, nil)
}

func breakableLines(file *ast.File, at func(token.Pos) int) map[int]bool {
	lines := map[int]bool{}
	group := func(open token.Pos, elems []ast.Node) {
		if !open.IsValid() || len(elems) == 0 {
			return
		}
		for _, e := range elems {
			if at(e.Pos()) != at(open) {
				return
			}
		}
		lines[at(open)] = true
	}
	body := func(b *ast.BlockStmt) {
		if b != nil && len(b.List) > 0 && at(b.Lbrace) == at(b.Rbrace) {
			lines[at(b.Lbrace)] = true
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			group(n.Lparen, exprs(n.Args))
		case *ast.CompositeLit:
			group(n.Lbrace, exprs(n.Elts))
		case *ast.FuncType:
			if n.Params != nil {
				group(n.Params.Opening, fields(n.Params.List))
			}
			if n.Results != nil {
				group(n.Results.Opening, fields(n.Results.List))
			}
		case *ast.FuncLit:
			body(n.Body)
		case *ast.FuncDecl:
			body(n.Body)
		case *ast.BinaryExpr:
			if n.Op == token.ADD && at(n.X.End()) == at(n.Y.Pos()) {
				lines[at(n.OpPos)] = true
			}
		}
		return true
	})
	return lines
}

func TestLongNamesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := fixtureCopy(filepath.Join("testdata", "longnames"), dir); err != nil {
		t.Fatal(err)
	}
	gomod := "module longnames\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(disk.Source{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("the long-names fixture has errors: %v", diags)
	}
	for path, pkg := range pr.Packages {
		files := map[string][]byte{}
		if pkg.HasSchema() {
			data, err := model.Generate(pkg, model.Options{Source: "package " + path})
			if err != nil {
				t.Fatal(err)
			}
			maps.Copy(files, data)
		}
		if pkg.HasRouting() {
			routes, err := router.Generate(pkg, router.Options{Source: "package " + path})
			if err != nil {
				t.Fatal(err)
			}
			maps.Copy(files, routes)
		}
		for name, code := range files {
			full := filepath.Join(pkg.Dir, name)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, code, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = dir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("the long-names fixture does not compile:\n%s", out)
	}
}

func fixtureCopy(src, dir string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		code, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), code, 0o644)
	})
}

func TestOverlongLinesPassOnlyWhenNothingCanBreak(t *testing.T) {
	long := strings.Repeat("x", 100)
	for name, c := range map[string]struct {
		src    string
		passes bool
	}{
		"struct tag": {
			"package p\n\ntype T struct {\n\tField string `json:\"" + long + "\"`\n}\n", true,
		},
		"string literal in a wrapped call": {
			"package p\n\nfunc f() {\n\tg(\n\t\t\"" + long + "\",\n\t)\n}\n\nfunc g(string) {}\n", true,
		},
		"identifier alone on its line": {
			"package p\n\nvar " + long + "Value = 1\n", true,
		},
		"call left on one line": {
			"package p\n\nfunc f() {\n\tg(\"" + long + "\")\n}\n\nfunc g(string) {}\n", false,
		},
		"concatenation left on one line": {
			"package p\n\nvar s = \"" + long + "\" + \"tail\"\n", false,
		},
		"function body left on one line": {
			"package p\n\nfunc f() int { return len(\"" + long + "\") }\n", false,
		},
		"nested call wrapped although it fits": {
			"package p\n\nfunc f() {\n\tg(\n\t\th(\n\t\t\t1,\n\t\t),\n\t\t2,\n\t)\n}\n\n" +
				"func g(int, int) {}\n\nfunc h(int) int { return 0 }\n", false,
		},
		"nested call wrapped because it cannot fit": {
			"package p\n\nfunc f() {\n\tg(\n\t\th(\n\t\t\t\"" + long + "\",\n\t\t),\n\t\t2,\n\t)\n}\n\n" +
				"func g(int, int) {}\n\nfunc h(string) int { return 0 }\n", true,
		},
	} {
		problems := rulesBroken([]byte(c.src))
		if c.passes && len(problems) > 0 {
			t.Errorf("%s: %v; want no problem, nothing on the line can break", name, problems)
		}
		if !c.passes && len(problems) == 0 {
			t.Errorf("%s: no problem; want the overlong line reported, it can still break", name)
		}
	}
}

func oneLineFits(fset *token.FileSet, call *ast.CallExpr) bool {
	var b bytes.Buffer
	if err := printer.Fprint(&b, token.NewFileSet(), call); err != nil {
		return false
	}
	flat := b.String()
	if strings.Contains(flat, "\n") {
		return false
	}
	column := fset.Position(call.Pos()).Column
	return (column-1)*layout.TabColumns+layout.Width(flat)+len(",") <= layout.LineColumnsMax
}
