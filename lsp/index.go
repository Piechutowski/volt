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
)

// SymbolID names one symbol. Tables and enums use their canonical
// "schema.name" key; columns live in a container — "table:<key>" for direct
// definitions or "partial:<name>" for columns defined in a TablePartial, so
// every table sharing the partial shares the symbol (a rename edits the
// partial once). Enum values live in "enum:<key>".
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

	Tables   map[string]*check.TableInfo // canonical key AND alias -> table
	Enums    map[string]*check.EnumInfo  // canonical key -> enum
	Partials map[string]*check.PartialInfo

	decls  map[SymbolID]*ast.Ident
	byBase map[string]*check.TableInfo // bare name -> the one table with it; absent when none or several

	// declTables holds the column declarations of the tables the memo
	// reused, by table key: their entries stay in the memo's per-table
	// maps instead of being copied into decls on every build (D85).
	declTables map[string]*tableOccs
	recording  *[]declEntry // the declarations a table's build adds, for its memo entry
}

// IndexMemo remembers a package's per-table occurrences across builds
// of its index (D85): a table whose declaration node and checked table
// are the objects they were contributes the occurrences it did. The
// zero value is ready; a memo belongs to one package and one goroutine
// at a time, and drops what a build did not use.
type IndexMemo struct {
	prev, next   map[*ast.Table]*tableOccs
	lastLen      int // occurrences of the last build, to size the next
	Hits, Misses int
}

// tableOccs is one call of tableOccurrences: its inputs (the
// declaration is the map key) and its outputs.
type tableOccs struct {
	ti, resolved    *check.TableInfo
	tables          map[string]*check.TableInfo
	enums           map[string]*check.EnumInfo
	partials        map[string]bool
	sym, cols, body []Occurrence // the three places a table's occurrences land, in build order
	decls           []declEntry
	declMap         map[SymbolID]*ast.Ident // decls with a "table:" container, built with the entry
}

