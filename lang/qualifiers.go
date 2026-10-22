package lang

import (
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

type qualKind int

const (
	qualImport qualKind = iota
	qualRuntime
	qualSelf
	qualUnknown
)

func qualifierResolve(qual, own string, imports map[string]string) (string, qualKind) {
	if qual == "volt" {
		return "", qualRuntime
	}
	if path, ok := imports[qual]; ok {
		return path, qualImport
	}
	if qual == own {
		return "", qualSelf
	}
	return "", qualUnknown
}

func selfQualified(pos token.Position, qual, name string) diag.Diagnostic {
	return diag.Errorf(pos, "spec/imports", "%s.%s qualifies a declaration with this package's own name: a package names its own declarations bare, so write %s (§imports)", qual, name, name)
}
