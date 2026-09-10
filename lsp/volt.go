// Volt-layer support. Volt is one language in three layers (docs/spec.md
// §V0), and this is one server for all of it: the shared parser already
// reads every .volt file, so the only question is which checker's
// diagnostics to publish. A file that belongs to a Volt project — a
// go.mod root above it (§V1.1) — is checked as part of that whole
// project through the lang package, with the open buffer overlaid over
// the disk; a file with no project keeps the single-file DBML pass.
package lsp

import (
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// voltProjectDiags runs the project-level pipeline (lang.Load, Check,
// Vet) for the document and returns the diagnostics that point into it.
// ok is false when the document is not part of any Volt project — no
// go.mod above it, the project cannot be loaded, or the loader never
// read this file (it reads only .volt files and skips dot, underscore
// and nested-module directories, §V1.6) — and the caller then falls
// back to the single-file DBML diagnostics.
//
// The project is loaded and checked through the root's lang.Session
// when the document has one (D79): an edit then costs one parse and
// the checks of the packages that could see it. The server runs this
// in the background, once per project per quiet moment (analysis.go);
// a document on its own runs it here.
func (d *Document) voltProjectDiags() ([]diag.Diagnostic, bool) {
	path := pathFromURI(d.URI)
	root, ok := projectRootOf(path)
	if !ok {
		d.vindex = nil
		d.vpkg = nil
		return nil, false
	}
	// Every open buffer overlays its saved file, not just this one:
	// cross-file analysis must see the sibling the user is editing in
	// the next tab, unsaved edits included. This document last, so its
	// current text always wins.
	overlay := map[string]string{}
	if d.Siblings != nil {
		for p, text := range d.Siblings() {
			overlay[p] = text
		}
	}
	overlay[path] = d.Text
	var session *lang.Session
	if d.Session != nil {
		session = d.Session(root)
	}
	res := projectAnalyze(root, overlay, session)
	if res == nil || res.packageOf(path) == nil {
		d.vindex = nil
		d.vpkg = nil
		return nil, false
	}
	d.vpkg = res.packageOf(path)
	d.vindex = res.vindex
	return res.docDiags(path), true
}
