// Migration productions (spec/migrations): the element, its body in
// either form, and the operations. The words of a migration body are
// keywords here and identifiers everywhere else (spec/keywords).
package parser

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (p *parser) migration() *ast.Migration {
	d := &ast.Migration{Doc: p.doc(), MigrationPos: p.next().Pos}
	d.Name = p.identOf(p.expect(token.IDENT, "migration name (spec/migrations)"))
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "migration (spec/migrations)")
	switch {
	case p.atKw("up"):
		d.UpDown = true
		p.kwTake("up")
		d.Up = p.operationsBlock("up block (spec/migrations)")
		if p.atKw("down") {
			d.DownPos = p.kwTake("down").Pos
			d.Down = p.operationsBlock("down block (spec/migrations)")
		}
	case p.atKw("down"):
		p.errorf(p.cur(),
			"a down block follows an up block; write up { } down { } (spec/migrations)")
		d.UpDown = true
		d.DownPos = p.kwTake("down").Pos
		d.Down = p.operationsBlock("down block (spec/migrations)")
	default:
		d.Up = p.operations("migration body (spec/migrations)")
	}
	d.Rbrace = p.expect(token.RBRACE, "migration (spec/migrations)").Pos
	return d
}

func (p *parser) operationsBlock(ctx string) []ast.Operation {
	p.expect(token.LBRACE, ctx)
	ops := p.operations(ctx)
	p.expect(token.RBRACE, ctx)
	return ops
}

func (p *parser) operations(ctx string) []ast.Operation {
	var ops []ast.Operation
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if op := p.operation(ctx); op != nil {
			ops = append(ops, op)
		}
	}
	return ops
}

var OperationWords = []string{
	"table_create", "table_drop", "table_rename", "table_alter", "table_rebuild",
	"index_create", "index_drop", "exec",
}

func (p *parser) operation(ctx string) (op ast.Operation) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			op = nil
			p.lineSync()
		}
	}()

	t := p.cur()
	doc := p.doc()
	switch p.operationWord() {
	case "table_create":
		return p.tableCreateOp(doc)
	case "index_create":
		return p.indexCreateOp(doc)
	case "table_drop":
		return p.tableDropOp(doc)
	case "index_drop":
		return p.indexDropOp(doc)
	case "table_rename":
		return p.tableRenameOp(doc)
	case "table_alter":
		return p.tableAlterOp(doc)
	case "table_rebuild":
		return p.tableRebuildOp(doc)
	case "exec":
		return p.execOp(doc)
	}
	switch {
	case p.atKw("down"):
		p.fail(t, "a down block follows an up block; write up { } down { } (spec/migrations)")
	case p.atKw("up"):
		p.fail(t, "an up block opens the migration body; nothing precedes it (spec/migrations)")
	default:
		p.fail(t, "expected an operation (%s) in %s, found %s",
			strings.Join(OperationWords, ", "), ctx, t)
	}
	return nil
}

func (p *parser) operationWord() string {
	t := p.cur()
	if t.Kind != token.IDENT {
		return ""
	}
	if word, ok := foldTo(t.Val, OperationWords); ok {
		return word
	}
	if p.pos+1 >= len(p.toks) || p.peekNL(1) || p.peekKind(1) != token.IDENT {
		return ""
	}
	second := p.toks[p.pos+1].Val
	if word, ok := foldTo(second+"_"+t.Val, OperationWords); ok {
		return word
	}
	if word, ok := foldTo(t.Val+"_"+second, OperationWords); ok {
		return word
	}
	return ""
}

func (p *parser) operationTake(word string) token.Token {
	if p.atKw(word) {
		return p.kwTake(word)
	}
	first := p.next()
	second := p.next()
	p.errorf(first, "write %s, not %s %s: a migration word is one identifier (spec/migrations)",
		word, first.Val, second.Val)
	return first

}

func (p *parser) tableCreateOp(doc *ast.CommentGroup) ast.Operation {
	d := &ast.TableCreate{Doc: doc, CreatePos: p.operationTake("table_create").Pos}
	d.Name = p.ident("table_create (spec/migrations)")
	d.Body, d.Incomplete, d.Rbrace = p.shapeBody("table_create (spec/migrations)")
	return d
}

func (p *parser) indexCreateOp(doc *ast.CommentGroup) ast.Operation {
	p.lineBegin("index_create (spec/migrations)")
	d := &ast.IndexCreate{Doc: doc, CreatePos: p.operationTake("index_create").Pos}
	d.Table = p.ident("index_create (spec/migrations)")
	d.Index = p.indexRest()
	p.endOfLine("index_create (spec/migrations)")
	return d
}

func (p *parser) tableDropOp(doc *ast.CommentGroup) ast.Operation {
	p.lineBegin("table_drop (spec/migrations)")
	d := &ast.TableDrop{Doc: doc, DropPos: p.operationTake("table_drop").Pos}
	d.Name = p.ident("table_drop (spec/migrations)")
	if p.at(token.LBRACE) && !p.cur().NLBefore {
		d.Lbrace = p.cur().Pos
		d.Body, d.Incomplete, d.Rbrace = p.shapeBody("table_drop (spec/migrations)")
	}
	p.endOfLine("table_drop (spec/migrations)")
	return d
}