// holds reports whether the entry's inputs are the given ones: the
// tables by identity, the enums by their declarations' identity, the
// partials by presence.
func (e *tableOccs) holds(ti, resolved *check.TableInfo, tables map[string]*check.TableInfo, enums map[string]*check.EnumInfo, partials map[string]bool) bool {
	if e.ti != ti || e.resolved != resolved || len(e.tables) != len(tables) || len(e.enums) != len(enums) || len(e.partials) != len(partials) {
		return false
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

// tableContainer is the container prefix of a column declared in a table.
const tableContainer = "table:"

// declMapOf is the per-table declaration map of an entry's declarations.
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

// canon normalizes a possibly-qualified name with the default schema.
func canon(q *ast.QualName) string {
	if q.Schema() == "" {
		return "public." + q.Base()
	}
	return q.String()
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
		Partials: map[string]*check.PartialInfo{},
		decls:    map[SymbolID]*ast.Ident{},
		byBase:   map[string]*check.TableInfo{},
	}
	ambiguous := map[string]bool{}
	for _, ti := range info.Tables {
		ix.Tables[ti.Key] = ti
		if ti.Alias != "" {
			ix.Tables["alias:"+ti.Alias] = ti
		}
		base := ti.Decl.Name.Base()
		if prev, seen := ix.byBase[base]; seen && prev != ti {
			ambiguous[base] = true
		} else if !seen {
			ix.byBase[base] = ti
		}
	}
	for base := range ambiguous {
		delete(ix.byBase, base)
	}
	for _, ei := range info.Enums {
		ix.Enums[ei.Key] = ei
	}
	for _, pi := range info.Partials {
		ix.Partials[pi.Decl.Name.Name()] = pi
	}

	// Every table's occurrences are tableOccurrences, a pure function
	// of the inputs resolved here (D86); the memo answers a table whose
	// inputs are what they were. An entry's lists are appended where
	// the build puts a table's occurrences: its symbol with the
	// declarations, its columns after the partials', its body with the
	// references.
	entries := make(map[*ast.Table]*tableOccs, len(info.Tables))
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
		// Sized from the last build: the occurrences are appended in
		// one pass and the slice must not grow by copying under them.
		ix.Occs = make([]Occurrence, 0, memo.lastLen)
		defer func() { memo.lastLen = len(ix.Occs) }()
	}
	for _, ti := range info.Tables {
		resolved := ix.resolveTable(ti.Decl.Name) // the table the body's references bind to
		tables := ix.tablesNamed(ti.Decl)
		enums := ix.enumsNamed(ti.Decl)
		partials := ix.partialsNamed(ti.Decl)
		var e *tableOccs
		if memo != nil {
			if prev := memo.prev[ti.Decl]; prev != nil && prev.holds(ti, resolved, tables, enums, partials) {
				e = prev
				memo.Hits++
			}
		}
		if e == nil {
			e = &tableOccs{ti: ti, resolved: resolved, tables: tables, enums: enums, partials: partials}
			e.sym, e.cols, e.body, e.decls = tableOccurrences(ti.Decl, ti, resolved, tables, enums, partials)
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

	// --- declarations ---
	for _, ti := range info.Tables {
		e := entries[ti.Decl]
		// The table's own symbol goes into the shared map; its columns
		// stay in the entry's map, reached through Decl.
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
		ix.add(SymbolID{SymEnum, "", ei.Key}, lastPart(ei.Decl.Name), true)
		for _, v := range ei.Decl.Values {
			ix.add(SymbolID{SymEnumValue, "enum:" + ei.Key, v.Name.Name()}, v.Name, true)
		}
	}
	for _, pi := range info.Partials {
		ix.add(SymbolID{SymPartial, "", pi.Decl.Name.Name()}, pi.Decl.Name, true)
	}
	// Column declarations, container-aware: a column written in a partial is
	// declared once even when injected into many tables.
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

	// Groups shadow tables as select targets (§V11.2): a group-targeted
	// select names agreed columns, not one table's.
	groups := map[string]bool{}
	for _, decl := range f.Decls {
		if g, ok := decl.(*ast.Group); ok {
			groups[g.Name.Name()] = true
		}
	}

	// --- references ---
	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.Table:
			if e := entries[n]; e != nil {
				ix.Occs = append(ix.Occs, e.body...)
			} else {
				// A duplicate declaration the schema check skipped: its
				// references still bind to the table that won.
				ix.walkTableBody(n.Body, ix.resolveTable(n.Name), "")
			}
		case *ast.TablePartial:
			ix.walkTableBody(n.Body, nil, n.Name.Name())
		case *ast.Ref:
			ix.walkEndpoint(n.Left)
			ix.walkEndpoint(n.Right)
		case *ast.Select:
			// A select over one table binds its columns to that table:
			// projection, order and where follow a rename. A group target
			// names an agreed column across members and is left alone —
			// the §V11.4 error then says what went missing.
			if ti := ix.tableByBase(n.Target.Name()); ti != nil && !groups[n.Target.Name()] {
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
				ix.add(SymbolID{SymTable, "", ti.Key}, lastPart(n.Table), false)
			}
			for _, c := range n.Columns {
				ix.addColumnRef(ti, "", c)
			}
			ix.walkRecordRows(n.Rows)
		case *ast.TableGroup:
			for _, m := range n.Members {
				if ti := ix.resolveTable(m); ti != nil {
					ix.add(SymbolID{SymTable, "", ti.Key}, lastPart(m), false)
				}
			}
		case *ast.DiagramView:
			for _, cat := range n.Categories {
				if !equalFold(cat.Kind.Name(), "tables") {
					continue
				}
				for _, m := range cat.Names {
					if ti := ix.resolveTable(m); ti != nil {
						ix.add(SymbolID{SymTable, "", ti.Key}, lastPart(m), false)
					}
				}
			}
		}
	}
	return ix
}

// tableOccurrences is one table's contribution to the index as a pure
// function of its inputs (D86): its declaration, its checked table,
// the table its body's references bind to (the checked one, or the
// namesake that won when this one is a duplicate), the tables its
// inline references name by lookup key, the enums the body names by
// canonical key, and the partials it injects by presence.
// It returns the occurrences of the table's symbol and alias, of its
// own column declarations, and of every reference inside its body,
// plus the declarations it contributes, reading nothing else and
// writing nothing it did not create.
func tableOccurrences(d *ast.Table, ti, resolved *check.TableInfo, tables map[string]*check.TableInfo, enums map[string]*check.EnumInfo, partials map[string]bool) (sym, cols, body []Occurrence, decls []declEntry) {
	// The walk asks its maps whether a key is declared, so the inputs'
	// "named but absent" entries (nil) must not be present in them.
	acc := &Index{Tables: map[string]*check.TableInfo{}, Enums: map[string]*check.EnumInfo{}, Partials: map[string]*check.PartialInfo{}, decls: map[SymbolID]*ast.Ident{}}
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
	id := SymbolID{SymTable, "", ti.Key}
	acc.add(id, lastPart(d.Name), true)
	if d.Alias != nil {
		acc.add(id, d.Alias, false)
	}
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

// enumsNamed is the enums a table's body names, by canonical key, nil
// where the name resolves to none: the column types, and the enum
// constants in settings and records.
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
				name("public." + ec.Enum.Name())
			}
		}
	}
	rows := func(rows []*ast.RecordRow) {
		for _, row := range rows {
			for _, v := range row.Values {
				if ec, ok := v.(*ast.EnumConst); ok {
					name("public." + ec.Enum.Name())
				}
			}
		}
	}
	for _, item := range d.Body {
		switch n := item.(type) {
		case *ast.Column:
			if n.Type != nil && n.Type.Name != nil {
				name(canon(n.Type.Name))
			}
			settings(n.Settings)
		case *ast.IndexesBlock:
			for _, index := range n.Indexes {
				settings(index.Settings)
			}
		case *ast.Records:
			rows(n.Rows)
		case *ast.ChecksBlock:
			for _, c := range n.Checks {
				settings(c.Settings)
			}
		}
	}
	return out
}

