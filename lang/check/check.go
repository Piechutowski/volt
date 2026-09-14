// Package check implements semantic analysis of a parsed DBML file — the
// role go/types plays for Go. It validates every constraint of the spec
// that goes beyond grammar (setting whitelists, duplicate names, reference
// resolution, records arity) and builds an Info symbol table that the vet
// package reuses, so analysis is done once.
//
// Diagnostic codes cite the spec section they enforce, e.g. "spec/6.7".
package check

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"sort"
)

// Info is the semantic model of one file, produced by File.
type Info struct {
	Tables     []*TableInfo
	Enums      []*EnumInfo
	Partials   []*PartialInfo
	Refs       []*RefInfo
	HasImports bool
	// TableGroups in declaration order, resolved to their member tables
	// (unknown members dropped; §6.12 reports them). A TableGroup is a
	// set wherever Volt names one (spec §V9.2, D65).
	TableGroups []*TableGroupInfo

	byTable map[string]*TableInfo // canonical key and alias -> table
	byBase  map[string]*TableInfo // bare name -> the first table declared with it (D84)
}

// TableByBase is the first table declared with the bare name, in any
// schema, or nil: what an unqualified reference in a select or a
// resources line means.
func (i *Info) TableByBase(base string) *TableInfo { return i.byBase[base] }

// TableByKey is the table with the canonical "schema.name" key, or nil.
func (i *Info) TableByKey(key string) *TableInfo { return i.byTable[key] }

// TableGroupInfo is one TableGroup with its members resolved.
type TableGroupInfo struct {
	Decl    *ast.TableGroup
	Members []*TableInfo // declaration order, unresolved names omitted
}

// TableGroup finds a TableGroup by exact name.
func (i *Info) TableGroup(name string) *TableGroupInfo {
	for _, g := range i.TableGroups {
		if g.Decl.Name.Name() == name {
			return g
		}
	}
	return nil
}

// LookupTable finds a table by its qualified name (canonical key or alias).
func (i *Info) LookupTable(q *ast.QualName) *TableInfo {
	return i.byTable[canonKey(q)]
}

// TableInfo describes one table with partial injection applied.
type TableInfo struct {
	Decl  *ast.Table
	Key   string // canonical "schema.name" with the public default applied
	Alias string // "" if none

	// Columns is the effective column list in definition order after
	// injection, conflicts resolved per spec §6.9.4.
	Columns []*ColumnDef
	// Shadowed lists column definitions that lost a §6.9.4 conflict.
	Shadowed []*ColumnDef

	Indexes []*ast.Index // all indexes, direct and injected
	Checks  []*ast.Check // all check lines, direct and injected (§6.9.3)
}

// ColumnDef is one column definition and where it came from.
type ColumnDef struct {
	Col     *ast.Column
	Partial *ast.TablePartial // nil for direct definitions
}

// Column returns the effective definition of the named column, or nil.
func (t *TableInfo) Column(name string) *ColumnDef {
	for _, c := range t.Columns {
		if c.Col.Name.Name() == name {
			return c
		}
	}
	return nil
}

type EnumInfo struct {
	Decl *ast.Enum
	Key  string
	Uses int // column type references within this file
}

type PartialInfo struct {
	Decl *ast.TablePartial
	Uses int
}

// RefInfo is one resolved (or unresolved) relationship, from any of the
// three syntactic forms.
type RefInfo struct {
	Node   ast.Node // *ast.Ref, or the *ast.Setting of an inline ref
	Op     token.Kind
	Inline bool
	Left   EndpointInfo
	Right  EndpointInfo
}

type EndpointInfo struct {
	Table   *TableInfo // nil if unresolved
	Columns []string
	Pos     token.Position
}

// canonKey normalizes a table name with the default schema (spec §8.1).
func canonKey(q *ast.QualName) string {
	if q.Schema() == "" {
		return "public." + q.Base()
	}
	return q.String()
}

// File checks one parsed file and returns its semantic model together with
// all diagnostics. When the file imports other files (§7), unresolved-name
// errors are suppressed, because the definitions may live elsewhere; all
// local constraints still apply.
func File(f *ast.File) (*Info, []diag.Diagnostic) {
	return FileMemo(f, nil)
}

// FileMemo is File with a memo of the file's earlier checks: a table
// whose declaration and injected partials are the nodes they were is
// answered from it (D84). A nil memo checks everything.
func FileMemo(f *ast.File, memo *Memo) (*Info, []diag.Diagnostic) {
	c := &checker{
		info: &Info{
			HasImports: f.HasImports(),
			byTable:    map[string]*TableInfo{},
			byBase:     map[string]*TableInfo{},
		},
		memo: memo,
	}
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
	}
	c.collect(f)
	c.declsCheck(f)
	c.resolve(f)
	if memo != nil {
		memo.finish()
	}
	diag.Sort(c.diags)
	return c.info, c.diags
}

