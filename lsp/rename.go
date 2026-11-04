package lsp

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

type renameEdit struct {
	file       string
	start, end int
	text       string
}

// Rename renames the symbol at pos (editor.md section 7): every
// occurrence spelled like the identifier under the cursor, the relation
// a foreign-key column names with the column, and nothing when the
// renamed project, or file, would check with more errors than it does,
// except at a Go reference, whose Go side gopls renames.
func (d *Document) Rename(pos protocol.Position, newName string) (*protocol.WorkspaceEdit, error) {
	if !validIdentifier(newName) {
		return nil, fmt.Errorf("%q is not a valid Volt identifier", newName)
	}
	edits, goRef, err := d.renameEdits(pos, newName)
	if err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("nothing renameable at this position")
	}
	if !goRef {
		if err := d.renameChecks(edits, newName); err != nil {
			return nil, err
		}
	}
	changes := map[protocol.DocumentUri][]protocol.TextEdit{}
	for _, e := range edits {
		text := d.Text
		if e.file != pathFromURI(d.URI) {
			text = d.vindex.text(e.file)
		}
		uri := protocol.DocumentUri("file://" + e.file)
		if e.file == pathFromURI(d.URI) {
			uri = d.URI
		}
		changes[uri] = append(changes[uri], protocol.TextEdit{
			Range:   protocol.Range{Start: offsetToLSP(text, e.start), End: offsetToLSP(text, e.end)},
			NewText: e.text,
		})
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func (d *Document) renameEdits(pos protocol.Position, newName string) (edits []renameEdit, goRef bool, err error) {
	path, off := pathFromURI(d.URI), d.FromLSP(pos)
	if d.vindex != nil {
		if ref := d.vindex.at(path, off); ref != nil && ref.sym.kind != "package" && ref.sym.kind != "param" {
			if def := d.vindex.defs[ref.sym]; ref.sym.kind == "relation" && def.column != "" {
				return nil, false, relationColumnRefusal(ref.text, def.column)
			}
			return d.vindex.renameEdits(ref.sym, ref.text, newName), ref.sym.kind == "gofunc", nil
		}
	}
	ix := d.Index
	if d.vpkg != nil && d.vpkg.Schema() != nil {
		ix = NewIndex(d.vpkg.Merged(), d.vpkg.Schema())
	}
	occ := ix.at(path, off)
	if occ == nil {
		return nil, false, nil
	}
	if col := ix.RelationColumn(occ.ID); col != "" {
		return nil, false, relationColumnRefusal(occ.ID.Name, col)
	}
	spelling := occ.Ident.Name()
	seen := map[string]bool{}
	for _, o := range ix.OccurrencesOf(occ.ID) {
		file, start := o.Ident.Pos().Filename(), int(o.Ident.Tok.Pos.Offset())
		if file == "" {
			file = path
		}
		key := fmt.Sprintf("%s:%d", file, start)
		if o.Ident.Name() != spelling || seen[key] {
			continue
		}
		seen[key] = true
		edits = append(edits, renameEdit{file, start, int(o.Ident.Tok.End().Offset()), newName})
	}
	if occ.ID.Kind == SymColumn {
		edits = append(edits, d.relationFollows(occ.ID, spelling, newName)...)
	}
	return edits, false, nil
}

// at is the occurrence at offset of the file path, an index over one
// file holding no other.
func (ix *Index) at(path string, offset int) *Occurrence {
	for i := range ix.Occs {
		o := &ix.Occs[i]
		if file := o.Ident.Pos().Filename(); file != "" && file != path {
			continue
		}
		if tok := o.Ident.Tok; int(tok.Pos.Offset()) <= offset && offset <= int(tok.End().Offset()) {
			return o
		}
	}
	return nil
}

func relationColumnRefusal(relation, column string) error {
	return fmt.Errorf("relation %s takes its name from the column %s; rename the column, or give its ref as: to name it otherwise (§relationships-ref)", relation, column)
}

// renameEdits are the edits that rename every occurrence of sym spelled
// spelling, each once.
func (ix *voltIndex) renameEdits(sym voltSym, spelling, newName string) []renameEdit {
	var edits []renameEdit
	seen := map[string]bool{}
	for _, r := range ix.refs {
		if r.sym != sym || r.text != spelling {
			continue
		}
		key := fmt.Sprintf("%s:%d", r.edit.file, r.edit.pos.Offset())
		if seen[key] {
			continue
		}
		seen[key] = true
		edits = append(edits, renameEdit{r.edit.file, int(r.edit.pos.Offset()), int(r.edit.end.Offset()), newName})
	}
	return edits
}

// relationFollows renames, with the column col of a table or a partial,
// every use of the forward relation the column names (spec
// §relationships-ref): a relation over that one column whose
// ref gives it no as: takes the column's name without its _id, so it
// takes the new one's.
func (d *Document) relationFollows(col SymbolID, oldName, newName string) []renameEdit {
	info := d.Info
	pkgPath := ""
	if d.vpkg != nil {
		if s := d.vpkg.Schema(); s != nil {
			info, pkgPath = s, d.vpkg.Path
		}
	}
	if info == nil {
		return nil
	}
	var tables []string
	if key, ok := strings.CutPrefix(col.Container, tableContainer); ok {
		tables = append(tables, key)
	} else if partial, ok := strings.CutPrefix(col.Container, "partial:"); ok {
		for _, ti := range info.Tables {
			if cd := ti.Column(oldName); cd != nil && cd.Partial != nil && cd.Partial.Name.Name() == partial {
				tables = append(tables, ti.Key)
			}
		}
	}
	renamed := check.RelationStem(newName)
	var edits []renameEdit
	for _, key := range tables {
		for _, r := range info.Relations(key) {
			named, column := relationNaming(r)
			if !r.Forward || named != nil || column != oldName || r.Name == renamed {
				continue
			}
			if d.vindex != nil {
				edits = append(edits, d.vindex.renameEdits(voltSym{"relation", pkgPath, key + "." + r.Name}, r.Name, renamed)...)
				continue
			}
			for _, o := range d.Index.OccurrencesOf(SymbolID{SymRelation, key, r.Name}) {
				if o.Ident.Name() == r.Name {
					edits = append(edits, renameEdit{pathFromURI(d.URI), int(o.Ident.Tok.Pos.Offset()), int(o.Ident.Tok.End().Offset()), renamed})
				}
			}
		}
	}
	return edits
}

// renameChecks refuses a rename whose result checks with more errors
// than the text it edits: the project's when the document is in one,
// the document's own otherwise, naming the first error it adds, so a
// name that makes a relation ambiguous, or that a use the rename cannot
// reach still spells, never lands.
func (d *Document) renameChecks(edits []renameEdit, newName string) error {
	path := pathFromURI(d.URI)
	if d.vindex == nil {
		after := editsApply(d.Text, edits)
		before, now := fileErrors(path, d.Text), fileErrors(path, after)
		return renameRefusal(newName, before, now)
	}
	root, ok := projectRootOf(path)
	if !ok {
		return nil
	}
	overlay := maps.Clone(d.vindex.texts)
	if overlay == nil {
		overlay = map[string]string{}
	}
	byFile := map[string][]renameEdit{}
	for _, e := range edits {
		byFile[e.file] = append(byFile[e.file], e)
	}
	for file, es := range byFile {
		overlay[file] = editsApply(d.vindex.text(file), es)
	}
	return renameRefusal(newName, projectErrors(root, d.vindex.texts), projectErrors(root, overlay))
}

func renameRefusal(newName string, before, after []diag.Diagnostic) error {
	if len(after) <= len(before) {
		return nil
	}
	known := map[string]bool{}
	for _, dg := range before {
		known[dg.Msg] = true
	}
	first := after[0]
	for _, dg := range after {
		if !known[dg.Msg] {
			first = dg
			break
		}
	}
	return fmt.Errorf("renaming to %s leaves %s: %s", newName, first.Pos, first.Msg)
}

func editsApply(text string, edits []renameEdit) string {
	edits = slices.Clone(edits)
	slices.SortFunc(edits, func(a, b renameEdit) int { return b.start - a.start })
	for _, e := range edits {
		if e.start < 0 || e.end > len(text) || e.start > e.end {
			continue
		}
		text = text[:e.start] + e.text + text[e.end:]
	}
	return text
}

func fileErrors(path, text string) []diag.Diagnostic {
	file, diags := parser.ParseFile(path, text)
	_, semDiags := check.File(file)
	return errorsOf(append(diags, semDiags...))
}

func projectErrors(root string, overlay map[string]string) []diag.Diagnostic {
	pr, err := lang.LoadOverlay(disk.Source{}, root, overlay)
	if err != nil {
		return nil
	}
	return errorsOf(lang.Check(pr))
}

func errorsOf(diags []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, dg := range diags {
		if dg.Severity == diag.Error {
			out = append(out, dg)
		}
	}
	diag.Sort(out)
	return out
}
