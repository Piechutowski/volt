// Volt-layer productions (spec Part II, docs/spec.md): the
// package clause, Go-style imports, pipelines, scopes, routes and
// resources. One method per production, in the style of the schema layer.
package parser

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

var voltVerbs = foldMap(Verbs)

func isVerb(v string) bool { return voltVerbs[strings.ToLower(v)] != "" }

func isPut(v string) bool { return strings.EqualFold(v, "put") }

func (p *parser) contiguous() bool {
	t := p.cur()
	return !t.SpBefore && !t.NLBefore
}

func (p *parser) packageClause() (d *ast.PackageClause) {
	p.lineBegin("package clause (§projects-and-packages)")
	d = &ast.PackageClause{PackagePos: p.next().Pos}
	d.Name = p.ident("package clause (§projects-and-packages)")
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			p.topLevelSync()
		}
	}()
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		p.errorf(p.cur(), "a package clause takes no settings list: a package carries no note (§notes), and its doc comment is written in a hand-written Go file of the package (§notes-become-doc-comments)")
		p.settingList()
	}
	p.endOfLine("package clause (§projects-and-packages)")
	return d
}

func (p *parser) importDecl() *ast.ImportDecl {
	d := &ast.ImportDecl{ImportPos: p.next().Pos}
	p.expect(token.LPAREN, "import declaration (§imports)")
	for !p.at(token.RPAREN) && !p.at(token.RBRACE) && !p.at(token.EOF) {
		if p.itemTo(func() { d.Specs = append(d.Specs, p.importSpec()) }, token.RPAREN, token.RBRACE) {
			d.Incomplete = true
		}
	}
	d.Rparen = p.expect(token.RPAREN, "import declaration (§imports)").End()
	if len(d.Specs) == 0 && !d.Incomplete {
		p.errorf(p.toks[p.pos-1], "empty import declaration (§imports)")
	}
	return d
}

func (p *parser) importSpec() *ast.ImportSpec {
	spec := &ast.ImportSpec{}
	p.lineBegin("import specifier (§imports)")
	first := p.ident("import path (§imports)")
	if p.at(token.IDENT) && !p.cur().NLBefore {
		spec.Alias = first
		first = p.ident("import path (§imports)")
	}
	spec.Path = []*ast.Ident{first}
	for p.at(token.SLASH) && p.contiguous() {
		p.next()
		if !p.at(token.IDENT) || !p.contiguous() {
			p.fail(p.cur(), "import path segments are identifiers separated by '/' with no spaces (§imports)")
		}
		spec.Path = append(spec.Path, p.ident("import path (§imports)"))
	}
	p.endOfLine("import specifier (§imports)")
	return spec
}

func (p *parser) pipeline() *ast.Pipeline {
	d := &ast.Pipeline{PipelinePos: p.next().Pos}
	d.Name = p.ident("pipeline name (§pipelines)")
	p.expect(token.LBRACE, "pipeline (§pipelines)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if plug := p.plugLine(); plug != nil {
			d.Plugs = append(d.Plugs, plug)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "pipeline (§pipelines)").Pos
	return d
}

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
		p.fail(p.cur(), "expected 'use' plug line in pipeline body (§pipelines), found %s", p.cur())
	}
	p.lineBegin("plug line (§pipelines)")
	plug = &ast.Plug{UsePos: p.kwTake("use").Pos}
	plug.Ref = p.goRef("plug reference (§pipelines)")
	p.endOfLine("plug line (§pipelines)")
	return plug
}

func (p *parser) goRef(ctx string) *ast.GoRef {
	first := p.ident(ctx)
	if p.at(token.DOT) {
		p.next()
		return &ast.GoRef{Parts: []*ast.Ident{first, p.ident(ctx)}}
	}
	return &ast.GoRef{Parts: []*ast.Ident{first}}
}

