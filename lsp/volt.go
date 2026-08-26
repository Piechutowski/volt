// Volt-layer support. Volt is one language in three layers (SPEC.md
// §V0), and this is one server for all of it: the shared parser already
// reads every .volt file, so the only question is which checker's
// diagnostics to publish. A file that belongs to a Volt project — a
// volt.mod root above it (§V1.1) — is checked as part of that whole
// project through the lang package, with the open buffer overlaid over
// the disk; a file with no project keeps the single-file DBML pass.
package lsp

import (
	"path/filepath"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// voltProjectDiags runs the project-level pipeline (lang.Load, Check,
// Vet) for the document and returns the diagnostics that point into it.
// ok is false when the document is not part of any Volt project — no
// volt.mod above it, the project cannot be loaded, or the loader never
// read this file (it reads only .volt files and skips dot, underscore
// and nested-module directories, §V1.6) — and the caller then falls
// back to the single-file DBML diagnostics.
//
// Projects are re-loaded and re-checked on every edit. That is the
// simple, obviously-correct v0: Volt projects are small (text files a
// human wrote), and lang.Check is a few milliseconds at that size.
func (d *Document) voltProjectDiags() ([]diag.Diagnostic, bool) {
	path := pathFromURI(d.URI)
	if !filepath.IsAbs(path) {
		d.vindex = nil
		return nil, false
	}
	root, err := lang.FindRoot(filepath.Dir(path))
	if err != nil {
		d.vindex = nil
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
	pr, err := lang.LoadOverlay(root, overlay)
	if err != nil {
		d.vindex = nil
		return nil, false
	}
	member := false
	for _, pkg := range pr.Packages {
		for _, f := range pkg.Files {
			if f.Name == path {
				member = true
			}
		}
	}
	if !member {
		d.vindex = nil
		return nil, false
	}
	diags := lang.Check(pr)
	// After Check: it is what resolves each package's imports, which
	// the index needs to follow a `db.Post` qualifier to its package.
	d.vindex = buildVoltIndex(pr, overlay)
	// Vet advice only on top of a clean check, matching the single-file
	// policy: style notes stacked on hard errors are noise while typing.
	if !diag.HasErrors(diags) {
		diags = append(diags, lang.Vet(pr)...)
	}
	// Publish per document: findings positioned in this file. A
	// conflict with another file still shows here whenever this file
	// holds one of its ends, because the checker names both positions.
	// volt.mod is never open in an editor, so its problems (which the
	// messages name explicitly) surface at the top of every project
	// file instead of nowhere.
	modPath := filepath.Join(root, lang.ModFile)
	var mine []diag.Diagnostic
	for _, dg := range diags {
		switch dg.Pos.Filename {
		case path:
			mine = append(mine, dg)
		case modPath:
			dg.Pos = token.Position{Filename: path, Line: 1, Column: 1}
			mine = append(mine, dg)
		}
	}
	return mine, true
}
