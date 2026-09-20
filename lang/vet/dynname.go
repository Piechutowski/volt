// Dynamic-layer name analyzer — feature DYN-7: the dynamic query layer
// (decisions D28-D30) mints package-scope Go names by concatenation
// (UserEmail, UserLimit), so distinct DBML declarations can demand one Go
// name. Generation refuses such schemas outright; this rule surfaces the
// collision at vet time with both origins named. The name derivation is
// the generator's own (nao/gen/golang), so rule and generator cannot drift.
package vet

// dynNameFold asks the plan, whose name base holds every model's
// names across plans (D99), for the collisions alone.
type dynNameFold struct{ meta }

var dynName = &dynNameFold{meta{
	name: "dynname",
	doc:  "reports declarations whose generated dynamic-query names (column-handle sets, option wrappers) collide in Go package scope",
}}

func (fo *dynNameFold) File(p *FilePass) {
	for _, c := range p.Plan().DynNameCollisions() {
		p.Reportf(c.Pos, "%s and %s both generate the Go name %s; rename one (e.g. with [model: '...'])",
			c.First, c.Second, c.Name)
	}
}
