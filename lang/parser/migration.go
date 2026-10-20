// Migration productions (spec §migrations): the element, its body in
// either form, and the operations. The words of a migration body are
// keywords here and identifiers everywhere else (§keywords).
package parser

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (p *parser) migration() *ast.Migration {
	d := &ast.Migration{MigrationPos: p.next().Pos}
	d.Name = p.ident("Migration name (§migrations)")
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.expect(token.LBRACE, "Migration (§migrations)")
	switch {
	case p.atKw("up"):
		d.UpDown = true
		p.next()
		d.Up = p.operationsBlock("up block (§migrations)")
		if p.atKw("down") {
			d.DownPos = p.next().Pos
			d.Down = p.operationsBlock("down block (§migrations)")
		}
	case p.atKw("down"):
		p.errorf(p.cur(), "a down block follows an up block; write up { } down { } (§migrations)")
		d.UpDown = true
		d.DownPos = p.next().Pos
		d.Down = p.operationsBlock("down block (§migrations)")
	default:
		d.Up = p.operations("Migration body (§migrations)")
	}
	d.Rbrace = p.expect(token.RBRACE, "Migration (§migrations)").Pos
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
	switch {
	case p.atKw("create"):
		return p.createOp()
	case p.atKw("drop"):
		return p.dropOp()
	case p.atKw("rename"):
		return p.renameTableOp()
	case p.atKw("alter"):
		return p.alterTableOp()
	case p.atKw("rebuild"):
		return p.rebuildTableOp()
	case p.atKw("exec"):
		return p.execOp()
	case p.atKw("down"):
		p.fail(t, "a down block follows an up block; write up { } down { } (§migrations)")
	case p.atKw("up"):
		p.fail(t, "an up block opens the Migration body; nothing precedes it (§migrations)")
	default:
		p.fail(t, "expected an operation (create, drop, rename, alter, rebuild, exec) in %s, found %s", ctx, t)
	}
	return nil
}

func (p *parser) createOp() ast.Operation {
	p.lineBegin("create (§migrations)")
	create := p.next()
	switch {
	case p.atKw("table"):
		p.lineClear()
		p.next()
		d := &ast.CreateTable{CreatePos: create.Pos}
		d.Name = p.ident("create table (§migrations)")
		d.Body, d.Rbrace = p.shapeBody("create table (§migrations)")
		return d
	case p.atKw("index"):
		p.next()
		d := &ast.CreateIndex{CreatePos: create.Pos}
		d.Table = p.ident("create index (§migrations)")
		d.Index = p.indexRest()
		p.endOfLine("create index (§migrations)")
		return d
	default:
		p.fail(p.cur(), "expected 'table' or 'index' after create (§migrations), found %s", p.cur())
		return nil
	}
}

func (p *parser) dropOp() ast.Operation {
	p.lineBegin("drop (§migrations)")
	drop := p.next()
	switch {
	case p.atKw("table"):
		p.next()
		d := &ast.DropTable{DropPos: drop.Pos}
		d.Name = p.ident("drop table (§migrations)")
		if p.at(token.LBRACE) && !p.cur().NLBefore {
			d.Lbrace = p.cur().Pos
			d.Body, d.Rbrace = p.shapeBody("drop table (§migrations)")
		}
		p.endOfLine("drop table (§migrations)")
		return d
	case p.atKw("index"):
		p.next()
		d := &ast.DropIndex{DropPos: drop.Pos}
		d.Table = p.ident("drop index (§migrations)")
		d.Index = p.indexRest()
		p.endOfLine("drop index (§migrations)")
		return d
	default:
		p.fail(p.cur(), "expected 'table' or 'index' after drop (§migrations), found %s", p.cur())
		return nil
	}
}

func (p *parser) renameTableOp() ast.Operation {
	p.lineBegin("rename table (§migrations)")
	d := &ast.RenameTable{RenamePos: p.next().Pos}
	if !p.atKw("table") {
		p.fail(p.cur(), "expected 'table' after rename (§migrations), found %s", p.cur())
	}
	p.next()
	d.From = p.ident("rename table (§migrations)")
	p.expectKw("to", "rename table (§migrations)")
	d.To = p.ident("rename table (§migrations)")
	p.endOfLine("rename table (§migrations)")
	return d
}

