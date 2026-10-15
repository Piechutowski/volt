// Go-reference navigation: the functions a Volt file names by rule live
// in the containing package's own Go files (§pipelines rule 2 plugs, §validation-checks rule 5
// checks).
package lsp

import (
	"path/filepath"
	"strings"

	"github.com/Piechutowski/volt/lang"
)

// goFuncHover renders the hover for a resolved (or missing) reference.
func goFuncHover(name, pkgName string, gf *lang.GoFunc) string {
	if gf == nil {
		return "`" + name + "` — no such function in package `" + pkgName + "`'s Go files yet; " +
			"declare it in this package (the diagnostic on the reference names the exact signature — §pipelines rule 2 for plugs, §validation-checks rule 5 for checks)"
	}
	var b strings.Builder
	b.WriteString("```go\n" + gf.Sig + "\n```\n")
	if gf.Doc != "" {
		b.WriteString("\n" + gf.Doc + "\n")
	}
	b.WriteString("\n*" + filepath.Base(gf.File) + "*")
	return b.String()
}
