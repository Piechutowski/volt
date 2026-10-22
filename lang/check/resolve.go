// Pass 3: cross-element resolution (§reference-resolution) of relationship endpoints,
// index columns and records columns, and duplicate-relationship
// detection (§relationships-ref).
package check

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

func (c *checker) resolve(f *ast.File) {
	for _, d := range f.Decls {
		if r, ok := d.(*ast.Ref); ok {
			ri := &RefInfo{Node: r, FK: c.endpointResolve(r.Left), Target: c.endpointResolve(r.Right)}
			c.refActionsNullCheck(r, ri)
			c.info.Refs = append(c.info.Refs, ri)
		}
	}
	for _, ti := range c.info.Tables {
		for _, cd := range ti.Columns {
			if cd.Col.Settings == nil {
				continue
			}
			for _, s := range cd.Col.Settings.Settings {
				ep, ok := s.Value.(*ast.RefEndpoint)
				if s.Name != "ref" || !ok {
					continue
				}
				ri := &RefInfo{Node: s, Inline: true}
				ri.FK = EndpointInfo{
					Table:   ti,
					Columns: []string{cd.Col.Name.Name()},
					Pos:     s.Pos(),
				}
				ri.Target = c.endpointResolve(ep)
				if len(ri.Target.Columns) != 1 {
					c.errorf(ep.Pos(), "relationships-ref", "an inline ref endpoint has exactly one column")
				}
				c.info.Refs = append(c.info.Refs, ri)
			}
		}
	}
	c.duplicateRefsCheck()

	for _, ti := range c.info.Tables {
		for _, ix := range ti.Indexes {
			for _, atom := range ix.Key {
				id, ok := atom.(*ast.Ident)
				if !ok {
					continue
				}
				if ti.Column(id.Name()) == nil {
					c.errorf(id.Pos(), "indexes", "index references unknown column %q in table %q", id.Name(), ti.Key)
				}
			}
		}
	}

	for _, d := range f.Decls {
		if r, ok := d.(*ast.Records); ok {
			c.recordsResolve(r, nil)
		}
	}
	recordsSeen := map[string]token.Position{}
	for _, ti := range c.info.Tables {
		for _, item := range ti.Decl.Body {
			if r, ok := item.(*ast.Records); ok {
				c.recordsResolve(r, ti)
			}
		}
	}
	for _, d := range f.Decls {
		if r, ok := d.(*ast.Records); ok && r.Table != nil {
			key := r.Table.Name()
			if _, dup := recordsSeen[key]; dup {
				c.errorf(r.Pos(), "records-sample-data", "at most one Records block per table, and %q has another (§records-sample-data)", key)
			}
			recordsSeen[key] = r.Pos()
			if ti := c.info.byName[key]; ti != nil {
				for _, item := range ti.Decl.Body {
					if _, ok := item.(*ast.Records); ok {
						c.errorf(r.Pos(), "records-sample-data", "table %q already has an in-table Records block (§records-sample-data)", key)
						break
					}
				}
			}
		}
	}
}

func (c *checker) refActionsNullCheck(r *ast.Ref, ri *RefInfo) {
	fk := ri.FK
	if fk.Table == nil || r.Settings == nil {
		return
	}
	for _, s := range r.Settings.Settings {
		if s.Name != "delete" && s.Name != "update" {
			continue
		}
		for _, name := range fk.Columns {
			if cd := fk.Table.Column(name); cd != nil {
				c.refActionNullCheck(s, cd.Col, fk.Table.Key+"."+name, "relationships-ref")
			}
		}
	}
}

func (c *checker) endpointResolve(ep *ast.RefEndpoint) EndpointInfo {
	out := EndpointInfo{Pos: ep.Pos()}
	for _, col := range ep.Columns {
		out.Columns = append(out.Columns, col.Name())
	}
	ti := c.info.byName[ep.Table.Name()]
	if ti == nil {
		c.errorf(ep.Table.Pos(), "reference-resolution", "unknown table %q in relationship endpoint", ep.Table.Name())
		return out
	}
	out.Table = ti
	for _, col := range ep.Columns {
		if ti.Column(col.Name()) == nil {
			c.errorf(col.Pos(), "reference-resolution", "unknown column %q in table %q", col.Name(), ti.Key)
		}
	}
	return out
}

func refKey(r *RefInfo) (string, bool) {
	side := func(e EndpointInfo) (string, bool) {
		if e.Table == nil {
			return "", false
		}
		return e.Table.Key + "(" + strings.Join(e.Columns, ",") + ")", true
	}
	l, okL := side(r.FK)
	rr, okR := side(r.Target)
	if !okL || !okR {
		return "", false
	}
	pair := []string{l, rr}
	sort.Strings(pair)
	return pair[0] + "|" + pair[1], true
}

func (c *checker) duplicateRefsCheck() {
	seen := map[string]bool{}
	for _, r := range c.info.Refs {
		key, ok := refKey(r)
		if !ok {
			continue
		}
		if seen[key] {
			c.errorf(r.Node.Pos(), "relationships-ref", "duplicate relationship: the same endpoints are already related")
			continue
		}
		seen[key] = true
	}
}

func (c *checker) recordsResolve(r *ast.Records, ti *TableInfo) {
	if r.Table != nil {
		ti = c.info.byName[r.Table.Name()]
		if ti == nil {
			c.errorf(r.Table.Pos(), "records-sample-data", "unknown table %q in records", r.Table.Name())
			return
		}
	}
	if ti == nil {
		return
	}
	for _, col := range r.Columns {
		if ti.Column(col.Name()) == nil {
			c.errorf(col.Pos(), "records-sample-data", "column %q does not exist in table %q", col.Name(), ti.Key)
		}
	}
	if !r.HasColumns {
		for _, row := range r.Rows {
			if len(row.Values) != len(ti.Columns) {
				c.errorf(row.Pos(), "records-sample-data", "record row has %d values but table %q has %d columns", len(row.Values), ti.Key, len(ti.Columns))
			}
		}
	}
}
