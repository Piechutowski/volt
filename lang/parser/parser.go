// Package parser turns Volt source into an ast.File, mirroring go/parser.
//
// The parser is a hand-written recursive descent over the EBNF of the
// specification (docs/spec.md): one method per production. It is
// syntax-only: it accepts any setting name in any position, any value
// shape the token stream allows, and unresolved names; the check package
// judges meaning.
//
// On a syntax error the parser records a diagnostic and synchronizes to
// the next line or the enclosing brace. A file is parsed element by
// element (reuse.go): the scanner cuts it where an element begins (spec
// §line-structure-and-whitespace), and each piece is parsed on its own.
package parser

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// ParseFile parses one Volt source file. It always returns a (possibly
// partial) AST; diagnostics carry every lexical and syntax error found.
func ParseFile(filename, src string) (*ast.File, []diag.Diagnostic) {
	f, diags, _, _ := ParseFileReuse(filename, src, nil)
	return f, diags
}

type parser struct {
	line      bool
	lineDepth int
	lineFirst bool
	lineCtx   string

	toks  []token.Token
	pos   int
	diags []diag.Diagnostic
	buf   []token.Token

	idents   slab[ast.Ident]
	quals    slab[ast.QualName]
	columns  slab[ast.Column]
	types    slab[ast.TypeRef]
	settings slab[ast.Setting]
	lists    slab[ast.SettingList]
	lits     slab[ast.BasicLit]
}

type slab[T any] struct{ chunk []T }

func (s *slab[T]) new() *T {
	if len(s.chunk) == cap(s.chunk) {
		s.chunk = make([]T, 0, 256)
	}
	s.chunk = s.chunk[:len(s.chunk)+1]
	return &s.chunk[len(s.chunk)-1]
}

func (p *parser) identOf(t token.Token) *ast.Ident {
	id := p.idents.new()
	id.Tok = t
	return id
}

func (p *parser) litOf(t token.Token) *ast.BasicLit {
	l := p.lits.new()
	l.Tok = t
	return l
}

func (p *parser) qualOf(parts ...*ast.Ident) *ast.QualName {
	q := p.quals.new()
	q.Parts = parts
	return q
}

type bailout struct{}

func (p *parser) cur() token.Token { return p.toks[p.pos] }
func (p *parser) next() token.Token {
	t := p.toks[p.pos]
	if p.line {
		if t.NLBefore && p.lineDepth == 0 && !p.lineFirst {
			p.fail(t, "expected end of line after %s, found %s", p.lineCtx, t)
		}
		p.lineFirst = false
		switch t.Kind {
		case token.LBRACE, token.LBRACKET, token.LPAREN:
			p.lineDepth++
		case token.RBRACE, token.RBRACKET, token.RPAREN:
			p.lineDepth--
		}
	}
	p.pos++
	return t
}

func (p *parser) lineBegin(ctx string) {
	p.line, p.lineDepth, p.lineFirst, p.lineCtx = true, 0, true, ctx
}

func (p *parser) lineClear() { p.line = false }
func (p *parser) at(k token.Kind) bool {
	return p.cur().Kind == k
}

func (p *parser) peekNL(n int) bool {
	return p.pos+n < len(p.toks) && p.toks[p.pos+n].NLBefore
}

func (p *parser) peekKind(n int) token.Kind {
	if p.pos+n >= len(p.toks) {
		return token.EOF
	}
	return p.toks[p.pos+n].Kind
}

func (p *parser) atKw(kw string) bool {
	t := p.cur()
	return t.Kind == token.IDENT && strings.EqualFold(t.Val, kw)
}

func (p *parser) errorf(t token.Token, format string, args ...any) {
	p.diags = append(p.diags, diag.Errorf(t.Pos, "syntax", format, args...))
}

func (p *parser) fail(t token.Token, format string, args ...any) {
	p.errorf(t, format, args...)
	panic(bailout{})
}

func (p *parser) expect(k token.Kind, ctx string) token.Token {
	if !p.at(k) {
		p.fail(p.cur(), "expected %s in %s, found %s", k, ctx, p.cur())
	}
	return p.next()
}

func (p *parser) endOfLine(ctx string) {
	p.line = false
	if p.at(token.RBRACE) || p.at(token.EOF) || p.cur().NLBefore {
		return
	}
	p.fail(p.cur(), "expected end of line after %s, found %s", ctx, p.cur())
}

