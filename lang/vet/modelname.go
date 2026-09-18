// Model-name analyzer — decision D10: Go models are the singular of the
// table name, derived by the deterministic inflector in the inflect
// package. When the inflector is only guessing, the schema should say the
// name out loud with the [model:] extension setting.
package vet

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/inflect"
)

func init() { register(modelName) }

type modelNameRule struct{ meta }

var modelName = &modelNameRule{meta{
	name: "modelname",
	doc:  "reports tables whose generated Go model name is an inflector guess not pinned by [model:]",
}}

func (r *modelNameRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	ti := f.Table
	if ti == nil {
		return out
	}
	t := ti.Decl
	if t.Settings.Get("model") != nil {
		return out
	}
	singular, ok := inflect.SingularLast(t.Name.Base())
	if !ok {
		out = append(out, r.warnf(t.Pos(), "cannot confidently singularize %q for its Go model name (would use %q); pin it with [model: '...']",
			t.Name.Base(), singular))
	}
	return out
}
