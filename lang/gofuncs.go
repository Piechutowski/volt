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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// GoSource is one Go file of a package directory as the scan reads it
// (D87): its name and its text, or its first bytes alone when they
// carry the generated marker (D81), so a million-line output costs one
// small read and is never parsed. Equal sources scan to equal
// functions; a tool that keeps a scan compares the sources it read
// with the sources on disk now, byte for byte, and nothing else.
type GoSource struct {
	Name string
	Text string
	Head bool // Text is the file's first bytes only: a generated file
}

// GoSourcesRead reads the non-test Go files directly in dir, by name.
// A file that cannot be read is absent, as it is for the go tool.
func GoSourcesRead(dir string) []GoSource {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var srcs []GoSource
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := goSourceRead(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		src.Name = name
		srcs = append(srcs, src)
	}
	return srcs
}

// goSourceRead reads a file's first bytes and, unless they carry the
// generated marker on the first line (^// Code generated .* DO NOT
// EDIT\.$), the rest.
func goSourceRead(path string) (GoSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return GoSource{}, err
	}
	defer f.Close()
	var head [256]byte
	n, err := io.ReadFull(f, head[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return GoSource{}, err
	}
	line, _, _ := bytes.Cut(head[:n], []byte("\n"))
	if bytes.HasPrefix(line, []byte("// Code generated ")) && bytes.HasSuffix(bytes.TrimSpace(line), []byte("DO NOT EDIT.")) {
		return GoSource{Text: string(head[:n]), Head: true}, nil
	}
	var text bytes.Buffer
	text.Write(head[:n])
	if _, err := text.ReadFrom(f); err != nil {
		return GoSource{}, err
	}
	return GoSource{Text: text.String()}, nil
}

// GoFuncsOf scans a package directory's sources the way the go tool
// would compile them — go/build's file constraints: no leading "_" or
// ".", a matching GOOS/GOARCH suffix, a satisfied //go:build line —
// and returns its top-level functions (methods excluded) by name. A
// file with a syntax error contributes the declarations parsed before
// the error and is reported in broken ("file: line:col message"), so a
// diagnostic can say why a function later in that file is invisible.
// The result is a function of the sources and of the process's build
// context, which never changes.
func GoFuncsOf(dir string, srcs []GoSource) (funcs map[string]GoFunc, broken []string) {
	funcs = map[string]GoFunc{}
	fset := gotoken.NewFileSet()
	ctxt := build.Default
	for _, src := range srcs {
		if src.Head {
			continue // volt's own output never declares a check or a plug (D81)
		}
		ctxt.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(src.Text)), nil }
		if ok, _ := ctxt.MatchFile(dir, src.Name); !ok {
			continue // the go tool would not compile it; neither do we count it
		}
		path := filepath.Join(dir, src.Name)
		f, perr := parser.ParseFile(fset, path, src.Text, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			broken = append(broken, fmt.Sprintf("%s: %v", src.Name, perr))
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

// GoFuncsScan is GoFuncsOf over the sources on disk now.
func GoFuncsScan(dir string) (funcs map[string]GoFunc, broken []string) {
	return GoFuncsOf(dir, GoSourcesRead(dir))
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
	once   sync.Once // the scan runs once, whichever package asks first
	funcs  map[string]GoFunc
	broken []string
}

// goFuncsCache holds one goScan per package directory, shared by the
// per-package checkers of a phase (PERF-7). A session seeds it with
// scans whose identity is stable while their sources are (D86, D87).
type goFuncsCache struct {
	mu sync.Mutex
	by map[string]*goScan
}

// goScanOf is a finished scan: its once is spent, its content given.
func goScanOf(funcs map[string]GoFunc, broken []string) *goScan {
	sc := &goScan{}
	sc.once.Do(func() { sc.funcs, sc.broken = funcs, broken })
	return sc
}

// goFuncs returns the package directory's functions, scanned once per
// check run.
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
	sc.once.Do(func() { sc.funcs, sc.broken = GoFuncsScan(pkg.Dir) })
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
