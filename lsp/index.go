package lsp

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"strings"
)

// SymKind classifies the symbols the server can navigate.
type SymKind int

const (
	SymTable SymKind = iota
	SymEnum
	SymPartial
	SymColumn
	SymEnumValue
	SymType
)

// SymbolID names one symbol. Tables and enums use their name; columns
// live in a container, "table:<name>" for direct definitions or
// "partial:<name>" for columns defined in a TablePartial, so every table
// sharing the partial shares the symbol. Enum values live in
// "enum:<name>".
type SymbolID struct {
	Kind      SymKind
	Container string
	Name      string
}

// Occurrence is one identifier that denotes a symbol.
type Occurrence struct {
	ID     SymbolID
	Ident  *ast.Ident
	IsDecl bool
}

// Index is every symbol occurrence in a document, plus resolution maps.
type Index struct {
	Occs []Occurrence

	Tables   map[string]*check.TableInfo // name -> table
	Enums    map[string]*check.EnumInfo  // name -> enum
	Types    map[string]*check.TypeInfo  // declared name -> Type
	Partials map[string]*check.PartialInfo

	decls map[SymbolID]*ast.Ident

	declTables map[string]*tableOccs
	recording  *[]declEntry
}

// IndexMemo remembers a package's per-table occurrences across builds
// of its index: a table whose declaration node and checked table
// are the objects they were contributes the occurrences it did. The
// zero value is ready; a memo belongs to one package and one goroutine
// at a time, and drops what a build did not use.
type IndexMemo struct {
	prev, next   map[*ast.Table]*tableOccs
	lastLen      int
	Hits, Misses int
}

type tableOccs struct {
	ti, resolved    *check.TableInfo
	tables          map[string]*check.TableInfo
	enums           map[string]*check.EnumInfo
	types           map[string]*check.TypeInfo
	partials        map[string]bool
	sym, cols, body []Occurrence
	decls           []declEntry
	declMap         map[SymbolID]*ast.Ident
}

func (e *tableOccs) holds(ti, resolved *check.TableInfo, tables map[string]*check.TableInfo, enums map[string]*check.EnumInfo, types map[string]*check.TypeInfo, partials map[string]bool) bool {
	if e.ti != ti || e.resolved != resolved || len(e.tables) != len(tables) || len(e.enums) != len(enums) || len(e.types) != len(types) || len(e.partials) != len(partials) {
		return false
	}
	for key, t := range types {
		old, ok := e.types[key]
		if !ok || (old == nil) != (t == nil) || (t != nil && old.Decl != t.Decl) {
			return false
		}
	}
	for key, t := range tables {
		if old, ok := e.tables[key]; !ok || old != t {
			return false
		}
	}
	for key, ei := range enums {
		old, ok := e.enums[key]
		if !ok || (old == nil) != (ei == nil) || (ei != nil && old.Decl != ei.Decl) {
			return false
		}
	}
	for name, present := range partials {
		if old, ok := e.partials[name]; !ok || old != present {
			return false
		}
	}
	return true
}

const tableContainer = "table:"

func declMapOf(decls []declEntry) map[SymbolID]*ast.Ident {
	m := make(map[SymbolID]*ast.Ident, len(decls))
	for _, d := range decls {
		if strings.HasPrefix(d.id.Container, tableContainer) {
			if _, exists := m[d.id]; !exists {
				m[d.id] = d.ident
			}
		}
	}
	return m
}

type declEntry struct {
	id    SymbolID
	ident *ast.Ident
}

// NewIndex resolves every name in the file against the semantic model.
func NewIndex(f *ast.File, info *check.Info) *Index {
	return NewIndexMemo(f, info, nil)
}

