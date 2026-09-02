// Go source facts for Go references (spec §V3.2, §V12.5): a .volt file
// names Go only in the containing package's own Go files, so the
// checker reads them with the standard library's parser — no build, no
// gopls — and holds every reference to a declared function with the
// exact spelled signature (D63). The Go compiler still compiles the
// result; this catches the typo and the wrong type first.
package lang

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	gotoken "go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/Piechutowski/volt/lang/token"
)

// GoParam is one parameter of a Go function, type by spelling.
type GoParam struct {
	Name string // "" when unnamed
	Type string // as spelled in source, e.g. "string", "http.Handler"
}

// GoFunc is one top-level function of a package directory.
type GoFunc struct {
	Name     string
	File     string
	Pos, End token.Position // the name's span in File
	Params   []GoParam      // one entry per parameter name (a, b string → two)
	Variadic bool           // final parameter is ...T
	Generic  bool           // declares type parameters: never referenceable
	Results  []string       // result types by spelling
	Sig      string         // "func EmailValid(email string) error"
	Doc      string         // doc comment text, "" when absent
}

// GoFuncsIn is GoFuncsScan without the parse-error report.
func GoFuncsIn(dir string) map[string]GoFunc {
	funcs, _ := GoFuncsScan(dir)
	return funcs
}

// GoFuncsScan parses the non-test Go files directly in dir that the go
// tool would compile for the package — go/build's file constraints:
// no leading "_" or ".", a matching GOOS/GOARCH suffix, a satisfied
// //go:build line — and returns its top-level functions (methods
// excluded) by name. A file with a syntax error contributes the
// declarations parsed before the error and is reported in broken
// ("file: line:col message"), so a diagnostic can say why a function
// later in that file is invisible.
func GoFuncsScan(dir string) (funcs map[string]GoFunc, broken []string) {
	funcs = map[string]GoFunc{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return funcs, nil
	}
	fset := gotoken.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, _ := build.Default.MatchFile(dir, name); !ok {
			continue // the go tool would not compile it; neither do we count it
		}
		path := filepath.Join(dir, name)
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			broken = append(broken, fmt.Sprintf("%s: %v", name, perr))
		}
		if f == nil {
			continue
		}
		for _, d := range f.Decls {
			fn, ok := d.(*goast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || fn.Type == nil {
				continue
			}
			if _, dup := funcs[fn.Name.Name]; dup {
				continue
			}
			funcs[fn.Name.Name] = goFuncOf(fset, path, fn)
		}
	}
	return funcs, broken
}

func goFuncOf(fset *gotoken.FileSet, path string, fn *goast.FuncDecl) GoFunc {
	start, end := fset.Position(fn.Name.Pos()), fset.Position(fn.Name.End())
	gf := GoFunc{
		Name:    fn.Name.Name,
		File:    path,
		Generic: fn.Type.TypeParams != nil && len(fn.Type.TypeParams.List) > 0,
		Pos:     token.Position{Filename: path, Offset: start.Offset, Line: start.Line, Column: start.Column},
		End:     token.Position{Filename: path, Offset: end.Offset, Line: end.Line, Column: end.Column},
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typ := field.Type
			if ell, ok := typ.(*goast.Ellipsis); ok {
				gf.Variadic = true
				typ = ell.Elt
			}
			spelled := types.ExprString(typ)
			if len(field.Names) == 0 {
				gf.Params = append(gf.Params, GoParam{Type: spelled})
			}
			for _, n := range field.Names {
				gf.Params = append(gf.Params, GoParam{Name: n.Name, Type: spelled})
			}
		}
	}
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			n := len(field.Names)
			if n == 0 {
				n = 1
			}
			for i := 0; i < n; i++ {
				gf.Results = append(gf.Results, types.ExprString(field.Type))
			}
		}
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, &goast.FuncDecl{Name: fn.Name, Type: fn.Type}); err == nil {
		gf.Sig = strings.TrimSpace(buf.String())
	}
	if fn.Doc != nil {
		gf.Doc = strings.TrimSpace(fn.Doc.Text())
	}
	return gf
}

// goScan is one package directory's scan, cached per check run.
type goScan struct {
	funcs  map[string]GoFunc
	broken []string
}

// goFuncs returns the package directory's functions, scanned once per
// check run.
func (c *checker) goFuncs(pkg *Package) *goScan {
	if c.gofuncs == nil {
		c.gofuncs = map[string]*goScan{}
	}
	sc, ok := c.gofuncs[pkg.Dir]
	if !ok {
		funcs, broken := GoFuncsScan(pkg.Dir)
		sc = &goScan{funcs: funcs, broken: broken}
		c.gofuncs[pkg.Dir] = sc
	}
	return sc
}

// brokenHint explains an invisible function when a Go file of the
// package failed to parse: everything after the error is unseen.
func (sc *goScan) brokenHint() string {
	if len(sc.broken) == 0 {
		return ""
	}
	return " (note: " + strings.Join(sc.broken, "; ") + " — functions after a syntax error are invisible)"
}
