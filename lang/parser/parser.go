// Package parser turns DBML source into an ast.File, mirroring go/parser.
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

// ParseFile parses one DBML source file. It always returns a (possibly
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
	return t.Kind == token.IDENT && !t.Quoted && strings.EqualFold(t.Val, kw)
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
	return p.identOf(p.expect(token.IDENT, ctx))
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
		p.fail(t, "expected an element type or use/reuse at top level (§program-structure), found %s", t)
	}
	switch strings.ToLower(t.Val) {
	case "use", "reuse":
		return p.useDecl()
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
	case "enum":
		return p.enum()
	case "type":
		return p.typeDecl()
	case "ref":
		return p.ref()
	case "note":
		return p.stickyNote()
	case "tablegroup":
		return p.tableGroup()
	case "records":
		return p.recordsDecl()
	case "diagramview":
		return p.diagramView()
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

func (p *parser) useDecl() *ast.Use {
	kw := p.next()
	d := &ast.Use{UsePos: kw.Pos, Reuse: strings.EqualFold(kw.Val, "reuse")}
	switch {
	case p.at(token.STAR):
		p.next()
		d.Wildcard = true
	case p.at(token.LBRACE):
		p.next()
		for !p.at(token.RBRACE) && !p.at(token.EOF) && !p.atKw("from") {
			d.Items = append(d.Items, p.useItem())
		}
		p.expect(token.RBRACE, "import specifier list (§file-imports-removed)")
		if len(d.Items) == 0 {
			p.fail(p.cur(), "selective import requires at least one item (§file-imports-removed)")
		}
	default:
		p.fail(p.cur(), "expected '*' or '{' after use/reuse (§file-imports-removed)")
	}
	if !p.atKw("from") {
		p.fail(p.cur(), "expected 'from' in import statement (§file-imports-removed)")
	}
	p.next()
	d.Path = p.litOf(p.expect(token.STRING, "import path (§file-imports-removed)"))
	return d
}

func (p *parser) useItem() *ast.UseItem {
	it := &ast.UseItem{Kind: p.ident("import item kind (§file-imports-removed)")}
	it.Name = p.qualName("import item name (§file-imports-removed)")
	if p.atKw("as") {
		p.next()
		it.Alias = p.ident("import alias (§file-imports-removed)")
	}
	return it
}

func (p *parser) project() *ast.Project {
	d := &ast.Project{ProjectPos: p.next().Pos}
	if p.at(token.IDENT) {
		d.Name = p.ident("project name")
	}
	p.expect(token.LBRACE, "Project (§project)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			if p.atNoteDef() {
				d.Notes = append(d.Notes, p.noteDef())
				return
			}
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
	d.Name = p.qualName("table name (§table)")
	if p.atKw("as") {
		p.next()
		d.Alias = p.ident("table alias (§table)")
	}
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "Table (§table)")
	d.Body = p.tableBody("Table")
	d.Rbrace = p.expect(token.RBRACE, "Table (§table)").Pos
	return d
}

func (p *parser) tablePartial() *ast.TablePartial {
	d := &ast.TablePartial{PartialPos: p.next().Pos}
	d.Name = p.ident("TablePartial name (§tablepartial)")
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "TablePartial (§tablepartial)")
	d.Body = p.tableBody("TablePartial")
	d.Rbrace = p.expect(token.RBRACE, "TablePartial (§tablepartial)").Pos
	return d
}

func (p *parser) atNoteDef() bool {
	return p.atKw("note") && (p.peekKind(1) == token.COLON || p.peekKind(1) == token.LBRACE)
}

func (p *parser) tableBody(what string) []ast.TableItem {
	var items []ast.TableItem
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		item := p.tableItem(what)
		if item != nil {
			items = append(items, item)
		}
	}
	return items
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
	case p.atKw("indexes") && p.peekKind(1) == token.LBRACE:
		return p.indexesBlock()
	case p.atKw("validators") && p.peekKind(1) == token.LBRACE:
		return p.validatorsBlock()
	case p.atNoteDef():
		return p.noteDef()
	case p.atKw("records") && (p.peekKind(1) == token.LPAREN || p.peekKind(1) == token.LBRACE):
		return p.records(nil)
	case t.Kind == token.IDENT:
		return p.column()
	default:
		p.fail(t, "expected a column, indexes, validators, Note, records or '~injection' in %s body, found %s", what, t)
		return nil
	}
}

