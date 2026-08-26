// Project-wide navigation for Volt-layer symbols. The single-file index
// (index.go) resolves DBML names inside one document; this one spans the
// project, so `model: db.Post` in app/routes.volt reaches `Table posts`
// in db/schema.volt, and asking for references on that table finds every
// resources declaration that names it.
package lsp

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/orm/gen/golang"
)

// voltSym names a symbol reachable from the Volt layer.
type voltSym struct {
	kind string // "table" | "pipeline" | "package"
	pkg  string // declaring package path; the target path for "package"
	name string // model name (tables) or pipeline name; "" for packages
}

// voltSpan is one identifier's extent in a file.
type voltSpan struct {
	file     string
	pos, end token.Position
}

type voltRef struct {
	span voltSpan
	sym  voltSym
	decl bool
}

// voltDef is a declaration: where it is, and the node itself so hover
// can describe what was declared.
type voltDef struct {
	span  voltSpan
	table *ast.Table
	pipe  *ast.Pipeline
}

// voltIndex is the whole project's Volt-layer symbol graph.
type voltIndex struct {
	defs  map[voltSym]voltDef
	refs  []voltRef
	texts map[string]string // open buffers, for position conversion
}

func spanOf(n ast.Node) voltSpan {
	p := n.Pos()
	return voltSpan{file: p.Filename, pos: p, end: n.End()}
}

// buildVoltIndex walks every package's declarations twice: definitions
// first (tables and pipelines), then the references that name them.
func buildVoltIndex(pr *lang.Project, overlay map[string]string) *voltIndex {
	ix := &voltIndex{defs: map[voltSym]voltDef{}, texts: overlay}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.Table:
				name, err := golang.ModelName(d)
				if err != nil {
					continue
				}
				ix.define(voltSym{"table", path, name}, voltDef{span: spanOf(d.Name), table: d})
			case *ast.Pipeline:
				ix.define(voltSym{"pipeline", path, d.Name.Name()}, voltDef{span: spanOf(d.Name), pipe: d})
			}
		}
		// A package "declaration" is its first file, so an import can
		// jump somewhere useful.
		if len(pkg.Files) > 0 {
			first := pkg.Files[0]
			ix.defs[voltSym{"package", path, ""}] = voltDef{span: voltSpan{
				file: first.Name,
				pos:  token.Position{Filename: first.Name, Line: 1, Column: 1},
				end:  token.Position{Filename: first.Name, Line: 1, Column: 1},
			}}
		}
	}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.ImportDecl:
				for _, spec := range d.Specs {
					target := spec.PathString()
					if _, ok := pr.Packages[target]; !ok {
						continue
					}
					last := spec.Path[len(spec.Path)-1]
					ix.refs = append(ix.refs, voltRef{spanOf(last), voltSym{"package", target, ""}, false})
				}
			case *ast.Scope:
				ix.scopeRefs(pkg, path, d)
			}
		}
	}
	return ix
}

func (ix *voltIndex) define(sym voltSym, def voltDef) {
	if _, dup := ix.defs[sym]; dup {
		return // duplicate declaration: the checker reports it
	}
	ix.defs[sym] = def
	ix.refs = append(ix.refs, voltRef{def.span, sym, true})
}

// scopeRefs records the symbol references a scope subtree makes: pipe:
// settings name pipelines, model: settings name tables.
func (ix *voltIndex) scopeRefs(pkg *lang.Package, path string, sc *ast.Scope) {
	ix.settingRefs(pkg, path, sc.Settings)
	for _, item := range sc.Items {
		switch it := item.(type) {
		case *ast.Scope:
			ix.scopeRefs(pkg, path, it)
		case *ast.Resources:
			// The declaration itself names the model (§V5.1).
			target := path
			if it.Pkg != nil {
				t, ok := pkg.Imports[it.Pkg.Name()]
				if !ok {
					continue
				}
				target = t
			}
			span := spanOf(it.Name)
			if it.Pkg != nil {
				span.pos = it.Pkg.Pos()
			}
			ix.refs = append(ix.refs, voltRef{span, voltSym{"table", target, it.Name.Name()}, false})
			ix.settingRefs(pkg, path, it.Settings)
		case *ast.Route:
			ix.settingRefs(pkg, path, it.Settings)
		}
	}
}

func (ix *voltIndex) settingRefs(pkg *lang.Package, path string, list *ast.SettingList) {
	if list == nil {
		return
	}
	for _, s := range list.Settings {
		switch s.Name {
		case "pipe":
			if id, ok := s.Value.(*ast.Ident); ok {
				ix.refs = append(ix.refs, voltRef{spanOf(id), voltSym{"pipeline", path, id.Name()}, false})
			}
		}
	}
}

// at returns the reference under the byte offset in file, if any.
func (ix *voltIndex) at(file string, offset int) *voltRef {
	for i := range ix.refs {
		r := &ix.refs[i]
		if r.span.file == file && offset >= r.span.pos.Offset && offset <= r.span.end.Offset {
			return r
		}
	}
	return nil
}

