// The Go rules (spec §requests-and-responses, docs/lint.md): what keeps
// a hand-written handler of a routes package a pure function, judged
// over the package's Go files with the standard library's parser and no
// type information.
package vet

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"path"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// GoSource is one hand-written Go file of a routes package as the Go
// rules read it: its name and its text.
type GoSource struct {
	Name, Text string
}

// GoRule is a check over one hand-written Go file of a routes package
// (spec §requests-and-responses): a pure function of the parsed file and
// the package-level names of its package.
type GoRule interface {
	Analyzer
	Go(f *GoFile) []diag.Diagnostic
}

// GoFile is one parsed hand-written Go file with what the Go rules
// read: whether it declares a handler, the package-level variables and
// type declarations of the whole package, and a way to place a node.
type GoFile struct {
	Name     string
	File     *goast.File
	Handlers bool
	Vars     map[string]bool
	Types    map[string]*goast.TypeSpec
	fset     *gotoken.FileSet
}

// Pos is the position of n in the file, as diagnostics carry it.
func (f *GoFile) Pos(n goast.Node) token.Position {
	p := f.fset.Position(n.Pos())
	return token.At(f.Name, p.Offset, p.Line, p.Column)
}

// RunGo parses the sources of the routes package at dir and runs every
// Go rule among analyzers (all registered ones if none are named) over
// each file; a file that does not parse is skipped, the compiler
// reporting it. The warnings come file by file, sorted by position.
func RunGo(dir string, srcs []GoSource, analyzers ...Analyzer) []diag.Diagnostic {
	if len(analyzers) == 0 {
		analyzers = All()
	}
	var rules []GoRule
	for _, a := range analyzers {
		if r, ok := a.(GoRule); ok {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 || len(srcs) == 0 {
		return nil
	}
	fset := gotoken.NewFileSet()
	files := make([]*GoFile, 0, len(srcs))
	vars, types := map[string]bool{}, map[string]*goast.TypeSpec{}
	for _, src := range srcs {
		name := path.Join(dir, src.Name)
		f, err := goparser.ParseFile(fset, name, src.Text, goparser.ParseComments|goparser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			gd, ok := d.(*goast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				switch s := spec.(type) {
				case *goast.ValueSpec:
					if gd.Tok == gotoken.VAR {
						for _, id := range s.Names {
							vars[id.Name] = true
						}
					}
				case *goast.TypeSpec:
					types[s.Name.Name] = s
				}
			}
		}
		files = append(files, &GoFile{Name: name, File: f, Handlers: declaresHandler(f), fset: fset})
	}
	var out []diag.Diagnostic
	for _, f := range files {
		f.Vars, f.Types = vars, types
		for _, r := range rules {
			out = append(out, r.Go(f)...)
		}
	}
	diag.Sort(out)
	return out
}

func declaresHandler(f *goast.File) bool {
	for _, d := range f.Decls {
		if fd, ok := d.(*goast.FuncDecl); ok && handlerShaped(fd) {
			return true
		}
	}
	return false
}

// handlerShaped reports whether fd is a method taking a volt.Request
// first and returning a volt.Response alone (spec §handler-references).
func handlerShaped(fd *goast.FuncDecl) bool {
	if fd.Recv == nil || fd.Type.Params.NumFields() == 0 || fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
		return false
	}
	return spelled(fd.Type.Params.List[0].Type) == "volt.Request" && spelled(fd.Type.Results.List[0].Type) == "volt.Response"
}

func spelled(e goast.Expr) string {
	switch x := e.(type) {
	case *goast.Ident:
		return x.Name
	case *goast.SelectorExpr:
		return spelled(x.X) + "." + x.Sel.Name
	case *goast.StarExpr:
		return "*" + spelled(x.X)
	}
	return ""
}

func receiverName(fd *goast.FuncDecl) string {
	t := fd.Recv.List[0].Type
	if s, ok := t.(*goast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*goast.Ident); ok {
		return id.Name
	}
	return ""
}

type handlerReceiverRule struct{ meta }

var handlerReceiver = &handlerReceiverRule{meta{
	name: "handlerreceiver",
	doc:  "reports a controller type that is not an empty struct, since a handler's receiver must carry nothing a pure function could read",
}}

func (r *handlerReceiverRule) Go(f *GoFile) []diag.Diagnostic {
	var out []diag.Diagnostic
	seen := map[string]bool{}
	for _, d := range f.File.Decls {
		fd, ok := d.(*goast.FuncDecl)
		if !ok || !handlerShaped(fd) {
			continue
		}
		name := receiverName(fd)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		ts := f.Types[name]
		if ts == nil {
			continue
		}
		st, ok := ts.Type.(*goast.StructType)
		if ok && st.Fields.NumFields() == 0 {
			continue
		}
		pos := f.Pos(fd.Recv.List[0].Type)
		if ts.Pos().IsValid() && f.fset.File(ts.Pos()) == f.fset.File(fd.Pos()) {
			pos = f.Pos(ts)
		}
		out = append(out, r.warnf(pos, "controller %s carries state: a handler's receiver is an empty struct (struct{}), so nothing reaches a handler but its parameters", name))
	}
	return out
}

var worldImports = []string{
	"github.com/Piechutowski/volt/nao/run", "database/sql", "net", "os", "os/exec", "math/rand", "math/rand/v2", "crypto/rand",
}

type handlerImportsRule struct{ meta }

var handlerImports = &handlerImportsRule{meta{
	name: "handlerimports",
	doc:  "reports a file that declares a handler and imports a package that touches the world: the runner, database/sql, net, os, os/exec or a random source",
}}

func (r *handlerImportsRule) Go(f *GoFile) []diag.Diagnostic {
	if !f.Handlers {
		return nil
	}
	var out []diag.Diagnostic
	for _, imp := range f.File.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		for _, w := range worldImports {
			if p == w {
				out = append(out, r.warnf(f.Pos(imp), "a file that declares a handler imports %s; a handler describes its effects and the shim performs them (§requests-and-responses)", p))
			}
		}
	}
	return out
}

