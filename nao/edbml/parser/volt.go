// Volt-layer productions (spec §V, SPEC.md at the repository root): the
// package clause, Go-style imports, pipelines, scopes, routes and
// resources. One method per production, in the style of the DBML layer.
package parser

import (
	"strings"

	"github.com/Piechutowski/volt/nao/edbml/ast"
	"github.com/Piechutowski/volt/nao/edbml/token"
)

// voltVerbs is the route verb set (spec §V4.2).
var voltVerbs = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true,
	"delete": true, "options": true, "head": true, "any": true,
}

func isVerb(v string) bool { return voltVerbs[strings.ToLower(v)] }

// contiguous reports whether the current token directly abuts the previous
// one — no whitespace, no line break. Route and import paths are single
// lexical islands (§V4.1.5), assembled from tokens but rejecting interior
// space.
func (p *parser) contiguous() bool {
	t := p.cur()
	return !t.SpBefore && !t.NLBefore
}

/* ===== package clause (§V1) ===== */

// packageClause = "package", name, newline (§V1.2).
func (p *parser) packageClause() *ast.PackageClause {
	d := &ast.PackageClause{PackagePos: p.next().Pos}
	d.Name = p.ident("package clause (§V1)")
	if d.Name.Quoted() {
		p.fail(p.toks[p.pos-1], "package name must be a plain identifier (§V1)")
	}
	p.endOfLine("package clause (§V1)")
	return d
}

/* ===== import declaration (§V2) ===== */

// importDecl = "import", "(", { import spec, newline }, ")" (§V2.1).
func (p *parser) importDecl() *ast.ImportDecl {
	d := &ast.ImportDecl{ImportPos: p.next().Pos}
	p.expect(token.LPAREN, "import declaration (§V2)")
	for !p.at(token.RPAREN) && !p.at(token.EOF) {
		d.Specs = append(d.Specs, p.importSpec())
	}
	d.Rparen = p.expect(token.RPAREN, "import declaration (§V2)").End()
	if len(d.Specs) == 0 {
		p.errorf(p.toks[p.pos-1], "empty import declaration (§V2)")
	}
	return d
}

// importSpec = [ alias ], import path, newline; import path = name,
// { "/", name } (§V2.2). An alias is present when two identifiers open
// the line: the first is the alias, the second starts the path.
func (p *parser) importSpec() *ast.ImportSpec {
	spec := &ast.ImportSpec{}
	first := p.ident("import path (§V2)")
	if p.at(token.IDENT) && !p.cur().NLBefore {
		spec.Alias = first
		first = p.ident("import path (§V2)")
	}
	spec.Path = []*ast.Ident{first}
	for p.at(token.SLASH) && p.contiguous() {
		p.next()
		if !p.at(token.IDENT) || !p.contiguous() {
			p.fail(p.cur(), "import path segments are identifiers separated by '/' with no spaces (§V2)")
		}
		spec.Path = append(spec.Path, p.ident("import path (§V2)"))
	}
	// Aliases and path segments name directories and qualify Go-bound
	// references: plain identifiers only (§V2.2, §V4.1.6).
	if spec.Alias != nil && spec.Alias.Quoted() {
		p.fail(spec.Alias.Tok, "import alias must be a plain (unquoted) identifier (§V2.2)")
	}
	for _, seg := range spec.Path {
		if seg.Quoted() {
			p.fail(seg.Tok, "import path segments are plain (unquoted) identifiers (§V2.2)")
		}
	}
	p.endOfLine("import specifier (§V2)")
	return spec
}

/* ===== Pipeline (§V3) ===== */

// pipeline = "Pipeline", name, "{", { plug }, "}" (§V3.1).
func (p *parser) pipeline() *ast.Pipeline {
	d := &ast.Pipeline{PipelinePos: p.next().Pos}
	d.Name = p.ident("Pipeline name (§V3)")
	p.expect(token.LBRACE, "Pipeline (§V3)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if plug := p.plugLine(); plug != nil {
			d.Plugs = append(d.Plugs, plug)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "Pipeline (§V3)").Pos
	return d
}

// plugLine parses one plug line, recovering to the next line so one bad
// plug does not abort the whole Pipeline body (matching scopeItem).
func (p *parser) plugLine() (plug *ast.Plug) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			plug = nil
			p.lineSync()
		}
	}()

	if !p.atKw("use") {
		p.fail(p.cur(), "expected 'use' plug line in Pipeline body (§V3), found %s", p.cur())
	}
	plug = &ast.Plug{UsePos: p.next().Pos}
	plug.Ref = p.goRef("plug reference (§V3)")
	p.endOfLine("plug line (§V3)")
	return plug
}

// goRef = name, [ ".", name ] (§V3.2).
func (p *parser) goRef(ctx string) *ast.GoRef {
	first := p.ident(ctx)
	if p.at(token.DOT) {
		p.next()
		return &ast.GoRef{Parts: []*ast.Ident{first, p.ident(ctx)}}
	}
	return &ast.GoRef{Parts: []*ast.Ident{first}}
}

/* ===== Scope, routes, resources (§V4, §V5) ===== */