func (p *parser) ident(ctx string) *ast.Ident {
	t := p.expect(token.IDENT, ctx)
	if c := t.Val[0]; '0' <= c && c <= '9' {
		p.errorf(t, "%s begins with a digit: a name is a Go identifier, a letter or underscore followed by letters, digits and underscores (§identifiers)", t.Val)
	}
	return p.identOf(t)
}

func (p *parser) qualName(ctx string) *ast.QualName {
	first := p.ident(ctx)
	if p.at(token.DOT) {
		p.next()
		return p.qualOf(first, p.ident(ctx))
	}
	return p.qualOf(first)
}

func (p *parser) file(name string) *ast.File {
	f := &ast.File{Name: name}
	for !p.at(token.EOF) {
		if d := p.decl(); d != nil {
			f.Decls = append(f.Decls, d)
		}
	}
	f.EOF = p.cur().Pos
	return f
}

func (p *parser) decl() (d ast.Decl) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			d = nil
			p.topLevelSync()
		}
	}()

	t := p.cur()
	if t.Kind != token.IDENT {
		p.fail(t, "expected an element type at top level (§program-structure), found %s", t)
	}
	lower := strings.ToLower(t.Val)
	if right, ok := declWords[lower]; ok && right != t.Val {
		p.caseErr(t, right)
		p.toks[p.pos].Val = right
	}
	switch lower {
	case "package":
		return p.packageClause()
	case "import":
		if p.peekKind(1) == token.LPAREN {
			return p.importDecl()
		}
		p.fail(t, "expected '(' after import (§imports); Volt uses Go-style import blocks")
		return nil
	case "pipeline":
		return p.pipeline()
	case "group":
		return p.groupDecl()
	case "pred":
		return p.predDecl()
	case "select":
		return p.selectDecl()
	case "scope":
		return p.scope()
	case "get", "post", "put", "patch", "delete", "options", "head", "any", "resources", "dataset":
		p.fail(t, "routes, resources and datasets must appear inside a Scope (§scopes-and-routes)")
		return nil
	case "project":
		return p.project()
	case "table":
		return p.table()
	case "tablepartial":
		return p.tablePartial()
	case "type":
		return p.typeDecl()
	case "ref":
		return p.ref()
	case "records":
		return p.recordsDecl()
	case "migration":
		return p.migration()
	default:
		p.fail(t, "unknown element type %q (§program-structure)", t.Val)
		return nil
	}
}

func (p *parser) topLevelSync() {
	p.line = false
	for !p.at(token.EOF) {
		p.next()
	}
}

func (p *parser) lineSync() {
	p.line = false
	if !p.at(token.EOF) && !p.at(token.RBRACE) {
		p.next()
	}
	for !p.at(token.EOF) && !p.at(token.RBRACE) && !p.cur().NLBefore {
		p.next()
	}
}

func (p *parser) item(parse func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			p.lineSync()
		}
	}()
	parse()
}

func (p *parser) project() *ast.Project {
	d := &ast.Project{ProjectPos: p.next().Pos}
	if p.at(token.IDENT) {
		d.Name = p.ident("project name")
	}
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "Project (§project)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			p.lineBegin("project property (§project)")
			key := p.ident("project property (§project)")
			p.expect(token.COLON, "project property (§project)")
			val := p.expect(token.STRING, "project property value (§project)")
			d.Props = append(d.Props, &ast.ProjectProp{Key: key, Value: p.litOf(val)})
			p.endOfLine("project property (§project)")
		})
	}
	d.Rbrace = p.expect(token.RBRACE, "Project (§project)").Pos
	return d
}

func (p *parser) table() *ast.Table {
	d := &ast.Table{TablePos: p.next().Pos}
	d.Name = p.ident("table name (§table)")
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "Table (§table)")
	d.Body, d.Incomplete = p.tableBody("Table")
	d.Rbrace = p.expect(token.RBRACE, "Table (§table)").Pos
	return d
}

func (p *parser) tablePartial() *ast.TablePartial {
	d := &ast.TablePartial{PartialPos: p.next().Pos}
	d.Name = p.ident("TablePartial name (§tablepartial)")
	if p.at(token.LBRACKET) {
		p.fail(p.cur(), "a TablePartial takes no settings; its body holds columns, indexes and validators (§tablepartial)")
	}
	p.expect(token.LBRACE, "TablePartial (§tablepartial)")
	d.Body, d.Incomplete = p.tableBody("TablePartial")
	d.Rbrace = p.expect(token.RBRACE, "TablePartial (§tablepartial)").Pos
	return d
}

