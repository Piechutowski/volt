// Dynamic-layer name analyzer: distinct schema
// declarations can demand one Go name; this rule surfaces the
// collision at vet time with both origins named. A collision with a
// bare declared type is the checker's error (spec §types), not a
// warning here.
package vet

type dynNameFold struct{ meta }

var dynName = &dynNameFold{meta{
	name: "dynname",
	doc:  "reports declarations whose generated dynamic-query names (column-handle sets, option wrappers) collide in Go package scope",
}}

func (fo *dynNameFold) File(p *FilePass) {
	for _, c := range p.Plan().DynNameCollisions() {
		if c.Type != nil {
			continue
		}
		p.Reportf(c.Pos, "%s and %s both generate the Go name %s; rename one (e.g. with [model: '...'])",
			c.First, c.Second, c.Name)
	}
}
