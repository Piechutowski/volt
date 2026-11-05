package vet

import "github.com/Piechutowski/volt/lang/diag"

type packageDocRule struct{ meta }

var packageDoc = &packageDocRule{meta{
	name: "packagedoc",
	doc:  "reports a package doc comment in a hand-written Go file of a Volt package, which documents itself above a package clause of its .volt files",
}}

func (r *packageDocRule) Go(f *GoFile) []diag.Diagnostic {
	if f.File.Doc == nil {
		return nil
	}
	return []diag.Diagnostic{diag.Warningf(f.Pos(f.File.Doc), "vet/packagedoc",
		"a package doc comment in a Go file: the .volt files are the package's source, so write it above a package clause there, from where volt gen carries it into the generated code (§doc-comments)")}
}