// tablesNamed is the tables a table's body names in inline references
// (§6.7), by the keys the walk looks them up with — the canonical
// name, and the alias form of an unqualified one — nil where a key
// resolves to none.
func (ix *Index) tablesNamed(d *ast.Table) map[string]*check.TableInfo {
	out := map[string]*check.TableInfo{}
	endpoint := func(ep *ast.RefEndpoint) {
		if ep == nil || ep.Table == nil {
			return
		}
		key := canon(ep.Table)
		out[key] = ix.Tables[key]
		if ep.Table.Schema() == "" {
			alias := "alias:" + ep.Table.Base()
			out[alias] = ix.Tables[alias]
		}
	}
	settings := func(s *ast.SettingList) {
		if s == nil {
			return
		}
		for _, st := range s.Settings {
			if rv, ok := st.Value.(*ast.RefValue); ok {
				endpoint(rv.Endpoint)
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
		case *ast.ChecksBlock:
			for _, c := range n.Checks {
				settings(c.Settings)
			}
		}
	}
	return out
}

// partialsNamed is the partials a table's body injects, by name, with
// whether each is declared.
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

// walkTableBody records references inside one Table or TablePartial body.
// Exactly one of ti / partial is set and names the column container.
func (ix *Index) walkTableBody(body []ast.TableItem, ti *check.TableInfo, partial string) {
	for _, item := range body {
		switch n := item.(type) {
		case *ast.Column:
			// column type may reference an enum
			if ei := ix.resolveEnum(n.Type.Name); ei != nil {
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
		case *ast.ChecksBlock:
			for _, c := range n.Checks {
				ix.walkSettings(c.Settings)
				if c.Pred != nil {
					ix.walkPredCols(ti, partial, c.Pred)
				}
				for _, a := range c.Args {
					ix.addColumnRef(ti, partial, a)
				}
			}
		}
	}
}

// walkPredCols records the column references of a typed check (§V12):
// every operand names a column of the enclosing table, so a rename of
// the column must follow it into the check.
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

// walkSettings records references in setting values: inline refs and enum
// constants (default: status.active).
func (ix *Index) walkSettings(s *ast.SettingList) {
	if s == nil {
		return
	}
	for _, setting := range s.Settings {
		switch v := setting.Value.(type) {
		case *ast.RefValue:
			ix.walkEndpoint(v.Endpoint)
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
	key := "public." + ec.Enum.Name()
	ei, ok := ix.Enums[key]
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

// walkEndpoint records the table and column references of one ref endpoint.
func (ix *Index) walkEndpoint(ep *ast.RefEndpoint) {
	if ep == nil {
		return
	}
	ti := ix.resolveTable(ep.Table)
	if ti == nil {
		return
	}
	ix.add(SymbolID{SymTable, "", ti.Key}, lastPart(ep.Table), false)
	for _, col := range ep.Columns {
		ix.addColumnRef(ti, "", col)
	}
}

// addColumnRef resolves a column identifier against its table (or partial)
// container and records the occurrence.
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

// tableByBase finds the one table with the given base name (the
// checker's §V11.2 rule for select targets), or nil when none or more
// than one matches.
func (ix *Index) tableByBase(base string) *check.TableInfo {
	return ix.byBase[base]
}

// resolveTable resolves a table name or alias to its info.
func (ix *Index) resolveTable(q *ast.QualName) *check.TableInfo {
	if q == nil {
		return nil
	}
	if ti, ok := ix.Tables[canon(q)]; ok {
		return ti
	}
	if q.Schema() == "" {
		if ti, ok := ix.Tables["alias:"+q.Base()]; ok {
			return ti
		}
	}
	return nil
}

// resolveEnum resolves a type name to an enum, if it names one.
func (ix *Index) resolveEnum(q *ast.QualName) *check.EnumInfo {
	if q == nil {
		return nil
	}
	if ei, ok := ix.Enums[canon(q)]; ok {
		return ei
	}
	return nil
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

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