func (p *parser) scope() *ast.Scope {
	d := &ast.Scope{ScopePos: p.kwTake("scope").Pos}
	d.Path = p.routePath()
	d.Settings = p.settingsAt("scope")
	p.expect(token.LBRACE, "scope (§scopes-and-routes)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.scopeItem(); item != nil {
			d.Items = append(d.Items, item)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "scope (§scopes-and-routes)").Pos
	return d
}

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
		p.fail(t, "expected a route, resources or nested scope in scope body (§scopes-and-routes), found %s", t)
	}
	switch {
	case p.atKw("scope"):
		return p.scope()
	case p.atKw("resources"):
		return p.resources()
	case p.atKw("dataset"):
		return p.dataset()
	case p.atKw("load"):
		return p.scopeLoad()
	case isVerb(t.Val), isPut(t.Val):
		return p.route()
	default:
		p.fail(t, "expected a verb (get, post, ...), 'resources', 'dataset', 'load' or 'scope' in scope body (§scopes-and-routes), found %q", t.Val)
		return nil
	}
}

func (p *parser) scopeLoad() *ast.ScopeLoad {
	p.lineBegin("scope load (§scope-loads)")
	d := &ast.ScopeLoad{LoadPos: p.kwTake("load").Pos}
	d.Name = p.ident("scope load name (§scope-loads)")
	d.Ref = p.goRef("scope load function (§scope-loads)")
	p.endOfLine("scope load (§scope-loads)")
	return d
}

func (p *parser) dataset() *ast.Dataset {
	p.lineBegin("dataset (§datasets)")
	d := &ast.Dataset{DatasetPos: p.kwTake("dataset").Pos}
	d.Name = p.ident("dataset select name (§datasets)")
	if p.at(token.DOT) {
		p.next()
		d.Pkg = d.Name
		d.Name = p.ident("dataset select name (§datasets)")
	}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.endOfLine("dataset (§datasets)")
	return d
}

func (p *parser) route() *ast.Route {
	p.lineBegin("route (§scopes-and-routes)")
	r := &ast.Route{}
	if isPut(p.cur().Val) {
		r.VerbTok = p.next()
		r.VerbTok.Val = "put"
	} else {
		r.VerbTok = p.kwTake(voltVerbs[strings.ToLower(p.cur().Val)])
	}
	r.Path = p.routePath()
	r.Handler = p.goRef("route handler (§scopes-and-routes)")
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		r.Settings = p.settingList()
	}
	if p.at(token.LBRACE) && !p.cur().NLBefore {
		p.lineClear()
		p.next()
		for !p.at(token.RBRACE) && !p.at(token.EOF) {
			if ld := p.load(); ld != nil {
				r.Loads = append(r.Loads, ld)
			}
		}
		r.Rbrace = p.expect(token.RBRACE, "load block (§route-loads)").End()
	}
	p.endOfLine("route (§scopes-and-routes)")
	return r
}

func (p *parser) load() (ld *ast.Load) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			ld = nil
			p.lineSync()
		}
	}()
	ld = &ast.Load{Name: p.ident("load name (§route-loads)")}
	ld.Call = p.queryCall(p.goRef("load query (§route-loads)"), "load (§route-loads)")
	p.endOfLine("load (§route-loads)")
	return ld
}

// queryCall parses the argument list of a query call after its reference
// (spec §route-loads): a load's query, or a body: setting's.
func (p *parser) queryCall(ref *ast.GoRef, ctx string) *ast.QueryCall {
	call := &ast.QueryCall{Ref: ref}
	p.expect(token.LPAREN, "query call in "+ctx)
	for !p.at(token.RPAREN) {
		call.Args = append(call.Args, p.loadArg(ctx))
		if !p.at(token.COMMA) {
			break
		}
		p.next()
		if p.at(token.RPAREN) {
			p.fail(p.cur(), "expected an argument after the comma in %s, found %s", ctx, p.cur())
		}
	}
	call.Rparen = p.expect(token.RPAREN, "query call in "+ctx).End()
	return call
}