func (p *parser) indexDropOp(doc *ast.CommentGroup) ast.Operation {
	p.lineBegin("index_drop (spec/migrations)")
	d := &ast.IndexDrop{Doc: doc, DropPos: p.operationTake("index_drop").Pos}
	d.Table = p.ident("index_drop (spec/migrations)")
	d.Index = p.indexRest()
	p.endOfLine("index_drop (spec/migrations)")
	return d
}

func (p *parser) tableRenameOp(doc *ast.CommentGroup) ast.Operation {
	p.lineBegin("table_rename (spec/migrations)")
	d := &ast.TableRename{Doc: doc, RenamePos: p.operationTake("table_rename").Pos}
	d.From = p.ident("table_rename (spec/migrations)")
	p.expectKw("to", "table_rename (spec/migrations)")
	d.To = p.ident("table_rename (spec/migrations)")
	p.endOfLine("table_rename (spec/migrations)")
	return d
}

func (p *parser) expectKw(kw, ctx string) {
	if !p.atKw(kw) {
		p.fail(p.cur(), "expected '%s' in %s, found %s", kw, ctx, p.cur())
	}
	p.kwTake(kw)
}

func (p *parser) tableAlterOp(doc *ast.CommentGroup) ast.Operation {
	d := &ast.TableAlter{Doc: doc, AlterPos: p.operationTake("table_alter").Pos}
	d.Name = p.ident("table_alter (spec/migrations)")
	p.expect(token.LBRACE, "table_alter (spec/migrations)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.alterItem(); item != nil {
			d.Items = append(d.Items, item)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "table_alter (spec/migrations)").Pos
	return d
}

func (p *parser) alterItem() (item ast.AlterItem) {
	doc := p.doc()
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
	case p.atKw("add"):
		pos := p.kwTake("add").Pos
		return &ast.AlterAdd{Doc: doc, AddPos: pos, Column: p.column()}
	case p.atKw("drop"):
		p.lineBegin("drop column (spec/migrations)")
		d := &ast.AlterDrop{Doc: doc, DropPos: p.kwTake("drop").Pos}
		d.Name = p.ident("drop column (spec/migrations)")
		if p.atTypeStart() {
			d.Type = p.typeRef()
			if p.at(token.LBRACKET) && !p.cur().NLBefore {
				d.Settings = p.settingList()
			}
		}
		p.endOfLine("drop column (spec/migrations)")
		return d
	case p.atKw("rename"):
		p.lineBegin("rename column (spec/migrations)")
		d := &ast.AlterRename{Doc: doc, RenamePos: p.kwTake("rename").Pos}
		d.From = p.ident("rename column (spec/migrations)")
		p.expectKw("to", "rename column (spec/migrations)")
		d.To = p.ident("rename column (spec/migrations)")
		p.endOfLine("rename column (spec/migrations)")
		return d
	default:
		p.fail(t, "expected add, drop or rename in table_alter body (spec/migrations), found %s", t)
		return nil
	}
}

func (p *parser) atTypeStart() bool {
	if p.cur().NLBefore {
		return false
	}
	return p.at(token.IDENT) || (p.at(token.LBRACKET) && p.peekKind(1) == token.RBRACKET)
}

func (p *parser) tableRebuildOp(doc *ast.CommentGroup) ast.Operation {
	d := &ast.TableRebuild{Doc: doc, RebuildPos: p.operationTake("table_rebuild").Pos}
	d.Name = p.ident("table_rebuild (spec/migrations)")
	d.Body, d.Incomplete, d.Rbrace = p.shapeBody("table_rebuild (spec/migrations)")
	return d
}

func (p *parser) execOp(doc *ast.CommentGroup) ast.Operation {
	p.lineBegin("exec (spec/migrations)")
	d := &ast.Exec{Doc: doc, ExecPos: p.kwTake("exec").Pos}

	if p.at(token.STRING) {
		p.fail(p.cur(), "exec takes its SQL in backticks, an expression literal, not a string (spec/migrations)")
	}
	d.SQL = &ast.FuncExpr{Tok: p.expect(token.FUNCEXPR, "exec (spec/migrations)")}
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.endOfLine("exec (spec/migrations)")
	return d
}

func (p *parser) shapeBody(ctx string) (items []ast.TableItem, incomplete bool, rbrace token.Position) {
	p.expect(token.LBRACE, ctx)
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.shapeItem(ctx); item != nil {
			items = append(items, item)
		} else {
			incomplete = true
		}
	}
	return items, incomplete, p.expect(token.RBRACE, ctx).Pos
}

func (p *parser) shapeItem(ctx string) (item ast.TableItem) {
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
	case p.atKw("indexes") && p.peekKind(1) == token.LBRACE:
		return p.indexesBlock()
	case t.Kind == token.TILDE,
		p.atKw("validators") && p.peekKind(1) == token.LBRACE,
		p.atKw("records") && (p.peekKind(1) == token.LPAREN || p.peekKind(1) == token.LBRACE):
		p.fail(t, "a migration's shape body holds columns and indexes only; a partial injection, a validators block and a records block are not part of it (spec/migrations)")
		return nil
	case t.Kind == token.IDENT:
		return p.column()
	default:
		p.fail(t, "expected a column or an indexes block in %s, found %s", ctx, t)
		return nil
	}
}