// NewIndexMemo is NewIndex with a memo of the package's earlier
// indexes; a nil memo builds everything.
func NewIndexMemo(f *ast.File, info *check.Info, memo *IndexMemo) *Index {
	ix := &Index{
		Tables:   map[string]*check.TableInfo{},
		Enums:    map[string]*check.EnumInfo{},
		Types:    map[string]*check.TypeInfo{},
		Partials: map[string]*check.PartialInfo{},
		decls:    map[SymbolID]*ast.Ident{},
	}
	for _, ti := range info.Tables {
		ix.Tables[ti.Key] = ti
	}
	for _, ei := range info.Enums {
		ix.Enums[ei.Key] = ei
	}
	for _, td := range info.Types {
		ix.Types[td.Key] = td
	}
	for _, pi := range info.Partials {
		ix.Partials[pi.Decl.Name.Name()] = pi
	}

	entries := make(map[*ast.Table]*tableOccs, len(info.Tables))
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
		ix.Occs = make([]Occurrence, 0, memo.lastLen)
		defer func() { memo.lastLen = len(ix.Occs) }()
	}
	for _, ti := range info.Tables {
		resolved := ix.resolveTable(ti.Decl.Name)
		tables := ix.tablesNamed(ti.Decl)
		enums := ix.enumsNamed(ti.Decl)
		types := ix.typesNamed(ti.Decl)
		partials := ix.partialsNamed(ti.Decl)
		var e *tableOccs
		if memo != nil {
			if prev := memo.prev[ti.Decl]; prev != nil && prev.holds(ti, resolved, tables, enums, types, partials) {
				e = prev
				memo.Hits++
			}
		}
		if e == nil {
			e = &tableOccs{ti: ti, resolved: resolved, tables: tables, enums: enums, types: types, partials: partials}
			e.sym, e.cols, e.body, e.decls = tableOccurrences(ti.Decl, ti, resolved, tables, enums, types, partials)
			e.declMap = declMapOf(e.decls)
			if memo != nil {
				memo.Misses++
			}
		}
		entries[ti.Decl] = e
	}
	if memo != nil {
		memo.next = entries
		defer func() { memo.prev, memo.next = memo.next, nil }()
	}
	if ix.declTables == nil {
		ix.declTables = make(map[string]*tableOccs, len(entries))
	}

	for _, ti := range info.Tables {
		e := entries[ti.Decl]
		for _, d := range e.decls {
			if strings.HasPrefix(d.id.Container, tableContainer) {
				continue
			}
			if _, exists := ix.decls[d.id]; !exists {
				ix.decls[d.id] = d.ident
			}
		}
		ix.declTables[ti.Key] = e
		ix.Occs = append(ix.Occs, e.sym...)
	}
	for _, ei := range info.Enums {
		ix.add(SymbolID{SymEnum, "", ei.Key}, ei.Decl.Name, true)
		for _, v := range ei.Decl.Values {
			ix.add(SymbolID{SymEnumValue, "enum:" + ei.Key, v.Name.Name()}, v.Name, true)
		}
	}
	for _, td := range info.Types {
		ix.add(SymbolID{SymType, "", td.Key}, lastPart(td.Decl.Name), true)
	}
	for _, pi := range info.Partials {
		ix.add(SymbolID{SymPartial, "", pi.Decl.Name.Name()}, pi.Decl.Name, true)
	}
	for _, pi := range info.Partials {
		for _, item := range pi.Decl.Body {
			if col, ok := item.(*ast.Column); ok {
				ix.add(SymbolID{SymColumn, "partial:" + pi.Decl.Name.Name(), col.Name.Name()}, col.Name, true)
			}
		}
	}
	for _, ti := range info.Tables {
		ix.Occs = append(ix.Occs, entries[ti.Decl].cols...)
	}

	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.Table:
			if e := entries[n]; e != nil {
				ix.Occs = append(ix.Occs, e.body...)
			} else {
				ix.walkTableBody(n.Body, ix.resolveTable(n.Name), "")
			}
		case *ast.TablePartial:
			ix.walkTableBody(n.Body, nil, n.Name.Name())
		case *ast.Ref:
			ix.walkEndpoint(n.Left)
			ix.walkEndpoint(n.Right)
		case *ast.Select:
			if ti := ix.Tables[n.Target.Name()]; ti != nil {
				for _, col := range n.Cols {
					ix.addColumnRef(ti, "", col)
				}
				if o := n.Settings.Get("order"); o != nil {
					if list, ok := o.Value.(*ast.IdentList); ok {
						for _, id := range list.Names {
							ix.addColumnRef(ti, "", id)
						}
					}
				}
				if n.Where != nil {
					ix.walkPredCols(ti, "", n.Where)
				}
			}
		case *ast.Records:
			ti := ix.resolveTable(n.Table)
			if ti != nil {
				ix.add(SymbolID{SymTable, "", ti.Key}, n.Table, false)
			}
			for _, c := range n.Columns {
				ix.addColumnRef(ti, "", c)
			}
			ix.walkRecordRows(n.Rows)
		}
	}
	return ix
}