func (p *parser) loadArg(ctx string) ast.Node {
	t := p.cur()
	switch t.Kind {
	case token.STRING, token.NUMBER:
		return p.litOf(p.next())
	case token.MINUS:
		p.next()
		return &ast.NegNumber{MinusPos: t.Pos, Num: p.litOf(p.expect(token.NUMBER, "argument in "+ctx))}
	case token.IDENT:
		return p.ident("argument in " + ctx)
	}
	p.fail(t, "expected a name or a literal as an argument in %s, found %s", ctx, t)
	return nil
}

func (p *parser) resources() *ast.Resources {
	p.lineBegin("resources (§resources)")
	d := &ast.Resources{ResourcesPos: p.kwTake("resources").Pos}
	d.Name = p.ident("resources model name (§resources)")
	if p.at(token.DOT) {
		p.next()
		d.Pkg = d.Name
		d.Name = p.ident("resources model name (§resources)")
	}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
		for _, s := range d.Settings.Settings {
			if l, ok := s.Value.(*ast.IdentList); ok && (s.Name == "only" || s.Name == "except") {
				for _, id := range l.Names {
					p.wordFold(id, ResourceActions)
				}
			}
		}
	}
	p.endOfLine("resources (§resources)")
	return d
}

func (p *parser) routePath() *ast.RoutePath {
	slash := p.expect(token.SLASH, "route path (§route-paths)")
	rp := &ast.RoutePath{SlashPos: slash.Pos}
	rp.SetEnd(slash.End())
	for {
		if !p.contiguous() {
			break
		}
		switch p.cur().Kind {
		case token.IDENT:
			seg := &ast.Segment{Kind: ast.SegLit, Name: p.ident("path segment (§route-paths)")}
			seg.MarkPos = seg.Name.Pos()
			rp.Segments = append(rp.Segments, seg)
		case token.COLON:
			mark := p.next()
			if !p.at(token.IDENT) || !p.contiguous() {
				p.fail(p.cur(), "':' must be followed by a parameter name (§route-paths)")
			}
			seg := &ast.Segment{Kind: ast.SegParam, MarkPos: mark.Pos, Name: p.ident("path parameter (§route-paths)")}
			if p.at(token.LPAREN) && p.contiguous() {
				p.next()
				if !p.at(token.IDENT) || !p.contiguous() {
					p.fail(p.cur(), "parameter type is written ':name(type)' with no spaces (§route-paths)")
				}
				seg.Type = p.ident("parameter type (§route-paths)")
				p.wordFold(seg.Type, ParamTypes)
				if !p.at(token.RPAREN) || !p.contiguous() {
					p.fail(p.cur(), "parameter type is written ':name(type)' with no spaces (§route-paths)")
				}
				seg.Rparen = p.next().End()
			}
			if p.at(token.DOT) && p.contiguous() {
				for i := 0; i < 3; i++ {
					if !p.at(token.DOT) || !p.contiguous() {
						p.fail(p.cur(), "wildcard ellipsis is spelled ':name...' (§route-paths)")
					}
					dot := p.next()
					seg.SetEnd(dot.End())
				}
				if seg.Type != nil {
					p.fail(p.toks[p.pos-1], "a wildcard cannot carry a type; ':name...' always captures a string (§route-paths)")
				}
				seg.Kind = ast.SegWild
			}
			rp.Segments = append(rp.Segments, seg)
		case token.SLASH:
			p.fail(p.cur(), "'/' must be followed by a path segment; routes match exactly, without a trailing slash (§route-paths)")
		default:
		}
		if len(rp.Segments) == 0 {
			break
		}
		last := rp.Segments[len(rp.Segments)-1]
		rp.SetEnd(last.End())
		if p.at(token.SLASH) && p.contiguous() {
			slash := p.next()
			if !p.contiguous() || (p.cur().Kind != token.IDENT && p.cur().Kind != token.COLON) {
				p.fail(slash, "'/' must be followed by a path segment; routes match exactly, without a trailing slash (§route-paths)")
			}
			continue
		}
		break
	}
	return rp
}