type checker struct {
	info  *Info
	diags []diag.Diagnostic
	memo  *Memo
	// enumSet is the file's enums as the table check needs them: a
	// pure input with a signature the memo can compare (D86).
	enumSet *enumSet

	partials map[string]*PartialInfo
	enums    map[string]*EnumInfo
	groups   map[string]*ast.TableGroup
	sticky   map[string]*ast.StickyNote
	views    map[string]*ast.DiagramView
	schemas  map[string]bool
	project  *ast.Project
}

func (c *checker) errorf(pos token.Position, section, format string, args ...any) {
	c.diags = append(c.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

/* ===== pass 1: collect declarations, detect duplicate names (§8.2) ===== */

func (c *checker) collect(f *ast.File) {
	c.partials = map[string]*PartialInfo{}
	c.enums = map[string]*EnumInfo{}
	c.groups = map[string]*ast.TableGroup{}
	c.sticky = map[string]*ast.StickyNote{}
	c.views = map[string]*ast.DiagramView{}
	c.schemas = map[string]bool{"public": true}

	// Partials first: tables need them for injection regardless of order (§5.2).
	for _, d := range f.Decls {
		if tp, ok := d.(*ast.TablePartial); ok {
			if _, dup := c.partials[tp.Name.Name()]; dup {
				c.errorf(tp.Pos(), "8.2", "duplicate TablePartial name %q", tp.Name.Name())
				continue
			}
			c.partials[tp.Name.Name()] = &PartialInfo{Decl: tp}
		}
	}

	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.Table:
			key := canonKey(d.Name)
			if _, dup := c.info.byTable[key]; dup {
				c.errorf(d.Pos(), "8.2", "duplicate table name %q", d.Name.String())
				continue
			}
			ti := &TableInfo{Decl: d, Key: key}
			if d.Alias != nil {
				ti.Alias = d.Alias.Name()
				aliasKey := "public." + ti.Alias
				if _, dup := c.info.byTable[aliasKey]; dup {
					c.errorf(d.Alias.Pos(), "8.2", "table alias %q collides with an existing table name or alias", ti.Alias)
				} else {
					c.info.byTable[aliasKey] = ti
				}
			}
			c.info.byTable[key] = ti
			if _, seen := c.info.byBase[d.Name.Base()]; !seen {
				c.info.byBase[d.Name.Base()] = ti
			}
			c.info.Tables = append(c.info.Tables, ti)
			if s := d.Name.Schema(); s != "" {
				c.schemas[s] = true
			}
		case *ast.Enum:
			key := canonKey(d.Name)
			if _, dup := c.enums[key]; dup {
				c.errorf(d.Pos(), "8.2", "duplicate enum name %q", d.Name.String())
				continue
			}
			ei := &EnumInfo{Decl: d, Key: key}
			c.enums[key] = ei
			c.info.Enums = append(c.info.Enums, ei)
			if s := d.Name.Schema(); s != "" {
				c.schemas[s] = true
			}
		case *ast.TableGroup:
			if _, dup := c.groups[d.Name.Name()]; dup {
				c.errorf(d.Pos(), "8.2", "duplicate TableGroup name %q", d.Name.Name())
				continue
			}
			c.groups[d.Name.Name()] = d
		case *ast.StickyNote:
			if _, dup := c.sticky[d.Name.Name()]; dup {
				c.errorf(d.Pos(), "8.2", "duplicate sticky note name %q", d.Name.Name())
				continue
			}
			c.sticky[d.Name.Name()] = d
		case *ast.DiagramView:
			if _, dup := c.views[d.Name.Name()]; dup {
				c.errorf(d.Pos(), "8.2", "duplicate DiagramView name %q", d.Name.Name())
				continue
			}
			c.views[d.Name.Name()] = d
		case *ast.Project:
			if c.project != nil {
				c.errorf(d.Pos(), "6.1", "at most one Project element per file")
				continue
			}
			c.project = d
		}
	}
	for _, pi := range c.partials {
		c.info.Partials = append(c.info.Partials, pi)
	}

	// Every table's check is a pure function of its declaration, the
	// partials it injects, the enum set and the imports flag (D86):
	// answered from the memo when those are what they were, computed
	// otherwise. The use counts are the caller's, as every side effect.
	c.enumSet = enumSetOf(c.enums)
	for i, ti := range c.info.Tables {
		partials := c.partialsOf(ti.Decl)
		for _, p := range partials {
			if p != nil {
				c.partials[p.Name.Name()].Uses++
			}
		}
		var checked *TableInfo
		var diags []diag.Diagnostic
		if e := c.memo.lookup(ti.Decl, partials, c.info.HasImports, c.enumSet.keys); e != nil {
			checked, diags = e.ti, e.diags
			c.memo.store(e)
			c.memo.Hits++
		} else {
			checked, diags = tableCheck(ti.Decl, ti.Key, ti.Alias, partials, c.info.HasImports, c.enumSet)
			if c.memo != nil {
				c.memo.store(&memoTable{ti: checked, partials: partials, hasImports: c.info.HasImports, enums: c.enumSet.keys, diags: diags})
				c.memo.Misses++
			}
		}
		if checked != ti {
			c.info.Tables[i] = checked
			c.info.byTable[checked.Key] = checked
			if c.info.byBase[checked.Decl.Name.Base()] == ti {
				c.info.byBase[checked.Decl.Name.Base()] = checked
			}
			if checked.Alias != "" {
				c.info.byTable["public."+checked.Alias] = checked
			}
		}
		c.diags = append(c.diags, diags...)
	}
}