func (p *parser) column() *ast.Column {
	c := p.columns.new()
	p.lineBegin("column definition (§columns)")
	c.Name = p.ident("column name (§columns)")
	c.Type = p.typeRef()
	for p.at(token.IDENT) && !p.cur().NLBefore {
		c.LegacyFlags = append(c.LegacyFlags, p.ident("legacy flag (§columns)"))
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
	b := &ast.IndexesBlock{IndexesPos: p.next().Pos}
	p.expect(token.LBRACE, "indexes block (§indexes)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() { b.Indexes = append(b.Indexes, p.index()) })
	}
	b.Rbrace = p.expect(token.RBRACE, "indexes block (§indexes)").Pos
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
	ix := &ast.Index{}
	p.lineBegin("index definition (§indexes)")
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
	p.endOfLine("index definition (§indexes)")
	return ix
}

func (p *parser) validatorsBlock() *ast.ValidatorsBlock {
	b := &ast.ValidatorsBlock{ValidatorsPos: p.next().Pos}
	p.expect(token.LBRACE, "validators block (§validators)")
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
	b.Rbrace = p.expect(token.RBRACE, "validators block (§validators)").Pos
	return b
}

func (p *parser) ref() *ast.Ref {
	d := &ast.Ref{RefPos: p.next().Pos}
	if p.at(token.IDENT) {
		d.Name = p.ident("relationship name")
	}
	if p.at(token.COLON) {
		p.next()
		p.refBody(d)
		d.SetEnd(p.toks[p.pos-1].End())
		return d
	}
	d.Long = true
	p.expect(token.LBRACE, "Ref (§relationships-ref)")
	if p.at(token.RBRACE) {
		p.fail(p.cur(), "a Ref declares exactly one relationship (§relationships-ref)")
	}
	p.refBody(d)
	if !p.at(token.RBRACE) {
		p.fail(p.cur(), "a Ref declares exactly one relationship (§relationships-ref); found %s after the first", p.cur())
	}
	d.SetEnd(p.expect(token.RBRACE, "Ref (§relationships-ref)").End())
	return d
}

func (p *parser) refBody(d *ast.Ref) {
	d.Left = p.refEndpoint()
	d.OpTok = p.relOp()
	d.Right = p.refEndpoint()
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
}

func (p *parser) relOp() token.Token {
	t := p.cur()
	switch t.Kind {
	case token.LT, token.GT, token.MINUS, token.LTGT:
		return p.next()
	}
	p.fail(t, "expected relationship operator <, >, - or <> (§relationships-ref), found %s", t)
	return token.Token{}
}

func (p *parser) refEndpoint() *ast.RefEndpoint {
	first := p.ident("relationship endpoint (§relationships-ref)")
	parts := []*ast.Ident{first}
	ep := &ast.RefEndpoint{}
	for {
		if !p.at(token.DOT) {
			p.fail(p.cur(), "endpoint must reference a column as [schema.]table.column (§relationships-ref)")
		}
		p.next()
		if p.at(token.LPAREN) {
			p.next()
			for {
				ep.Columns = append(ep.Columns, p.ident("endpoint column (§relationships-ref)"))
				if p.at(token.COMMA) {
					p.next()
					continue
				}
				break
			}
			ep.SetEnd(p.expect(token.RPAREN, "endpoint column group (§relationships-ref)").End())
			break
		}
		parts = append(parts, p.ident("relationship endpoint (§relationships-ref)"))
		if !p.at(token.DOT) {
			ep.Columns = []*ast.Ident{parts[len(parts)-1]}
			parts = parts[:len(parts)-1]
			ep.SetEnd(ep.Columns[0].End())
			break
		}
		if len(parts) == 3 {
			p.fail(p.cur(), "endpoint has too many name parts; expected [schema.]table.column (§relationships-ref)")
		}
	}
	if len(parts) == 0 || len(parts) > 2 {
		p.fail(p.cur(), "endpoint must be [schema.]table.column (§relationships-ref)")
	}
	ep.Table = p.qualOf(parts...)
	return ep
}

func (p *parser) enum() *ast.Enum {
	d := &ast.Enum{EnumPos: p.next().Pos}
	d.Name = p.qualName("enum name (§enum)")
	p.expect(token.LBRACE, "Enum (§enum)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			p.lineBegin("enum value (§enum)")
			v := &ast.EnumValue{Name: p.ident("enum value (§enum)")}
			if p.at(token.LBRACKET) && !p.cur().NLBefore {
				v.Settings = p.settingList()
			}
			p.endOfLine("enum value (§enum)")
			d.Values = append(d.Values, v)
		})
	}
	d.Rbrace = p.expect(token.RBRACE, "Enum (§enum)").Pos
	return d
}