func (p *parser) groupDecl() *ast.Group {
	p.lineBegin("group declaration (§groups)")
	d := &ast.Group{GroupPos: p.next().Pos}
	d.Name = p.ident("group declaration (§groups)")
	switch {
	case p.at(token.LBRACE):
		p.next()
		for !p.at(token.RBRACE) && !p.at(token.EOF) {
			dropped := p.item(func() {
				name := p.ident("group member (§groups)")
				d.Terms = append(d.Terms, &ast.GroupTerm{Names: []*ast.Ident{name}})
				if !p.at(token.RBRACE) && !p.cur().NLBefore {
					p.fail(p.cur(), "group members are one per line (§groups)")
				}
			})
			if dropped {
				d.Incomplete = true
			}
		}
		d.EndPos = p.expect(token.RBRACE, "group declaration (§groups)").End()
		p.endOfLine("group declaration (§groups)")
	case p.at(token.EQ):
		p.next()
		d.Terms = append(d.Terms, p.groupTerm(false))
		for p.at(token.PLUS) || p.at(token.BACKSLASH) || p.at(token.MINUS) {
			if p.at(token.MINUS) {
				p.fail(p.cur(), "'-' is not a group operator: set difference is spelled '\\' (§groups)")
			}
			neg := !p.at(token.PLUS)
			p.next()
			d.Terms = append(d.Terms, p.groupTerm(neg))
		}
		d.EndPos = d.Terms[len(d.Terms)-1].End()
		p.endOfLine("group declaration (§groups)")
	default:
		p.fail(p.cur(), "expected '{' or '=' after the group name (§groups)")
	}
	return d
}