// text returns a file's current content: the open buffer when there is
// one (so positions match what the user sees), else the saved file.
func (ix *voltIndex) text(file string) string {
	if t, ok := ix.texts[file]; ok {
		return t
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(b)
}

func (ix *voltIndex) location(sp voltSpan) *protocol.Location {
	text := ix.text(sp.file)
	return &protocol.Location{
		URI: "file://" + sp.file,
		Range: protocol.Range{
			Start: offsetToLSP(text, sp.pos.Offset),
			End:   offsetToLSP(text, sp.end.Offset),
		},
	}
}

// offsetToLSP converts a byte offset into text to an LSP position
// (0-based line, 0-based UTF-16 column).
func offsetToLSP(text string, off int) protocol.Position {
	if off > len(text) {
		off = len(text)
	}
	if off < 0 {
		off = 0
	}
	line := strings.Count(text[:off], "\n")
	lineStart := strings.LastIndexByte(text[:off], '\n') + 1
	col := len(utf16.Encode([]rune(text[lineStart:off])))
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

/* ===== the two features ===== */

// voltDefinition resolves the Volt-layer symbol under the cursor to its
// declaration, in whichever file of the project declares it.
func (d *Document) voltDefinition(pos protocol.Position) *protocol.Location {
	if d.vindex == nil {
		return nil
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	def, ok := d.vindex.defs[ref.sym]
	if !ok {
		return nil
	}
	return d.vindex.location(def.span)
}

// voltReferences lists every project-wide use of the symbol under the
// cursor — including the resources declarations naming a table.
func (d *Document) voltReferences(pos protocol.Position, includeDecl bool) []protocol.Location {
	if d.vindex == nil {
		return nil
	}
	ref := d.vindex.at(pathFromURI(d.URI), d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	var out []protocol.Location
	for _, r := range d.vindex.refs {
		if r.sym != ref.sym || (r.decl && !includeDecl) {
			continue
		}
		out = append(out, *d.vindex.location(r.span))
	}
	return out
}

/* ===== hover ===== */

// voltHover describes the Volt-layer symbol under the cursor. For a
// table it spells out the mapping that is otherwise invisible in
// routes.volt: `model: db.Post` names the *Go model* nao derives from
// `Table posts`, not the table's own name (§V5.4.2).
func (d *Document) voltHover(pos protocol.Position) *protocol.Hover {
	if d.vindex == nil {
		return nil
	}
	path := pathFromURI(d.URI)
	ref := d.vindex.at(path, d.FromLSP(pos))
	if ref == nil {
		return nil
	}
	def, ok := d.vindex.defs[ref.sym]
	if !ok {
		return nil
	}

	var md string
	switch ref.sym.kind {
	case "table":
		md = tableModelHover(ref.sym, def)
	case "pipeline":
		md = pipelineHover(ref.sym, def)
	case "package":
		md = "**package** `" + ref.sym.pkg + "`"
	}
	if md == "" {
		return nil
	}
	rng := protocol.Range{
		Start: offsetToLSP(d.Text, ref.span.pos.Offset),
		End:   offsetToLSP(d.Text, ref.span.end.Offset),
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		Range:    &rng,
	}
}

func tableModelHover(sym voltSym, def voltDef) string {
	t := def.table
	if t == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**model `%s`** — `Table %s` in package `%s`\n\n",
		sym.name, t.Name.String(), sym.pkg)

	pk, pkType := tablePK(t)
	if pk != "" {
		if pkType != "" {
			fmt.Fprintf(&b, "key: `%s` → route parameter type `%s`\n\n", pk, pkType)
		} else {
			fmt.Fprintf(&b, "key: `%s`\n\n", pk)
		}
	}

	b.WriteString("```dbml\n")
	fmt.Fprintf(&b, "Table %s {\n", t.Name.String())
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		mark := ""
		if col.Name.Name() == pk {
			mark = " [pk]"
		}
		fmt.Fprintf(&b, "\t%s %s%s\n", col.Name.Name(), col.Type.String(), mark)
	}
	b.WriteString("}\n```\n\n")
	b.WriteString("_The model name is nao's singularization of the table name; `[model:]` on the table overrides it (§V5.4.2)._")
	return b.String()
}

func pipelineHover(sym voltSym, def voltDef) string {
	p := def.pipe
	if p == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Pipeline `%s`** — %d plug(s), outermost first\n\n```volt\n", sym.name, len(p.Plugs))
	fmt.Fprintf(&b, "Pipeline %s {\n", sym.name)
	for _, plug := range p.Plugs {
		fmt.Fprintf(&b, "\tuse %s\n", plug.Ref.String())
	}
	b.WriteString("}\n```")
	return b.String()
}

// tablePK returns the single-column primary key and the Go type it maps
// to, mirroring the checker's rule (§V5.4.3); "" when there is none or
// the key is composite.
func tablePK(t *ast.Table) (name, goType string) {
	var names []string
	for _, item := range t.Body {
		col, ok := item.(*ast.Column)
		if !ok {
			continue
		}
		isPK := col.Settings.Get("pk") != nil || col.Settings.Get("primary key") != nil
		for _, f := range col.LegacyFlags {
			if strings.EqualFold(f.Name(), "pk") {
				isPK = true
			}
		}
		if isPK {
			names = append(names, col.Name.Name())
		}
	}
	if len(names) != 1 {
		return "", ""
	}
	name = names[0]
	for _, item := range t.Body {
		if col, ok := item.(*ast.Column); ok && col.Name.Name() == name {
			if gt, known := golang.GoTypeName(strings.ToLower(col.Type.Name.String())); known {
				goType = gt
			}
		}
	}
	return name, goType
}
