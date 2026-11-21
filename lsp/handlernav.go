package lsp

import (
	goast "go/ast"
	"go/parser"
	"go/printer"
	gotoken "go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (ix *voltIndex) handlerRefAdd(pkg *lang.Package, path string, ref *ast.GoRef) {
	if ref == nil || len(ref.Parts) != 2 {
		return
	}
	ctrl, action := ref.Parts[0].Name(), ref.Parts[1].Name()
	ci, ok := pkg.Controllers[ctrl]
	if !ok || ci.Action(action) == nil {
		return
	}
	csym := voltSym{"controller", path, ctrl}
	asym := voltSym{"action", path, ctrl + "." + action}
	cs, as := spanOf(ref.Parts[0]), spanOf(ref.Parts[1])
	ix.refs = append(ix.refs, voltRef{cs, cs, csym, false, ctrl, ""}, voltRef{as, as, asym, false, action, ""})
	if _, done := ix.defs[csym]; !done {
		ix.controllerDefine(pkg, path, ci)
	}
}

func (ix *voltIndex) controllerDefine(pkg *lang.Package, path string, ci *lang.ControllerInfo) {
	actions := make([]string, len(ci.Actions))
	for i, a := range ci.Actions {
		actions[i] = a.Name
	}
	scan := ix.goScan(pkg)
	methods := scan.methods
	var impls []string
	for typ, ms := range methods {
		if !slices.ContainsFunc(actions, func(a string) bool { _, ok := ms[a]; return !ok }) {
			impls = append(impls, typ)
		}
	}
	slices.Sort(impls)
	impl := ""
	switch {
	case len(impls) == 1:
		impl = impls[0]
	case slices.Contains(impls, ci.Name):
		impl = ci.Name
	}
	csym := voltSym{"controller", path, ci.Name}
	if impl != "" {
		def := voltDef{md: "**controller** `" + ci.Name + "`, implemented by `" + impl + "` in package `" + pkg.Name + "`"}
		if gt, ok := scan.types[impl]; ok {
			def.span = voltSpan{file: gt.File, pos: gt.Pos, end: gt.End}
		}
		ix.defs[csym] = def
		for _, a := range actions {
			gf := methods[impl][a]
			ix.defs[voltSym{"action", path, ci.Name + "." + a}] = voltDef{
				span: voltSpan{file: gf.File, pos: gf.Pos, end: gf.End},
				gofn: &gf,
				md:   goFuncHover(a, pkg.Name, &gf),
			}
		}
		return
	}
	why := "no type in package `" + pkg.Name + "`'s Go files declares every action of `" + ci.Name + "`"
	if len(impls) > 1 {
		why = "several types in package `" + pkg.Name + "` declare every action of `" + ci.Name + "` (" + strings.Join(impls, ", ") + ")"
	}
	iface := ci.Name + "Controller"
	gen := handlersInterfaceRead(filepath.Join(pkg.Dir, "volt_handlers.go"), iface)
	def := voltDef{md: "**controller** `" + ci.Name + "`: " + why + "; the generated interface `" + iface + "` is the contract"}
	if gen.typ.file != "" {
		def.span = gen.typ
	}
	ix.defs[csym] = def
	for _, a := range actions {
		m, ok := gen.methods[a]
		adef := voltDef{md: "`" + a + "`, an action of `" + ci.Name + "`: " + why}
		if ok {
			adef.span = m.span
			adef.md = "```go\n" + m.sig + "\n```\nthe action of the generated `" + iface + "`; " + why + "\n\n*volt_handlers.go*"
		}
		ix.defs[voltSym{"action", path, ci.Name + "." + a}] = adef
	}
}

type handlersInterface struct {
	typ     voltSpan
	methods map[string]handlersMethod
}

type handlersMethod struct {
	span voltSpan
	sig  string
}

func handlersInterfaceRead(file, name string) handlersInterface {
	out := handlersInterface{methods: map[string]handlersMethod{}}
	text, err := os.ReadFile(file)
	if err != nil {
		return out
	}
	fset := gotoken.NewFileSet()
	f, err := parser.ParseFile(fset, file, text, parser.SkipObjectResolution)
	if f == nil {
		return out
	}
	span := func(n goast.Node) voltSpan {
		s, e := fset.Position(n.Pos()), fset.Position(n.End())
		return voltSpan{file: file, pos: token.At(file, s.Offset, s.Line, s.Column), end: token.At(file, e.Offset, e.Line, e.Column)}
	}
	for _, d := range f.Decls {
		gd, ok := d.(*goast.GenDecl)
		if !ok || gd.Tok != gotoken.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*goast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			out.typ = span(ts.Name)
			it, ok := ts.Type.(*goast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range it.Methods.List {
				ft, ok := m.Type.(*goast.FuncType)
				if !ok || len(m.Names) != 1 {
					continue
				}
				var b strings.Builder
				b.WriteString(m.Names[0].Name)
				if err := printer.Fprint(&b, fset, ft); err == nil {
					sig := strings.Replace(b.String(), "func(", "(", 1)
					out.methods[m.Names[0].Name] = handlersMethod{span: span(m.Names[0]), sig: sig}
				}
			}
		}
	}
	return out
}