func tableOccurrences(d *ast.Table, ti, resolved *check.TableInfo, tables map[string]*check.TableInfo, enums map[string]*check.EnumInfo, types map[string]*check.TypeInfo, partials map[string]bool) (sym, cols, body []Occurrence, decls []declEntry) {
	acc := &Index{Tables: map[string]*check.TableInfo{}, Enums: map[string]*check.EnumInfo{}, Types: map[string]*check.TypeInfo{}, Partials: map[string]*check.PartialInfo{}, decls: map[SymbolID]*ast.Ident{}}
	for key, t := range types {
		if t != nil {
			acc.Types[key] = t
		}
	}
	for key, t := range tables {
		if t != nil {
			acc.Tables[key] = t
		}
	}
	for key, e := range enums {
		if e != nil {
			acc.Enums[key] = e
		}
	}
	for name, present := range partials {
		if present {
			acc.Partials[name] = nil
		}
	}
	acc.recording = &decls
	acc.add(SymbolID{SymTable, "", ti.Key}, d.Name, true)
	sym = acc.Occs[:len(acc.Occs):len(acc.Occs)]
	from := len(acc.Occs)
	for _, cd := range ti.Columns {
		if cd.Partial == nil {
			acc.add(SymbolID{SymColumn, "table:" + ti.Key, cd.Col.Name.Name()}, cd.Col.Name, true)
		}
	}
	cols = acc.Occs[from:len(acc.Occs):len(acc.Occs)]
	from = len(acc.Occs)
	acc.walkTableBody(d.Body, resolved, "")
	body = acc.Occs[from:len(acc.Occs):len(acc.Occs)]
	return sym, cols, body, decls
}

func (ix *Index) enumsNamed(d *ast.Table) map[string]*check.EnumInfo {
	out := map[string]*check.EnumInfo{}
	name := func(key string) { out[key] = ix.Enums[key] }
	var settings func(s *ast.SettingList)
	settings = func(s *ast.SettingList) {
		if s == nil {
			return
		}
		for _, st := range s.Settings {
			if ec, ok := st.Value.(*ast.EnumConst); ok {
				name(ec.Enum.Name())
			}
		}
	}
	rows := func(rows []*ast.RecordRow) {
		for _, row := range rows {
			for _, v := range row.Values {
				if ec, ok := v.(*ast.EnumConst); ok {
					name(ec.Enum.Name())
				}
			}
		}
	}
	for _, item := range d.Body {
		switch n := item.(type) {
		case *ast.Column:
			if n.Type != nil && n.Type.Name != nil {
				name(n.Type.Name.String())
			}
			settings(n.Settings)
		case *ast.IndexesBlock:
			for _, index := range n.Indexes {
				settings(index.Settings)
			}
		case *ast.Records:
			rows(n.Rows)
		}
	}
	return out
}

