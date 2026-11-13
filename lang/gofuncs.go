// Go source facts for Go references (spec §pipelines, §validators, §types): a .volt file
// names Go only in the containing package's own Go files, read with the
// standard library's parser, and every reference is held to the exact
// spelled signature or declaration.
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
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/source"
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

// GoType is one top-level type spec of a package directory, its
// underlying type by spelling (spec §types).
type GoType struct {
	Name       string
	File       string
	Pos, End   token.Position // the name's span in File
	Underlying string         // as spelled after the name; the aliased type for an alias
	Alias      bool           // declared with =
	Generic    bool           // declares type parameters
}

// GoSource is one Go file of a package directory as the scan reads it
// : its name and its text, or its first bytes alone when they
// carry the generated marker. Equal sources scan to equal
// functions.
type GoSource struct {
	Name string
	Text string
	Head bool // Text is the file's first bytes only: a generated file
}

// GoSourcesRead reads the non-test Go files directly in dir through
// src, by name. A file that cannot be read is absent, as it is for the
// go tool.
func GoSourcesRead(src source.Source, dir string) []GoSource {
	entries, err := src.ReadDir(dir)
	if err != nil {
		return nil
	}
	var srcs []GoSource
	for _, e := range entries {
		if e.Dir || !strings.HasSuffix(e.Name, ".go") || strings.HasSuffix(e.Name, "_test.go") {
			continue
		}
		b, err := src.ReadFile(filepath.Join(dir, e.Name))
		if err != nil {
			continue
		}
		srcs = append(srcs, goSourceOf(e.Name, b))
	}
	return srcs
}

func goSourceOf(name string, b []byte) GoSource {
	head := b
	if len(head) > 256 {
		head = head[:256]
	}
	line, _, _ := bytes.Cut(head, []byte("\n"))
	if bytes.HasPrefix(line, []byte("// Code generated ")) && bytes.HasSuffix(bytes.TrimSpace(line), []byte("DO NOT EDIT.")) {
		return GoSource{Name: name, Text: string(head), Head: true}
	}
	return GoSource{Name: name, Text: string(b)}
}

// GoFuncsOf scans a package directory's sources under go/build's file
// constraints and returns its top-level functions (methods excluded)
// and its top-level type specs, each by name. A file with a syntax
// error contributes the declarations parsed before the error and is
// reported in broken ("file: line:col message"). The result is a
// function of the sources and of the process's build context.
func GoFuncsOf(dir string, srcs []GoSource) (funcs map[string]GoFunc, gotypes map[string]GoType, broken []string) {
	sc := goScanBuild(dir, srcs)
	return sc.funcs, sc.types, sc.broken
}

type goPackage struct {
	Name string
	File string
}

func goScanBuild(dir string, srcs []GoSource) *goScan {
	funcs := map[string]GoFunc{}
	gotypes := map[string]GoType{}
	var gopkg goPackage
	broken := goFilesParse(dir, srcs, func(fset *gotoken.FileSet, path string, f *goast.File) {
		if gopkg.Name == "" && f.Name != nil && f.Name.Name != "documentation" {
			gopkg = goPackage{Name: f.Name.Name, File: filepath.Base(path)}
		}
		for _, d := range f.Decls {
			if gd, ok := d.(*goast.GenDecl); ok && gd.Tok == gotoken.TYPE {
				for _, spec := range gd.Specs {
					ts, ok := spec.(*goast.TypeSpec)
					if !ok || ts.Name == nil || ts.Type == nil {
						continue
					}
					if _, dup := gotypes[ts.Name.Name]; !dup {
						gotypes[ts.Name.Name] = goTypeOf(fset, path, ts)
					}
				}
				continue
			}
			fn, ok := d.(*goast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || fn.Type == nil {
				continue
			}
			if _, dup := funcs[fn.Name.Name]; dup {
				continue
			}
			funcs[fn.Name.Name] = goFuncOf(fset, path, fn)
		}
	})
	sc := &goScan{}
	sc.once.Do(func() { sc.funcs, sc.types, sc.broken, sc.gopkg = funcs, gotypes, broken, gopkg })
	return sc
}

// GoMethodsOf scans a package directory's sources as GoFuncsOf does and
// returns its methods by the name of their receiver's type, then by
// their own name (spec §handler-references): what a type implementing a
// controller declares. Sig spells the receiver.
func GoMethodsOf(dir string, srcs []GoSource) map[string]map[string]GoFunc {
	methods := map[string]map[string]GoFunc{}
	goFilesParse(dir, srcs, func(fset *gotoken.FileSet, path string, f *goast.File) {
		for _, d := range f.Decls {
			fn, ok := d.(*goast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || fn.Name == nil || fn.Type == nil {
				continue
			}
			recv := receiverTypeName(fn.Recv.List[0].Type)
			if recv == "" {
				continue
			}
			if methods[recv] == nil {
				methods[recv] = map[string]GoFunc{}
			}
			if _, dup := methods[recv][fn.Name.Name]; !dup {
				methods[recv][fn.Name.Name] = goFuncOf(fset, path, fn)
			}
		}
	})
	return methods
}

func receiverTypeName(x goast.Expr) string {
	if star, ok := x.(*goast.StarExpr); ok {
		x = star.X
	}
	switch t := x.(type) {
	case *goast.IndexExpr:
		x = t.X
	case *goast.IndexListExpr:
		x = t.X
	}
	if id, ok := x.(*goast.Ident); ok {
		return id.Name
	}
	return ""
}

func goFilesParse(dir string, srcs []GoSource, each func(fset *gotoken.FileSet, path string, f *goast.File)) (broken []string) {
	fset := gotoken.NewFileSet()
	ctxt := build.Default
	for _, src := range srcs {
		if src.Head {
			continue
		}
		ctxt.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(src.Text)), nil }
		if ok, _ := ctxt.MatchFile(dir, src.Name); !ok {
			continue
		}
		path := filepath.Join(dir, src.Name)
		f, perr := parser.ParseFile(fset, path, src.Text, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			broken = append(broken, fmt.Sprintf("%s: %v", src.Name, perr))
		}
		if f != nil {
			each(fset, path, f)
		}
	}
	return broken
}