func (p *parser) expectKw(kw, ctx string) {
	if !p.atKw(kw) {
		p.fail(p.cur(), "expected '%s' in %s, found %s", kw, ctx, p.cur())
	}
	p.next()
}

func (p *parser) alterTableOp() ast.Operation {
	d := &ast.AlterTable{AlterPos: p.next().Pos}
	p.expectKw("table", "alter table (§migrations)")
	d.Name = p.ident("alter table (§migrations)")
	p.expect(token.LBRACE, "alter table (§migrations)")
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.alterItem(); item != nil {
			d.Items = append(d.Items, item)
		}
	}
	d.Rbrace = p.expect(token.RBRACE, "alter table (§migrations)").Pos
	return d
}

func (p *parser) alterItem() (item ast.AlterItem) {
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
		pos := p.next().Pos
		return &ast.AlterAdd{AddPos: pos, Column: p.column()}
	case p.atKw("drop"):
		p.lineBegin("drop column (§migrations)")
		d := &ast.AlterDrop{DropPos: p.next().Pos}
		d.Name = p.ident("drop column (§migrations)")
		if p.atTypeStart() {
			d.Type = p.typeRef()
			if p.at(token.LBRACKET) && !p.cur().NLBefore {
				d.Settings = p.settingList()
			}
		}
		p.endOfLine("drop column (§migrations)")
		return d
	case p.atKw("rename"):
		p.lineBegin("rename column (§migrations)")
		d := &ast.AlterRename{RenamePos: p.next().Pos}
		d.From = p.ident("rename column (§migrations)")
		p.expectKw("to", "rename column (§migrations)")
		d.To = p.ident("rename column (§migrations)")
		p.endOfLine("rename column (§migrations)")
		return d
	default:
		p.fail(t, "expected add, drop or rename in alter table body (§migrations), found %s", t)
		return nil
	}
}

func (p *parser) atTypeStart() bool {
	if p.cur().NLBefore {
		return false
	}
	return p.at(token.IDENT) || (p.at(token.LBRACKET) && p.peekKind(1) == token.RBRACKET)
}

func (p *parser) rebuildTableOp() ast.Operation {
	d := &ast.RebuildTable{RebuildPos: p.next().Pos}
	p.expectKw("table", "rebuild table (§migrations)")
	d.Name = p.ident("rebuild table (§migrations)")
	d.Body, d.Rbrace = p.shapeBody("rebuild table (§migrations)")
	return d
}

func (p *parser) execOp() ast.Operation {
	p.lineBegin("exec (§migrations)")
	d := &ast.Exec{ExecPos: p.next().Pos}
	d.SQL = p.litOf(p.expect(token.STRING, "exec (§migrations)"))
	if p.at(token.LBRACKET) && !p.cur().NLBefore {
		d.Settings = p.settingList()
	}
	p.endOfLine("exec (§migrations)")
	return d
}

func (p *parser) shapeBody(ctx string) ([]ast.TableItem, token.Position) {
	p.expect(token.LBRACE, ctx)
	var items []ast.TableItem
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		if item := p.shapeItem(ctx); item != nil {
			items = append(items, item)
		}
	}
	return items, p.expect(token.RBRACE, ctx).Pos
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
	case t.Kind == token.TILDE, p.atNoteDef(),
		p.atKw("validators") && p.peekKind(1) == token.LBRACE,
		p.atKw("records") && (p.peekKind(1) == token.LPAREN || p.peekKind(1) == token.LBRACE):
		p.fail(t, "a migration's shape body holds columns and indexes only; a partial injection, a validators block, a records block and a Note are not part of it (§migrations)")
		return nil
	case t.Kind == token.IDENT:
		return p.column()
	default:
		p.fail(t, "expected a column or an indexes block in %s, found %s", ctx, t)
		return nil
	}
}
