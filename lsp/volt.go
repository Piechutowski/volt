// Volt-layer support. Volt is one language in three layers (docs/spec.md
// §layers-files-and-the-superset-rule), and this is one server for all of it. A file that belongs to a
// Volt project — a go.mod root above it (§projects-and-packages) — is checked as part of
// that whole project; a file with no project keeps the single-file DBML
// pass.
package lsp

import (
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

func (d *Document) voltProjectDiags() ([]diag.Diagnostic, bool) {
	path := pathFromURI(d.URI)
	root, ok := projectRootOf(path)
	if !ok {
		d.vindex = nil
		d.vpkg = nil
		return nil, false
	}
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
	res := projectAnalyze(root, overlay, session, nil)
	if res == nil || res.packageOf(path) == nil {
		d.vindex = nil
		d.vpkg = nil
		return nil, false
	}
	d.vpkg = res.packageOf(path)
	d.vindex = res.vindex
	return res.docDiags(path), true
}