func (p *parser) typeDecl() *ast.TypeDecl {
	d := &ast.TypeDecl{TypePos: p.next().Pos}
	d.Name = p.qualName("declared type name (§types)")
	d.Storage = p.typeRef()
	d.EndPos = d.Storage.End()
	if p.at(token.IDENT) && !p.cur().NLBefore && strings.EqualFold(p.cur().Val, "from") {
		p.next()
		d.From = &ast.BasicLit{Tok: p.expect(token.STRING, "import path after from (§types)")}
		d.EndPos = d.From.End()
	}
	p.endOfLine("Type declaration (§types)")
	return d
}

func (p *parser) recordsDecl() *ast.Records {
	pos := p.next().Pos
	table := p.qualName("records target table (§records-sample-data)")
	return p.recordsRest(pos, table)
}

func (p *parser) records(_ *ast.QualName) *ast.Records {
	pos := p.next().Pos
	return p.recordsRest(pos, nil)
}

func (p *parser) recordsRest(pos token.Position, table *ast.QualName) *ast.Records {
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
		p.item(func() { d.Rows = append(d.Rows, p.recordRow()) })
	}
	d.Rbrace = p.expect(token.RBRACE, "records body (§records-sample-data)").Pos
	return d
}

func (p *parser) recordRow() *ast.RecordRow {
	row := &ast.RecordRow{}
	p.lineBegin("record row (§records-sample-data)")
	rowStart := true
	for {
		switch {
		case p.at(token.COMMA) || p.at(token.RBRACE) || p.at(token.EOF),
			!rowStart && p.cur().NLBefore:
			at := p.cur().Pos
			if !rowStart {
				at = p.toks[p.pos-1].End()
			}
			row.Values = append(row.Values, &ast.Empty{At: at})
		default:
			row.Values = append(row.Values, p.recordValue())
			rowStart = false
		}
		if p.at(token.COMMA) && (!p.cur().NLBefore || rowStart) {
			p.next()
			rowStart = false
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
		if p.at(token.DOT) {
			p.next()
			return &ast.EnumConst{Enum: id, Value: p.ident("enum constant (§records-sample-data)")}
		}
		return id
	default:
		p.fail(t, "invalid record value (§records-sample-data): found %s", t)
		return nil
	}
}

func (p *parser) noteDef() *ast.Note {
	p.lineBegin("note definition (§notes)")
	n := &ast.Note{NotePos: p.next().Pos}
	if p.at(token.COLON) {
		p.next()
		n.Text = p.litOf(p.expect(token.STRING, "note value (§notes)"))
		n.SetEnd(n.Text.End())
		p.endOfLine("note definition (§notes)")
		return n
	}
	p.lineClear()
	p.expect(token.LBRACE, "note definition (§notes)")
	n.Text = p.litOf(p.expect(token.STRING, "note value (§notes)"))
	n.SetEnd(p.expect(token.RBRACE, "note definition (§notes)").End())
	return n
}

func (p *parser) stickyNote() *ast.StickyNote {
	d := &ast.StickyNote{NotePos: p.next().Pos}
	d.Name = p.ident("sticky note name (§notes)")
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "sticky note (§notes)")
	d.Text = p.litOf(p.expect(token.STRING, "sticky note value (§notes)"))
	d.Rbrace = p.expect(token.RBRACE, "sticky note (§notes)").Pos
	return d
}

func (p *parser) tableGroup() *ast.TableGroup {
	d := &ast.TableGroup{GroupPos: p.next().Pos}
	d.Name = p.ident("TableGroup name (§tablegroup)")
	if p.at(token.LBRACKET) {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "TableGroup (§tablegroup)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.item(func() {
			if p.atNoteDef() {
				d.Notes = append(d.Notes, p.noteDef())
				return
			}
			p.lineBegin("TableGroup member (§tablegroup)")
			d.Members = append(d.Members, p.qualName("TableGroup member (§tablegroup)"))
			p.endOfLine("TableGroup member (§tablegroup)")
		})
	}
	d.Rbrace = p.expect(token.RBRACE, "TableGroup (§tablegroup)").Pos
	return d
}