func (ix *Index) tablesNamed(d *ast.Table) map[string]*check.TableInfo {
	out := map[string]*check.TableInfo{}
	endpoint := func(ep *ast.RefEndpoint) {
		if ep == nil || ep.Table == nil {
			return
		}
		key := ep.Table.Name()
		out[key] = ix.Tables[key]
	}
	settings := func(s *ast.SettingList) {
		if s == nil {
			return
		}
		for _, st := range s.Settings {
			if ep, ok := st.Value.(*ast.RefEndpoint); ok {
				endpoint(ep)
			}
		}
	}
	for _, item := range d.Body {
		switch n := item.(type) {
		case *ast.Column:
			settings(n.Settings)
		case *ast.IndexesBlock:
			for _, index := range n.Indexes {
				settings(index.Settings)
			}
		}
	}
	return out
}

func (ix *Index) typesNamed(d *ast.Table) map[string]*check.TypeInfo {
	out := map[string]*check.TypeInfo{}
	for _, item := range d.Body {
		if col, ok := item.(*ast.Column); ok && col.Type != nil && !col.Type.Slice {
			out[col.Type.String()] = ix.Types[col.Type.String()]
		}
	}
	return out
}

func (ix *Index) partialsNamed(d *ast.Table) map[string]bool {
	out := map[string]bool{}
	for _, item := range d.Body {
		if ref, ok := item.(*ast.PartialRef); ok {
			_, present := ix.Partials[ref.Name.Name()]
			out[ref.Name.Name()] = present
		}
	}
	return out
}

func (ix *Index) walkTableBody(body []ast.TableItem, ti *check.TableInfo, partial string) {
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Column:
			if td := ix.Types[n.Type.String()]; td != nil {
				ix.add(SymbolID{SymType, "", td.Key}, lastPart(n.Type.Name), false)
			} else if ei := ix.resolveEnum(n.Type.Name); ei != nil {
				ix.add(SymbolID{SymEnum, "", ei.Key}, lastPart(n.Type.Name), false)
			}
			ix.walkSettings(n.Settings)
		case *ast.PartialRef:
			if _, ok := ix.Partials[n.Name.Name()]; ok {
				ix.add(SymbolID{SymPartial, "", n.Name.Name()}, n.Name, false)
			}
		case *ast.IndexesBlock:
			for _, index := range n.Indexes {
				for _, key := range index.Key {
					if id, ok := key.(*ast.Ident); ok {
						ix.addColumnRef(ti, partial, id)
					}
				}
				ix.walkSettings(index.Settings)
			}
		case *ast.Records:
			for _, c := range n.Columns {
				ix.addColumnRef(ti, "", c)
			}
			ix.walkRecordRows(n.Rows)
		case *ast.ValidatorsBlock:
			for _, v := range n.Validators {
				for _, a := range v.Args {
					ix.addColumnRef(ti, partial, a)
				}
			}
		}
	}
}

func (ix *Index) walkPredCols(ti *check.TableInfo, partial string, x ast.PredExpr) {
	switch x := x.(type) {
	case *ast.PredBinary:
		ix.walkPredCols(ti, partial, x.X)
		ix.walkPredCols(ti, partial, x.Y)
	case *ast.PredNot:
		ix.walkPredCols(ti, partial, x.X)
	case *ast.PredParen:
		ix.walkPredCols(ti, partial, x.X)
	case *ast.PredCompare:
		for _, side := range []ast.Operand{x.X, x.Y} {
			if cr, ok := side.(*ast.ColRef); ok {
				ix.addColumnRef(ti, partial, cr.Name)
			}
		}
	case *ast.PredIn:
		ix.addColumnRef(ti, partial, x.Col)
	case *ast.PredLike:
		ix.addColumnRef(ti, partial, x.Col)
	case *ast.PredNull:
		ix.addColumnRef(ti, partial, x.Col)
	}
}

func (ix *Index) walkSettings(s *ast.SettingList) {
	if s == nil {
		return
	}
	for _, setting := range s.Settings {
		switch v := setting.Value.(type) {
		case *ast.RefEndpoint:
			ix.walkEndpoint(v)
		case *ast.EnumConst:
			ix.walkEnumConst(v)
		}
	}
}