func (p *parser) groupTerm(neg bool) *ast.GroupTerm {
	t := &ast.GroupTerm{Neg: neg}
	if !p.at(token.LPAREN) {
		t.Names = append(t.Names, p.ident("group expression (§groups)"))
		return t
	}
	t.Lparen = p.next().Pos
	for {
		t.Names = append(t.Names, p.ident("group set member (§groups)"))
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	t.Rparen = p.expect(token.RPAREN, "group set (§groups)").End()
	return t
}

func (p *parser) predDecl() *ast.Pred {
	p.lineBegin("predicate declaration (§predicates)")
	d := &ast.Pred{PredPos: p.next().Pos}
	d.Name = p.ident("predicate declaration (§predicates)")
	p.expect(token.LBRACE, "predicate declaration (§predicates)")
	d.X = p.predExpr()
	d.Rbrace = p.expect(token.RBRACE, "predicate declaration (§predicates)").End()
	p.endOfLine("predicate declaration (§predicates)")
	return d
}

func (p *parser) predExpr() ast.PredExpr {
	x := p.predAnd()
	for p.atKw("or") {
		p.kwTake("or")
		x = &ast.PredBinary{Op: "or", X: x, Y: p.predAnd()}
	}
	return x
}

func (p *parser) predAnd() ast.PredExpr {
	x := p.predUnary()
	for p.atKw("and") {
		p.kwTake("and")
		x = &ast.PredBinary{Op: "and", X: x, Y: p.predUnary()}
	}
	return x
}

func (p *parser) predUnary() ast.PredExpr {
	if p.atKw("not") {
		pos := p.kwTake("not").Pos
		return &ast.PredNot{NotPos: pos, X: p.predUnary()}
	}
	return p.predPrimary()
}

func (p *parser) predPrimary() ast.PredExpr {
	t := p.cur()
	switch {
	case p.at(token.LPAREN):
		lp := p.next().Pos
		x := p.predExpr()
		rp := p.expect(token.RPAREN, "parenthesized predicate (§predicates)").End()
		return &ast.PredParen{Lparen: lp, X: x, Rparen: rp}
	case p.at(token.COLON), p.at(token.NUMBER), p.at(token.STRING), p.at(token.MINUS),
		(p.atKw("true") || p.atKw("false")) && p.compOpAt(1):
		x := p.operand()
		return p.comparisonTail(x)
	case p.atKw("exists") && p.peekKind(1) == token.IDENT:
		return p.predExists()
	case p.at(token.IDENT):
		col := p.colRef("predicate (§predicates)")
		switch {
		case p.compOpAt(0):
			return p.comparisonTail(col)
		case p.atKw("in"):
			return p.predIn(col)
		case p.atKw("like"):
			return p.predLike(col)
		case p.atKw("is"):
			return p.predNull(col)
		case col.Qual != nil:
			p.fail(p.cur(), "expected a comparison operator, in, like or is after the column %s (§predicates)", col.String())
			return nil
		default:
			return &ast.PredRef{Name: col.Name}
		}
	default:
		p.fail(t, "expected a predicate expression (§predicates), found %s", t)
		return nil
	}
}

func (p *parser) param() *ast.Param {
	colon := p.next().Pos
	if !p.at(token.IDENT) || !p.contiguous() {
		p.fail(p.cur(), "':' must be directly followed by a parameter name (§predicates)")
	}
	x := &ast.Param{ColonPos: colon, Name: p.ident("parameter (§predicates)")}
	if p.at(token.QUESTION) {
		if !p.contiguous() {
			p.fail(p.cur(), "'?' makes a parameter optional only directly after its name, :%s? (§predicates)", x.Name.Name())
		}
		p.next()
		x.Optional = true
	}
	return x
}

func (p *parser) predExists() ast.PredExpr {
	x := &ast.PredExists{ExistsPos: p.kwTake("exists").Pos}
	p.predKeywordCheck()
	x.Rel = p.ident("exists (§predicates)")
	x.Lparen = p.expect(token.LPAREN, "exists (§predicates)").Pos
	x.X = p.predExpr()
	x.Rparen = p.expect(token.RPAREN, "exists (§predicates)").End()
	return x
}

func (p *parser) predKeywordCheck() {
	t := p.cur()
	if t.Kind != token.IDENT {
		return
	}
	if _, ok := foldTo(t.Val, predWords); ok {
		p.fail(t, "%q is a keyword inside a predicate and names no column there (§predicates)", t.Val)
	}
}

func (p *parser) compOpAt(n int) bool {
	switch p.peekKind(n) {
	case token.EQ, token.NEQ, token.LT, token.LE, token.GT, token.GE:
		return true
	}
	return false
}

func (p *parser) comparisonTail(x ast.Operand) ast.PredExpr {
	if !p.compOpAt(0) {
		p.fail(p.cur(), "expected a comparison operator (= != < <= > >=) (§predicates)")
	}
	op := p.next().Kind
	return &ast.PredCompare{X: x, Op: op, Y: p.operand()}
}

func (p *parser) operand() ast.Operand {
	t := p.cur()
	switch {
	case p.at(token.COLON):
		return p.param()
	case p.at(token.NUMBER), p.at(token.STRING):
		return &ast.Lit{Tok: p.next()}
	case p.at(token.MINUS) && p.peekKind(1) == token.NUMBER:
		minus := p.next()
		num := p.next()
		num.Val = "-" + num.Val
		num.Pos = minus.Pos
		return &ast.Lit{Tok: num}
	case p.atKw("true"):
		return &ast.Lit{Tok: p.kwTake("true")}
	case p.atKw("false"):
		return &ast.Lit{Tok: p.kwTake("false")}
	case p.at(token.IDENT):
		return p.colRef("operand (§predicates)")
	default:
		p.fail(t, "expected a column, :param or literal (§predicates), found %s", t)
		return nil
	}
}

// colRef reads a column reference: a name, or relation.column
// (§predicates, §selects-over-groups); neither part is a predicate
// keyword.
func (p *parser) colRef(ctx string) *ast.ColRef {
	p.predKeywordCheck()
	name := p.ident(ctx)
	if !p.at(token.DOT) {
		return &ast.ColRef{Name: name}
	}
	p.next()
	p.predKeywordCheck()
	return &ast.ColRef{Qual: name, Name: p.ident(ctx)}
}

func (p *parser) projCol(ctx string) *ast.ColRef {
	name := p.ident(ctx)
	if !p.at(token.DOT) {
		return &ast.ColRef{Name: name}
	}
	p.next()
	return &ast.ColRef{Qual: name, Name: p.ident(ctx)}
}

func (p *parser) predIn(col *ast.ColRef) ast.PredExpr {
	p.kwTake("in")
	x := &ast.PredIn{Col: col}
	if p.at(token.COLON) {
		prm, ok := p.operand().(*ast.Param)
		if !ok {
			p.fail(p.toks[p.pos-1], "expected a :list parameter after 'in' (§predicates)")
			return x
		}
		x.List = prm
		return x
	}
	p.expect(token.LPAREN, "in list (§predicates)")
	for {
		lit, ok := p.operand().(*ast.Lit)
		if !ok {
			p.fail(p.toks[p.pos-1], "in items are literals (§predicates)")
		}
		x.Items = append(x.Items, lit)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	x.Rparen = p.expect(token.RPAREN, "in list (§predicates)").End()
	return x
}

func (p *parser) predLike(col *ast.ColRef) ast.PredExpr {
	p.kwTake("like")
	x := &ast.PredLike{Col: col}
	switch pat := p.operand().(type) {
	case *ast.Lit:
		if pat.Tok.Kind != token.STRING {
			p.fail(pat.Tok, "like takes a string pattern or :param (§predicates)")
		}
		x.Pattern = pat
	case *ast.Param:
		x.Pattern = pat
	default:
		p.fail(p.toks[p.pos-1], "like takes a string pattern or :param (§predicates)")
	}
	return x
}

func (p *parser) predNull(col *ast.ColRef) ast.PredExpr {
	p.kwTake("is")
	x := &ast.PredNull{Col: col}
	if p.atKw("not") {
		p.kwTake("not")
		x.Not = true
	}
	if !p.atKw("null") {
		p.fail(p.cur(), "expected 'null' after 'is' (§predicates)")
	}
	x.EndPos = p.kwTake("null").End()
	return x
}

func (p *parser) selectDecl() *ast.Select {
	p.lineBegin("select declaration (§selects-over-groups)")
	d := &ast.Select{SelectPos: p.next().Pos}
	d.Name = p.ident("select declaration (§selects-over-groups)")
	if p.at(token.LPAREN) {
		p.selectProjection(d)
	}
	if !p.atKw("for") {
		p.fail(p.cur(), "expected 'for' after the select name (§selects-over-groups)")
	}
	p.kwTake("for")
	d.Target = p.ident("select target (§selects-over-groups)")
	d.EndPos = d.Target.End()
	for p.atKw("join") {
		p.kwTake("join")
		rel := p.ident("join (§selects-over-groups)")
		d.Joins = append(d.Joins, rel)
		d.EndPos = rel.End()
	}
	if p.atKw("with") {
		p.kwTake("with")
		d.With = p.ident("with (§selects-over-groups)")
		d.EndPos = d.With.End()
	}
	if p.atKw("where") {
		p.kwTake("where")
		d.Where = p.predExpr()
		d.EndPos = d.Where.End()
	}
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
		d.EndPos = d.Settings.End()
	}
	p.endOfLine("select declaration (§selects-over-groups)")
	return d
}

func (p *parser) selectProjection(d *ast.Select) {
	d.Lparen = p.next().Pos
	if p.at(token.STAR) {
		d.Star = true
		p.next()
		for p.at(token.BACKSLASH) || p.at(token.MINUS) {
			if p.at(token.MINUS) {
				p.fail(p.cur(), "'-' is not a projection operator: exclusion is spelled '\\' (§selects-over-groups)")
			}
			p.next()
			if !p.at(token.LPAREN) {
				d.Cols = append(d.Cols, &ast.ColRef{Name: p.ident("excluded column (§selects-over-groups)")})
				continue
			}
			p.next()
			for {
				d.Cols = append(d.Cols, &ast.ColRef{Name: p.ident("excluded column (§selects-over-groups)")})
				if !p.at(token.COMMA) {
					break
				}
				p.next()
			}
			p.expect(token.RPAREN, "excluded column set (§selects-over-groups)")
		}
		if len(d.Cols) == 0 {
			p.fail(p.cur(), "a star projection needs at least one '\\ column' exclusion; drop the parens to select every column (§selects-over-groups)")
		}
	} else {
		for {
			d.Items = append(d.Items, p.projItem())
			if !p.at(token.COMMA) {
				break
			}
			p.next()
		}
	}
	d.Rparen = p.expect(token.RPAREN, "projection (§selects-over-groups)").End()
	d.EndPos = d.Rparen
}

// projItem reads one item of an explicit projection (§selects-over-groups):
// a column, an aggregate or an arithmetic expression, the derived ones
// followed by "as" and an alias.
func (p *parser) projItem() *ast.ProjItem {
	item := &ast.ProjItem{}
	if p.at(token.IDENT) && p.peekKind(1) == token.LPAREN {
		if _, ok := foldTo(p.cur().Val, AggregateWords); ok {
			agg := p.ident("aggregate (§selects-over-groups)")
			p.wordFold(agg, AggregateWords)
			item.Agg = agg
			p.expect(token.LPAREN, "aggregate (§selects-over-groups)")
			if !p.at(token.RPAREN) || agg.Name() != "count" {
				if p.at(token.RPAREN) {
					p.fail(p.cur(), "%s takes a column; only count() counts the rows (§selects-over-groups)", agg.Name())
					return item
				}
				item.Col = p.projCol("aggregate column (§selects-over-groups)")
			}
			item.EndPos = p.expect(token.RPAREN, "aggregate (§selects-over-groups)").End()
		}
	}
	if item.Agg == nil {
		x := p.arith()
		if x == nil {
			return item
		}
		if cr, ok := x.(*ast.ColRef); ok && !p.atKw("as") {
			item.Col, item.EndPos = cr, cr.End()
			return item
		}
		item.Expr, item.EndPos = x, x.End()
	}
	if !p.atKw("as") {
		p.fail(p.cur(), "a derived item takes an alias: write it as name (§selects-over-groups)")
		return item
	}
	p.kwTake("as")
	item.As = p.ident("alias (§selects-over-groups)")
	item.EndPos = item.As.End()
	return item
}

func (p *parser) arith() ast.ArithExpr {
	x := p.arithTerm()
	for x != nil && (p.at(token.PLUS) || p.at(token.MINUS)) {
		op := "+"
		if p.at(token.MINUS) {
			op = "-"
		}
		p.next()
		y := p.arithTerm()
		if y == nil {
			return nil
		}
		x = &ast.ArithBinary{X: x, Op: op, Y: y}
	}
	return x
}

func (p *parser) arithTerm() ast.ArithExpr {
	x := p.arithFactor()
	for x != nil && (p.at(token.STAR) || p.at(token.SLASH)) {
		op := "*"
		if p.at(token.SLASH) {
			op = "/"
		}
		p.next()
		y := p.arithFactor()
		if y == nil {
			return nil
		}
		x = &ast.ArithBinary{X: x, Op: op, Y: y}
	}
	return x
}

func (p *parser) arithFactor() ast.ArithExpr {
	t := p.cur()
	switch {
	case p.at(token.LPAREN):
		lp := p.next().Pos
		x := p.arith()
		if x == nil {
			return nil
		}
		rp := p.expect(token.RPAREN, "parenthesized expression (§selects-over-groups)").End()
		return &ast.ArithParen{Lparen: lp, X: x, Rparen: rp}
	case p.at(token.NUMBER):
		return &ast.Lit{Tok: p.next()}
	case p.at(token.MINUS) && p.peekKind(1) == token.NUMBER:
		minus := p.next()
		num := p.next()
		num.Val = "-" + num.Val
		num.Pos = minus.Pos
		return &ast.Lit{Tok: num}
	case p.at(token.COLON):
		return p.param()
	case p.at(token.IDENT):
		return p.projCol("projected column (§selects-over-groups)")
	default:
		p.fail(t, "expected a column, an aggregate, a number, a :param or a parenthesized expression in the projection (§selects-over-groups), found %s", t)
		return nil
	}
}
