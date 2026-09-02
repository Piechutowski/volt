// Go-reference navigation: the functions a Volt file names by rule live
// in the containing package's own Go files (§V3.2 plugs, §V12.5
// checks), so the language server can find them with the standard
// library's Go parser — no gopls, no build — and answer go-to-definition
// and hover with the real signature and doc comment. The Go compiler
// stays the authority on whether the reference is valid; this is the
// editor's courtesy, not a check.
package lsp

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/Piechutowski/volt/lang/token"
)

// goFunc is one top-level function found in a package directory.
type goFunc struct {
	span voltSpan // the function name in its Go file
	sig  string   // "func EmailValid(email string) error"
	doc  string   // doc comment text, "" when absent
}

// goFuncsIn parses every non-test Go file directly in dir and returns
// its top-level functions (methods excluded) by name. Errors are
// swallowed: a half-typed Go file must not break Volt navigation.
func goFuncsIn(dir string) map[string]goFunc {
	out := map[string]goFunc{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	fset := gotoken.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if f == nil {
			continue // unparsable beyond recovery
		}
		_ = err
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}
			var buf bytes.Buffer
			sigDecl := &ast.FuncDecl{Name: fn.Name, Type: fn.Type}
			if err := printer.Fprint(&buf, fset, sigDecl); err != nil {
				continue
			}
			start := fset.Position(fn.Name.Pos())
			end := fset.Position(fn.Name.End())
			gf := goFunc{
				span: voltSpan{
					file: path,
					pos:  token.Position{Filename: path, Offset: start.Offset, Line: start.Line, Column: start.Column},
					end:  token.Position{Filename: path, Offset: end.Offset, Line: end.Line, Column: end.Column},
				},
				sig: strings.TrimSpace(buf.String()),
			}
			if fn.Doc != nil {
				gf.doc = strings.TrimSpace(fn.Doc.Text())
			}
			if _, dup := out[fn.Name.Name]; !dup {
				out[fn.Name.Name] = gf
			}
		}
	}
	return out
}

// goFuncHover renders the hover for a resolved (or missing) reference.
func goFuncHover(name, pkgName string, gf *goFunc) string {
	if gf == nil {
		return "`" + name + "` — no such function in package `" + pkgName + "`'s Go files yet; " +
			"declare `func " + name + "(...) error` beside the schema (the Go compiler is the authority, §V12.5)"
	}
	var b strings.Builder
	b.WriteString("```go\n" + gf.sig + "\n```\n")
	if gf.doc != "" {
		b.WriteString("\n" + gf.doc + "\n")
	}
	b.WriteString("\n*" + filepath.Base(gf.span.file) + "*")
	return b.String()
}