// GoFuncsScan is GoFuncsOf over the sources src holds now.
func GoFuncsScan(src source.Source, dir string) (funcs map[string]GoFunc, gotypes map[string]GoType, broken []string) {
	return GoFuncsOf(dir, GoSourcesRead(src, dir))
}

func goTypeOf(fset *gotoken.FileSet, path string, ts *goast.TypeSpec) GoType {
	start, end := fset.Position(ts.Name.Pos()), fset.Position(ts.Name.End())
	return GoType{
		Name:       ts.Name.Name,
		File:       path,
		Pos:        token.At(path, start.Offset, start.Line, start.Column),
		End:        token.At(path, end.Offset, end.Line, end.Column),
		Underlying: types.ExprString(ts.Type),
		Alias:      ts.Assign.IsValid(),
		Generic:    ts.TypeParams != nil && len(ts.TypeParams.List) > 0,
	}
}

func goFuncOf(fset *gotoken.FileSet, path string, fn *goast.FuncDecl) GoFunc {
	start, end := fset.Position(fn.Name.Pos()), fset.Position(fn.Name.End())
	gf := GoFunc{
		Name:    fn.Name.Name,
		File:    path,
		Generic: fn.Type.TypeParams != nil && len(fn.Type.TypeParams.List) > 0,
		Pos:     token.At(path, start.Offset, start.Line, start.Column),
		End:     token.At(path, end.Offset, end.Line, end.Column),
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
	if err := printer.Fprint(&buf, fset, &goast.FuncDecl{Recv: fn.Recv, Name: fn.Name, Type: fn.Type}); err == nil {
		gf.Sig = strings.TrimSpace(buf.String())
	}
	if fn.Doc != nil {
		gf.Doc = strings.TrimSpace(fn.Doc.Text())
	}
	return gf
}

type goScan struct {
	once   sync.Once
	funcs  map[string]GoFunc
	types  map[string]GoType
	broken []string
	gopkg  goPackage
}

type goFuncsCache struct {
	mu sync.Mutex
	by map[string]*goScan
}

func (c *checker) goFuncs(pkg *Package) *goScan {
	if c.gofuncs == nil {
		c.gofuncs = &goFuncsCache{}
	}
	c.gofuncs.mu.Lock()
	if c.gofuncs.by == nil {
		c.gofuncs.by = map[string]*goScan{}
	}
	sc := c.gofuncs.by[pkg.Dir]
	if sc == nil {
		sc = &goScan{}
		c.gofuncs.by[pkg.Dir] = sc
	}
	c.gofuncs.mu.Unlock()
	sc.once.Do(func() {
		b := goScanBuild(pkg.Dir, GoSourcesRead(c.pr.src, pkg.Dir))
		sc.funcs, sc.types, sc.broken, sc.gopkg = b.funcs, b.types, b.broken, b.gopkg
	})
	return sc
}

func (sc *goScan) goTypes() check.GoTypes {
	out := make(check.GoTypes, len(sc.types))
	for name, t := range sc.types {
		if !t.Generic {
			out[name] = t.Underlying
		}
	}
	return out
}

func (sc *goScan) brokenTypesNote() string {
	if len(sc.broken) == 0 {
		return ""
	}
	return " (note: " + strings.Join(sc.broken, "; ") + "; declarations after a syntax error are invisible)"
}

func (sc *goScan) brokenHint() string {
	if len(sc.broken) == 0 {
		return ""
	}
	return " (note: " + strings.Join(sc.broken, "; ") + " — functions after a syntax error are invisible)"
}