func (p *parser) tableBody(what string) (items []ast.TableItem, incomplete bool) {
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.tableItem(what); item != nil {
			items = append(items, item)
		} else {
			incomplete = true
		}
	}
	return items, incomplete
}

func (p *parser) tableItem(what string) (item ast.TableItem) {
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
	switch {
	case t.Kind == token.TILDE:
		p.lineBegin("partial injection (§tablepartial)")
		p.next()
		pr := &ast.PartialRef{TildePos: t.Pos, Name: p.ident("partial injection (§tablepartial)")}
		p.endOfLine("partial injection (§tablepartial)")
		return pr
	case p.atKw("Indexes") && p.peekKind(1) == token.LBRACE:
		return p.indexesBlock()
	case p.atKw("Validators") && p.peekKind(1) == token.LBRACE:
		return p.validatorsBlock()
	case p.atKw("Records") && (p.peekKind(1) == token.LPAREN || p.peekKind(1) == token.LBRACE):
		return p.records(nil)
	case t.Kind == token.IDENT:
		return p.column()
	default:
		p.fail(t, "expected a column, Indexes, Validators, Records or '~injection' in %s body, found %s", what, t)
		return nil
	}
}

func (p *parser) column() *ast.Column {
	c := p.columns.new()
	p.lineBegin("column definition (§columns)")
	c.Name = p.ident("column name (§columns)")
	c.Type = p.typeRef()
	if !p.cur().NLBefore && !p.at(token.LBRACKET) && !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.errorf(p.cur(), "expected a settings list or end of line after the column type (§columns), found %s", p.cur())
		for !p.at(token.EOF) && !p.at(token.RBRACE) && !p.cur().NLBefore {
			p.next()
		}
	}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		c.Settings = p.settingList()
	}
	p.endOfLine("column definition (§columns)")
	return c
}

func (p *parser) typeRef() *ast.TypeRef {
	tr := p.types.new()
	if p.at(token.LBRACKET) && p.peekKind(1) == token.RBRACKET {
		tr.Slice = true
		tr.Lbrack = p.next().Pos
		p.next()
	}
	tr.Name = p.qualName("column type (§columns)")
	return tr
}

func (p *parser) indexesBlock() *ast.IndexesBlock {
	b := &ast.IndexesBlock{IndexesPos: p.kwTake("Indexes").Pos}
	p.expect(token.LBRACE, "Indexes block (§indexes)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() { b.Indexes = append(b.Indexes, p.index()) })
	}
	b.Rbrace = p.expect(token.RBRACE, "Indexes block (§indexes)").Pos
	return b
}

func (p *parser) indexAtom() ast.Node {
	t := p.cur()
	switch t.Kind {
	case token.IDENT:
		return p.ident("index key (§indexes)")
	case token.FUNCEXPR:
		return &ast.FuncExpr{Tok: p.next()}
	default:
		p.fail(t, "index key must be a column name or backtick expression (§indexes), found %s", t)
		return nil
	}
}

func (p *parser) index() *ast.Index {
	p.lineBegin("index definition (§indexes)")
	ix := p.indexRest()
	p.endOfLine("index definition (§indexes)")
	return ix
}

func (p *parser) indexRest() *ast.Index {
	ix := &ast.Index{}
	if p.at(token.LPAREN) {
		ix.Composite = true
		p.next()
		for {
			ix.Key = append(ix.Key, p.indexAtom())
			if p.at(token.COMMA) {
				p.next()
				continue
			}
			break
		}
		p.expect(token.RPAREN, "composite index key (§indexes)")
	} else {
		ix.Key = append(ix.Key, p.indexAtom())
	}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		ix.Settings = p.settingList()
	}
	return ix
}