func (p *parser) diagramView() *ast.DiagramView {
	d := &ast.DiagramView{ViewPos: p.next().Pos}
	d.Name = p.ident("DiagramView name (§diagramview)")
	p.expect(token.LBRACE, "DiagramView (§diagramview)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		c := &ast.ViewCategory{Kind: p.ident("view category (§diagramview)")}
		p.expect(token.LBRACE, "view category body (§diagramview)")
		if p.at(token.STAR) {
			p.next()
			c.Wildcard = true
		} else {
			for !p.at(token.RBRACE) && !p.at(token.EOF) {
				p.item(func() {
					p.lineBegin("view category member (§diagramview)")
					c.Names = append(c.Names, p.qualName("view category member (§diagramview)"))
					p.endOfLine("view category member (§diagramview)")
				})
			}
		}
		c.Rbrace = p.expect(token.RBRACE, "view category body (§diagramview)").Pos
		d.Categories = append(d.Categories, c)
	}
	d.Rbrace = p.expect(token.RBRACE, "DiagramView (§diagramview)").Pos
	return d
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
	sl.Rbrack = p.expect(token.RBRACKET, "settings list (§settings-lists)").End()
	return sl
}

func (p *parser) setting() *ast.Setting {
	nameTok := p.expect(token.IDENT, "setting name (§settings-lists)")
	name := strings.ToLower(nameTok.Val)
	end := nameTok.End()
	for p.at(token.IDENT) && !p.cur().NLBefore {
		w := p.next()
		name += " " + strings.ToLower(w.Val)
		end = w.End()
	}
	s := p.settings.new()
	s.NameTok, s.Name = nameTok, name
	s.SetEnd(end)
	if p.at(token.COLON) {
		p.next()
		s.Value = p.settingValue()
		s.SetEnd(s.Value.End())
	}
	return s
}

func (p *parser) settingValue() ast.Node {
	t := p.cur()
	switch t.Kind {
	case token.STRING, token.NUMBER, token.COLOR:
		return p.litOf(p.next())
	case token.FUNCEXPR:
		return &ast.FuncExpr{Tok: p.next()}
	case token.MINUS:
		if p.peekKind(1) == token.IDENT {
			op := p.next()
			return &ast.RefValue{OpTok: op, Endpoint: p.refEndpoint()}
		}
		p.next()
		return &ast.NegNumber{MinusPos: t.Pos, Num: p.litOf(p.expect(token.NUMBER, "setting value (§settings-lists)"))}
	case token.LT, token.GT, token.LTGT:
		op := p.next()
		return &ast.RefValue{OpTok: op, Endpoint: p.refEndpoint()}
	case token.LPAREN:
		l := p.next()
		il := &ast.IdentList{Lparen: l.Pos}
		for {
			il.Names = append(il.Names, p.ident("identifier list (§settings-whitelists)"))
			var mod *ast.Ident
			if p.at(token.IDENT) && !p.cur().NLBefore {
				mod = p.ident("identifier list (§settings-whitelists)")
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
			return &ast.EnumConst{Enum: id, Value: p.ident("enum constant (§names-and-qualified-names)")}
		}
		words := []*ast.Ident{id}
		for p.at(token.IDENT) && !p.cur().NLBefore {
			words = append(words, p.ident("setting value (§settings-lists)"))
		}
		if len(words) == 1 {
			return id
		}
		return &multiWordValue{words: words}
	default:
		p.fail(t, "invalid setting value (§settings-lists): found %s", t)
		return nil
	}
}

type multiWordValue struct{ words []*ast.Ident }

func (m *multiWordValue) Pos() token.Position { return m.words[0].Pos() }
func (m *multiWordValue) End() token.Position { return m.words[len(m.words)-1].End() }

// Words exposes the phrase.
func (m *multiWordValue) Words() []string {
	out := make([]string, len(m.words))
	for i, w := range m.words {
		out[i] = strings.ToLower(w.Name())
	}
	return out
}

// MultiWord is implemented by setting values consisting of several
// space-separated identifiers.
type MultiWord interface{ Words() []string }