// enumSet is the enums a table check consults, by canonical key, and
// the same keys sorted: the set itself as a memo key, compared element
// by element (D91). Two equal key lists answer every presence question
// alike; no spelling of the set stands in for it.
type enumSet struct {
	present map[string]bool
	keys    []string
}

func enumSetOf(enums map[string]*EnumInfo) *enumSet {
	keys := make([]string, 0, len(enums))
	for k := range enums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	set := &enumSet{present: make(map[string]bool, len(enums)), keys: keys}
	for _, k := range keys {
		set.present[k] = true
	}
	return set
}

// tableCheck is one table's check as a pure function of its inputs
// (D86): the declaration, its key and alias, the partial declarations
// it injects in body order (nil where the name resolves to none),
// whether imports are in play, and the enum set. It returns the
// checked table — columns expanded per §6.9.4, indexes and checks
// gathered — and every diagnostic of the table's body, columns,
// expansion, increment and required rules. It reads nothing else and
// writes nothing it did not create.
func tableCheck(d *ast.Table, key, alias string, partials []*ast.TablePartial, hasImports bool, enums *enumSet) (*TableInfo, []diag.Diagnostic) {
	acc := &checker{info: &Info{HasImports: hasImports}, enumSet: enums}
	ti := &TableInfo{Decl: d, Key: key, Alias: alias}
	acc.columnsExpand(ti, partials)
	acc.tableBodyCheck(d.Name.String(), d.Settings, d.Body, true)
	return ti, acc.diags
}

// partialsOf lists the partial declarations a table injects, in body
// order, nil where the name resolves to none.
func (c *checker) partialsOf(d *ast.Table) []*ast.TablePartial {
	var out []*ast.TablePartial
	for _, item := range d.Body {
		if ref, ok := item.(*ast.PartialRef); ok {
			var p *ast.TablePartial
			if pi := c.partials[ref.Name.Name()]; pi != nil {
				p = pi.Decl
			}
			out = append(out, p)
		}
	}
	return out
}

// columnsExpand applies §6.9.4 conflict resolution: direct definitions win;
// otherwise the last-injected partial wins.
func (c *checker) columnsExpand(ti *TableInfo, partials []*ast.TablePartial) {
	type slot struct {
		def   *ColumnDef
		order int
	}
	byName := map[string]*slot{}
	direct := map[string]bool{}
	order := 0
	addCol := func(col *ast.Column, from *ast.TablePartial) {
		name := col.Name.Name()
		def := &ColumnDef{Col: col, Partial: from}
		if s, ok := byName[name]; ok {
			if from == nil && direct[name] {
				// duplicate direct definition — an error, reported in declsCheck
				return
			}
			if from == nil || !direct[name] {
				// direct wins; among partials, the later injection wins
				ti.Shadowed = append(ti.Shadowed, s.def)
				s.def = def
				direct[name] = from == nil
			} else {
				ti.Shadowed = append(ti.Shadowed, def)
			}
			return
		}
		byName[name] = &slot{def: def, order: order}
		direct[name] = from == nil
		order++
	}
	injected := 0
	for _, item := range ti.Decl.Body {
		switch item := item.(type) {
		case *ast.Column:
			addCol(item, nil)
		case *ast.PartialRef:
			p := partials[injected]
			injected++
			if p == nil {
				if !c.info.HasImports {
					c.errorf(item.Pos(), "6.9", "unknown TablePartial %q", item.Name.Name())
				}
				continue
			}
			for _, pit := range p.Body {
				switch pit := pit.(type) {
				case *ast.Column:
					addCol(pit, p)
				case *ast.IndexesBlock:
					ti.Indexes = append(ti.Indexes, pit.Indexes...)
				case *ast.ChecksBlock:
					ti.Checks = append(ti.Checks, pit.Checks...)
				}
			}
		case *ast.IndexesBlock:
			ti.Indexes = append(ti.Indexes, item.Indexes...)
		case *ast.ChecksBlock:
			ti.Checks = append(ti.Checks, item.Checks...)
		}
	}
	slots := make([]*slot, 0, len(byName))
	for _, s := range byName {
		slots = append(slots, s)
	}
	for i := 0; i < len(slots); i++ {
		for j := i + 1; j < len(slots); j++ {
			if slots[j].order < slots[i].order {
				slots[i], slots[j] = slots[j], slots[i]
			}
		}
	}
	for _, s := range slots {
		ti.Columns = append(ti.Columns, s.def)
	}
	c.incrementCheck(ti)
	c.requiredCheck(ti)
}