func (p *parser) validatorsBlock() *ast.ValidatorsBlock {
	b := &ast.ValidatorsBlock{ValidatorsPos: p.kwTake("Validators").Pos}
	p.expect(token.LBRACE, "Validators block (§validators)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			p.lineBegin("validator (§validators)")
			v := &ast.Validator{Ref: p.goRef("validator reference (§validators)")}
			p.expect(token.LPAREN, "validator arguments (§validators)")
			for {
				v.Args = append(v.Args, p.ident("validator argument (§validators)"))
				if !p.at(token.COMMA) {
					break
				}
				p.next()
			}
			v.EndPos = p.expect(token.RPAREN, "validator arguments (§validators)").End()
			p.endOfLine("validator (§validators)")
			b.Validators = append(b.Validators, v)
		})
	}
	b.Rbrace = p.expect(token.RBRACE, "Validators block (§validators)").Pos
	return b
}

func (p *parser) ref() *ast.Ref {
	d := &ast.Ref{RefPos: p.next().Pos}
	if !p.at(token.COLON) {
		p.fail(p.cur(), "expected ':' after Ref (§relationships-ref), found %s", p.cur())
	}
	p.next()
	d.Left = p.refEndpoint()
	if !p.at(token.GT) {
		p.fail(p.cur(), "expected '>' after the foreign key: a Ref is written foreign key > referenced columns (§relationships-ref), found %s", p.cur())
	}
	p.next()
	d.Right = p.refEndpoint()
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	d.SetEnd(p.toks[p.pos-1].End())
	return d
}

func (p *parser) refEndpoint() *ast.RefEndpoint {
	ep := &ast.RefEndpoint{Table: p.ident("relationship endpoint (§relationships-ref)")}
	if !p.at(token.DOT) {
		p.fail(p.cur(), "an endpoint is table.column or table.(columns) (§relationships-ref), found %s", p.cur())
	}
	p.next()
	if !p.at(token.LPAREN) {
		ep.Columns = []*ast.Ident{p.ident("endpoint column (§relationships-ref)")}
		ep.SetEnd(ep.Columns[0].End())
		return ep
	}
	p.next()
	for {
		ep.Columns = append(ep.Columns, p.ident("endpoint column (§relationships-ref)"))
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	ep.SetEnd(p.expect(token.RPAREN, "endpoint column group (§relationships-ref)").End())
	return ep
}

func (p *parser) typeDecl() *ast.TypeDecl {
	d := &ast.TypeDecl{TypePos: p.next().Pos}
	d.Name = p.qualName("declared type name (§types)")
	d.Storage = p.typeRef()
	d.EndPos = d.Storage.End()
	if p.atKw("from") && !p.cur().NLBefore {
		p.kwTake("from")
		d.From = &ast.BasicLit{Tok: p.expect(token.STRING, "import path after from (§types)")}
		d.EndPos = d.From.End()
	}
	p.endOfLine("Type declaration (§types)")
	return d
}

func (p *parser) recordsDecl() *ast.Records {
	pos := p.next().Pos
	table := p.ident("records target table (§records-sample-data)")
	return p.recordsRest(pos, table)
}

func (p *parser) records(_ *ast.Ident) *ast.Records {
	pos := p.kwTake("Records").Pos
	return p.recordsRest(pos, nil)
}

func (p *parser) recordsRest(pos token.Position, table *ast.Ident) *ast.Records {
	d := &ast.Records{RecordsPos: pos, Table: table}
	if p.at(token.LPAREN) {
		d.HasColumns = true
		p.next()
		for {
			d.Columns = append(d.Columns, p.ident("records column (§records-sample-data)"))
			if p.at(token.COMMA) {
				p.next()
				continue
			}
			break
		}
		p.expect(token.RPAREN, "records column list (§records-sample-data)")
	}
	p.expect(token.LBRACE, "records body (§records-sample-data)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			if row := p.recordRow(); row != nil {
				d.Rows = append(d.Rows, row)
			}
		})
	}
	d.Rbrace = p.expect(token.RBRACE, "records body (§records-sample-data)").Pos
	return d
}

func (p *parser) recordRow() *ast.RecordRow {
	row := &ast.RecordRow{}
	p.lineBegin("record row (§records-sample-data)")
	for {
		if len(row.Values) > 0 && (p.cur().NLBefore || p.at(token.RBRACE) || p.at(token.EOF)) {
			p.errorf(p.toks[p.pos-1], "empty record field after the last ',': every field holds a value, and NULL is written null (§records-sample-data)")
			p.lineClear()
			return nil
		}
		if p.at(token.COMMA) || p.at(token.RBRACE) || p.at(token.EOF) {
			p.fail(p.cur(), "empty record field: every field holds a value, and NULL is written null (§records-sample-data)")
		}
		row.Values = append(row.Values, p.recordValue())
		if p.at(token.COMMA) && !p.cur().NLBefore {
			p.next()
			continue
		}
		p.endOfLine("record row (§records-sample-data)")
		return row
	}
}