// scope = "Scope", route path, [ settings ], "{", { scope item }, "}" (§V4.1).
func (p *parser) scope() *ast.Scope {
	d := &ast.Scope{ScopePos: p.next().Pos}
	d.Path = p.routePath()
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "Scope (§V4)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.scopeItem(); item != nil {
			d.Items = append(d.Items, item)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "Scope (§V4)").Pos
	return d
}

// scopeItem parses one Scope body item, recovering to the next line so a
// broken route does not abort the whole scope.
func (p *parser) scopeItem() (item ast.ScopeItem) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			item = nil
			p.lineSync()
		}
	}()

	t := p.cur()
	if t.Kind != token.IDENT {
		p.fail(t, "expected a route, resources or nested Scope in Scope body (§V4), found %s", t)
	}
	switch {
	case p.atKw("scope"):
		return p.scope()
	case p.atKw("resources"):
		return p.resources()
	case isVerb(t.Val) && !t.Quoted:
		return p.route()
	default:
		p.fail(t, "expected a verb (get, post, ...), 'resources' or 'Scope' in Scope body (§V4), found %q", t.Val)
		return nil
	}
}

// route = verb, route path, handler ref, [ settings ], newline (§V4.2).
func (p *parser) route() *ast.Route {
	r := &ast.Route{VerbTok: p.next()}
	r.Path = p.routePath()
	r.Handler = p.goRef("route handler (§V4)")
	if len(r.Handler.Parts) != 2 {
		p.fail(p.toks[p.pos-1], "route handler must be Controller.Action (§V4.3), found %q", r.Handler.String())
	}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		r.Settings = p.settingList()
	}
	p.endOfLine("route (§V4)")
	return r
}

// resources = "resources", name, [ settings ], newline (§V5.1).
func (p *parser) resources() *ast.Resources {
	d := &ast.Resources{ResourcesPos: p.next().Pos}
	d.Name = p.ident("resources table name (§V5)")
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.endOfLine("resources (§V5)")
	return d
}

// routePath = "/", [ segment, { "/", segment } ] with all tokens
// contiguous (§V4.1). A bare "/" is the root path.
func (p *parser) routePath() *ast.RoutePath {
	slash := p.expect(token.SLASH, "route path (§V4.1)")
	rp := &ast.RoutePath{SlashPos: slash.Pos}
	rp.SetEnd(slash.End())
	for {
		if !p.contiguous() {
			break
		}
		switch p.cur().Kind {
		case token.IDENT:
			seg := &ast.Segment{Kind: ast.SegLit, Name: p.ident("path segment (§V4.1)")}
			seg.MarkPos = seg.Name.Pos()
			rp.Segments = append(rp.Segments, seg)
		case token.COLON:
			mark := p.next()
			if !p.at(token.IDENT) || !p.contiguous() {
				p.fail(p.cur(), "':' must be followed by a parameter name (§V4.1)")
			}
			seg := &ast.Segment{Kind: ast.SegParam, MarkPos: mark.Pos, Name: p.ident("path parameter (§V4.1)")}
			if p.at(token.LPAREN) && p.contiguous() {
				p.next()
				// The annotation is part of the path's lexical island
				// (§V4.1.1): every token of ':name(type)' abuts.
				if !p.at(token.IDENT) || !p.contiguous() {
					p.fail(p.cur(), "parameter type is written ':name(type)' with no spaces (§V4.1)")
				}
				seg.Type = p.ident("parameter type (§V4.1)")
				if !p.at(token.RPAREN) || !p.contiguous() {
					p.fail(p.cur(), "parameter type is written ':name(type)' with no spaces (§V4.1)")
				}
				seg.Rparen = p.next().End()
			}
			// ":name..." — the rest-of-path wildcard (§V4.1.4). Written
			// with three contiguous '.' tokens; a type annotation cannot
			// combine with it (the wildcard's Go type is always string).
			if p.at(token.DOT) && p.contiguous() {
				for i := 0; i < 3; i++ {
					if !p.at(token.DOT) || !p.contiguous() {
						p.fail(p.cur(), "wildcard ellipsis is spelled ':name...' (§V4.1)")
					}
					dot := p.next()
					seg.SetEnd(dot.End())
				}
				if seg.Type != nil {
					p.fail(p.toks[p.pos-1], "a wildcard cannot carry a type; ':name...' always captures a string (§V4.1)")
				}
				seg.Kind = ast.SegWild
			}
			rp.Segments = append(rp.Segments, seg)
		default:
			p.fail(p.cur(), "invalid path segment (§V4.1): found %s", p.cur())
		}
		last := rp.Segments[len(rp.Segments)-1]
		rp.SetEnd(last.End())
		if p.at(token.SLASH) && p.contiguous() {
			slash := p.next()
			// Routes match exactly (§V4.4): a trailing '/' would be a
			// distinct, silently different pattern, so it is an error, as
			// is an empty segment ("//").
			if !p.contiguous() || (p.cur().Kind != token.IDENT && p.cur().Kind != token.COLON) {
				p.fail(slash, "'/' must be followed by a path segment; routes match exactly, without a trailing slash (§V4.1)")
			}
			continue
		}
		break
	}
	return rp
}
