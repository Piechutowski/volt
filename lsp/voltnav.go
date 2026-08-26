// Project-wide navigation for Volt-layer symbols. The single-file index
// (index.go) resolves DBML names inside one document; this one spans the
// project, so `model: db.Post` in app/routes.volt reaches `Table posts`
// in db/schema.volt, and asking for references on that table finds every
// resources declaration that names it.
package lsp

import (
	"os"
	"strings"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/nao/edbml/ast"
	"github.com/Piechutowski/volt/nao/edbml/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
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

// voltIndex is the whole project's Volt-layer symbol graph.
type voltIndex struct {
	defs  map[voltSym]voltSpan
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
	ix := &voltIndex{defs: map[voltSym]voltSpan{}, texts: overlay}

	for path, pkg := range pr.Packages {
		for _, d := range pkg.Merged().Decls {
			switch d := d.(type) {
			case *ast.Table:
				name, err := golang.ModelName(d)
				if err != nil {
					continue
				}
				ix.define(voltSym{"table", path, name}, spanOf(d.Name))
			case *ast.Pipeline:
				ix.define(voltSym{"pipeline", path, d.Name.Name()}, spanOf(d.Name))
			}
		}
		// A package "declaration" is its first file, so an import can
		// jump somewhere useful.
		if len(pkg.Files) > 0 {
			first := pkg.Files[0]
			ix.defs[voltSym{"package", path, ""}] = voltSpan{
				file: first.Name,
				pos:  token.Position{Filename: first.Name, Line: 1, Column: 1},
				end:  token.Position{Filename: first.Name, Line: 1, Column: 1},
			}
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

func (ix *voltIndex) define(sym voltSym, sp voltSpan) {
	if _, dup := ix.defs[sym]; dup {
		return // duplicate declaration: the checker reports it
	}
	ix.defs[sym] = sp
	ix.refs = append(ix.refs, voltRef{sp, sym, true})
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
		case "model":
			switch v := s.Value.(type) {
			case *ast.Ident: // Model — this package
				ix.refs = append(ix.refs, voltRef{spanOf(v), voltSym{"table", path, v.Name()}, false})
			case *ast.EnumConst: // pkg.Model — an imported package
				target, ok := pkg.Imports[v.Enum.Name()]
				if !ok {
					continue
				}
				ix.refs = append(ix.refs, voltRef{spanOf(v), voltSym{"table", target, v.Value.Name()}, false})
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
	sp, ok := d.vindex.defs[ref.sym]
	if !ok {
		return nil
	}
	return d.vindex.location(sp)
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