func (p *parser) recordValue() ast.Node {
	t := p.cur()
	switch t.Kind {
	case token.STRING, token.NUMBER:
		return p.litOf(p.next())
	case token.FUNCEXPR:
		return &ast.FuncExpr{Tok: p.next()}
	case token.MINUS:
		p.next()
		return &ast.NegNumber{MinusPos: t.Pos, Num: p.litOf(p.expect(token.NUMBER, "record value (§records-sample-data)"))}
	case token.IDENT:
		id := p.ident("record value (§records-sample-data)")
		p.wordFold(id, literalWords)
		if p.at(token.DOT) {
			p.fail(p.cur(), "a dotted name is not a record value: a record value is a literal, true, false or null, and a declared type takes a literal of its storage type (§records-sample-data)")
		}
		return id
	default:
		p.fail(t, "invalid record value (§records-sample-data): found %s", t)
		return nil
	}
}

func (p *parser) settingList() *ast.SettingList {
	sl := p.lists.new()
	sl.Lbrack = p.expect(token.LBRACKET, "settings list (§settings-lists)").Pos
	for {
		s := p.setting()
		sl.Settings = append(sl.Settings, s)
		if p.at(token.COMMA) {
			p.next()
			continue
		}
		break
	}
	if p.at(token.IDENT) {
		p.fail(p.cur(), "expected ',' or ']' in settings list, found %s: settings are separated by commas, and a setting name or value is one identifier (§settings-lists)", p.cur())
	}
	sl.Rbrack = p.expect(token.RBRACKET, "settings list (§settings-lists)").End()
	return sl
}

func (p *parser) setting() *ast.Setting {
	nameTok := p.expect(token.IDENT, "setting name (§settings-lists)")
	if right, ok := foldTo(nameTok.Val, SettingNames); ok && right != nameTok.Val {
		p.caseErr(nameTok, right)
		nameTok.Val = right
	}
	name := nameTok.Val
	s := p.settings.new()
	s.NameTok, s.Name = nameTok, name
	s.SetEnd(nameTok.End())
	if p.at(token.COLON) {
		p.next()
		if name == "ref" {
			s.Value = p.refEndpoint()
		} else {
			s.Value = p.settingValue()
		}
		if id, ok := s.Value.(*ast.Ident); ok {
			switch name {
			case "delete", "update":
				p.wordFold(id, RefActions)
			case "default":
				p.wordFold(id, literalWords)
			}
		}
		s.SetEnd(s.Value.End())
	}
	return s
}

func (p *parser) settingValue() ast.Node {
	t := p.cur()
	switch t.Kind {
	case token.STRING, token.NUMBER:
		return p.litOf(p.next())
	case token.FUNCEXPR:
		return &ast.FuncExpr{Tok: p.next()}
	case token.MINUS:
		p.next()
		return &ast.NegNumber{MinusPos: t.Pos, Num: p.litOf(p.expect(token.NUMBER, "setting value (§settings-lists)"))}
	case token.LPAREN:
		l := p.next()
		il := &ast.IdentList{Lparen: l.Pos}
		for {
			il.Names = append(il.Names, p.ident("identifier list (§settings-whitelists)"))
			var mod *ast.Ident
			if p.at(token.IDENT) && !p.cur().NLBefore {
				mod = p.ident("identifier list (§settings-whitelists)")
				p.wordFold(mod, orderWords)
			}
			il.Mods = append(il.Mods, mod)
			if p.at(token.COMMA) {
				p.next()
				continue
			}
			break
		}
		il.Rparen = p.expect(token.RPAREN, "identifier list (§settings-whitelists)").End()
		return il
	case token.IDENT:
		id := p.ident("setting value (§settings-lists)")
		if p.at(token.DOT) {
			p.next()
			return &ast.QualIdent{Qual: id, Name: p.ident("qualified setting value (§scopes)")}
		}
		return id
	default:
		p.fail(t, "invalid setting value (§settings-lists): found %s", t)
		return nil
	}
}