type handlerClockRule struct{ meta }

var handlerClock = &handlerClockRule{meta{
	name: "handlerclock",
	doc:  "reports time.Now, time.Since and time.Until in a file that declares a handler, which reads the clock from the request instead",
}}

func (r *handlerClockRule) Go(f *GoFile) []diag.Diagnostic {
	if !f.Handlers {
		return nil
	}
	var out []diag.Diagnostic
	goast.Inspect(f.File, func(n goast.Node) bool {
		call, ok := n.(*goast.CallExpr)
		if !ok {
			return true
		}
		switch spelled(call.Fun) {
		case "time.Now", "time.Since", "time.Until":
			out = append(out, r.warnf(f.Pos(call), "%s reads the clock; a handler reads r.Now(), the time the shim captured (§requests-and-responses)", spelled(call.Fun)))
		}
		return true
	})
	return out
}

type packageStateRule struct{ meta }

var packageState = &packageStateRule{meta{
	name: "packagestate",
	doc:  "reports package state in a routes package: a package-level var that is not a constant table, an init function, and an assignment to a package-level var",
}}

func (r *packageStateRule) Go(f *GoFile) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range f.File.Decls {
		switch d := d.(type) {
		case *goast.GenDecl:
			if d.Tok != gotoken.VAR {
				continue
			}
			for _, spec := range d.Specs {
				vs := spec.(*goast.ValueSpec)
				if !constantTable(vs) {
					out = append(out, r.warnf(f.Pos(vs), "package-level var %s is state; a routes package holds constant tables alone (§requests-and-responses)", names(vs.Names)))
				}
			}
		case *goast.FuncDecl:
			if d.Recv == nil && d.Name.Name == "init" {
				out = append(out, r.warnf(f.Pos(d), "init runs before anything is asked; a routes package has nothing to initialize (§requests-and-responses)"))
			}
			if d.Body == nil {
				continue
			}
			goast.Inspect(d.Body, func(n goast.Node) bool {
				switch s := n.(type) {
				case *goast.AssignStmt:
					if s.Tok == gotoken.DEFINE {
						return true
					}
					for _, l := range s.Lhs {
						if name := rootIdent(l); name != "" && f.Vars[name] {
							out = append(out, r.warnf(f.Pos(s), "assigns to the package-level var %s; a handler returns a new value instead (§requests-and-responses)", name))
						}
					}
				case *goast.IncDecStmt:
					if name := rootIdent(s.X); name != "" && f.Vars[name] {
						out = append(out, r.warnf(f.Pos(s), "changes the package-level var %s; a handler returns a new value instead (§requests-and-responses)", name))
					}
				}
				return true
			})
		}
	}
	return out
}

func constantTable(vs *goast.ValueSpec) bool {
	if len(vs.Values) == 0 {
		return false
	}
	for _, v := range vs.Values {
		switch v.(type) {
		case *goast.CompositeLit, *goast.BasicLit:
		default:
			return false
		}
	}
	return true
}

func rootIdent(e goast.Expr) string {
	for {
		switch x := e.(type) {
		case *goast.Ident:
			return x.Name
		case *goast.SelectorExpr:
			e = x.X
		case *goast.IndexExpr:
			e = x.X
		case *goast.StarExpr:
			e = x.X
		case *goast.ParenExpr:
			e = x.X
		default:
			return ""
		}
	}
}

func names(ids []*goast.Ident) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.Name
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

type goroutineRule struct{ meta }

var goroutine = &goroutineRule{meta{
	name: "goroutine",
	doc:  "reports a go statement in a routes package: a handler owns no lifetime, so it starts nothing",
}}

func (r *goroutineRule) Go(f *GoFile) []diag.Diagnostic {
	var out []diag.Diagnostic
	goast.Inspect(f.File, func(n goast.Node) bool {
		if s, ok := n.(*goast.GoStmt); ok {
			out = append(out, r.warnf(f.Pos(s), "starts a goroutine; a handler owns no lifetime, so parallel effects go through rt.All and work after the request to a job (§values-and-effects)"))
		}
		return true
	})
	return out
}

type streamCountRule struct{ meta }

var streamCount = &streamCountRule{meta{
	name: "streamcount",
	doc:  "reports every volt.Stream, the one response written by a function rather than described as a value, so the escapes stay counted",
}}

func (r *streamCountRule) Go(f *GoFile) []diag.Diagnostic {
	var out []diag.Diagnostic
	goast.Inspect(f.File, func(n goast.Node) bool {
		if call, ok := n.(*goast.CallExpr); ok && spelled(call.Fun) == "volt.Stream" {
			out = append(out, r.warnf(f.Pos(call), "volt.Stream writes the body with a function, the one escape from response values; keep the count low (§requests-and-responses)"))
		}
		return true
	})
	return out
}