func (ix *Index) walkRecordRows(rows []*ast.RecordRow) {
	for _, row := range rows {
		for _, v := range row.Values {
			if ec, ok := v.(*ast.EnumConst); ok {
				ix.walkEnumConst(ec)
			}
		}
	}
}

func (ix *Index) walkEnumConst(ec *ast.EnumConst) {
	ei, ok := ix.Enums[ec.Enum.Name()]
	if !ok {
		return
	}
	ix.add(SymbolID{SymEnum, "", ei.Key}, ec.Enum, false)
	for _, v := range ei.Decl.Values {
		if v.Name.Name() == ec.Value.Name() {
			ix.add(SymbolID{SymEnumValue, "enum:" + ei.Key, ec.Value.Name()}, ec.Value, false)
			break
		}
	}
}

func (ix *Index) walkEndpoint(ep *ast.RefEndpoint) {
	if ep == nil {
		return
	}
	ti := ix.resolveTable(ep.Table)
	if ti == nil {
		return
	}
	ix.add(SymbolID{SymTable, "", ti.Key}, ep.Table, false)
	for _, col := range ep.Columns {
		ix.addColumnRef(ti, "", col)
	}
}

func (ix *Index) addColumnRef(ti *check.TableInfo, partial string, id *ast.Ident) {
	if id == nil {
		return
	}
	if partial != "" {
		if pi, ok := ix.Partials[partial]; ok {
			for _, item := range pi.Decl.Body {
				if col, ok := item.(*ast.Column); ok && col.Name.Name() == id.Name() {
					ix.add(SymbolID{SymColumn, "partial:" + partial, id.Name()}, id, false)
					return
				}
			}
		}
		return
	}
	if ti == nil {
		return
	}
	cd := ti.Column(id.Name())
	if cd == nil {
		return
	}
	container := "table:" + ti.Key
	if cd.Partial != nil {
		container = "partial:" + cd.Partial.Name.Name()
	}
	ix.add(SymbolID{SymColumn, container, id.Name()}, id, false)
}

func (ix *Index) resolveTable(id *ast.Ident) *check.TableInfo {
	if id == nil {
		return nil
	}
	return ix.Tables[id.Name()]
}

func (ix *Index) resolveEnum(q *ast.QualName) *check.EnumInfo {
	if q == nil {
		return nil
	}
	return ix.Enums[q.String()]
}

func (ix *Index) add(id SymbolID, ident *ast.Ident, isDecl bool) {
	if ident == nil {
		return
	}
	ix.Occs = append(ix.Occs, Occurrence{ID: id, Ident: ident, IsDecl: isDecl})
	if isDecl {
		if _, exists := ix.decls[id]; !exists {
			ix.decls[id] = ident
		}
		if ix.recording != nil {
			*ix.recording = append(*ix.recording, declEntry{id, ident})
		}
	}
}

// At returns the occurrence whose identifier spans the byte offset.
func (ix *Index) At(offset int) *Occurrence {
	for i := range ix.Occs {
		tok := ix.Occs[i].Ident.Tok
		if int(tok.Pos.Offset()) <= offset && offset <= int(tok.End().Offset()) {
			return &ix.Occs[i]
		}
	}
	return nil
}

// Decl returns the declaring identifier of a symbol, or nil.
func (ix *Index) Decl(id SymbolID) *ast.Ident {
	if ident := ix.decls[id]; ident != nil {
		return ident
	}
	if key, ok := strings.CutPrefix(id.Container, tableContainer); ok {
		if e := ix.declTables[key]; e != nil {
			return e.declMap[id]
		}
	}
	return nil
}

// OccurrencesOf returns all occurrences of a symbol.
func (ix *Index) OccurrencesOf(id SymbolID) []Occurrence {
	var out []Occurrence
	for _, o := range ix.Occs {
		if o.ID == id {
			out = append(out, o)
		}
	}
	return out
}

func lastPart(q *ast.QualName) *ast.Ident {
	if q == nil || len(q.Parts) == 0 {
		return nil
	}
	return q.Parts[len(q.Parts)-1]
}
