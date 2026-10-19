// Package check implements semantic analysis of a parsed DBML file — the
// role go/types plays for Go. It validates every constraint of the spec
// that goes beyond grammar (setting whitelists, duplicate names, reference
// resolution, records arity) and builds an Info symbol table.
//
// Diagnostic codes cite the spec section they enforce, e.g. "spec/relationships-ref".
package check

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// Info is the semantic model of one file, produced by File.
type Info struct {
	Tables     []*TableInfo
	Enums      []*EnumInfo
	Types      []*TypeInfo
	Partials   []*PartialInfo
	Refs       []*RefInfo
	HasImports bool
	// TableGroups in declaration order, resolved to their member tables
	// (unknown members dropped; §tablegroup reports them). A TableGroup is a
	// set wherever Volt names one (spec §groups).
	TableGroups []*TableGroupInfo

	byTable map[string]*TableInfo
	byBase  map[string]*TableInfo
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
	// injection, conflicts resolved per spec §tablepartial.
	Columns []*ColumnDef
	// Shadowed lists column definitions that lost a §tablepartial conflict.
	Shadowed []*ColumnDef

	Indexes    []*ast.Index // all indexes, direct and injected
	Validators []*ast.Validator
}

// ColumnDef is one column definition and where it came from.
type ColumnDef struct {
	Col     *ast.Column
	Partial *ast.TablePartial // nil for direct definitions
	Type    ColumnType
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

func canonKey(q *ast.QualName) string {
	if q.Schema() == "" {
		return "public." + q.Base()
	}
	return q.String()
}

// File checks one parsed file and returns its semantic model together with
// all diagnostics. When the file imports other files (§file-imports-removed), unresolved-name
// errors are suppressed; all local constraints still apply.
func File(f *ast.File) (*Info, []diag.Diagnostic) {
	return FileMemo(f, nil)
}

// FileMemo is File with a memo of the file's earlier checks: a table
// whose declaration and injected partials are the nodes they were is
// answered from it. A nil memo checks everything.
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
	env   *typeEnv

	partials map[string]*PartialInfo
	enums    map[string]*EnumInfo
	types    map[string]*TypeInfo
	groups   map[string]*ast.TableGroup
	sticky   map[string]*ast.StickyNote
	views    map[string]*ast.DiagramView
	schemas  map[string]bool
	project  *ast.Project
}

func (c *checker) errorf(pos token.Position, section, format string, args ...any) {
	c.diags = append(c.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

func (c *checker) collect(f *ast.File) {
	c.partials = map[string]*PartialInfo{}
	c.enums = map[string]*EnumInfo{}
	c.types = map[string]*TypeInfo{}
	var typeDecls []*ast.TypeDecl
	c.groups = map[string]*ast.TableGroup{}
	c.sticky = map[string]*ast.StickyNote{}
	c.views = map[string]*ast.DiagramView{}
	c.schemas = map[string]bool{"public": true}

	for _, d := range f.Decls {
		if tp, ok := d.(*ast.TablePartial); ok {
			if _, dup := c.partials[tp.Name.Name()]; dup {
				c.errorf(tp.Pos(), "namespaces-and-uniqueness", "duplicate TablePartial name %q", tp.Name.Name())
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
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate table name %q", d.Name.String())
				continue
			}
			ti := &TableInfo{Decl: d, Key: key}
			if d.Alias != nil {
				ti.Alias = d.Alias.Name()
				aliasKey := "public." + ti.Alias
				if _, dup := c.info.byTable[aliasKey]; dup {
					c.errorf(d.Alias.Pos(), "namespaces-and-uniqueness", "table alias %q collides with an existing table name or alias", ti.Alias)
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
		case *ast.TypeDecl:
			typeDecls = append(typeDecls, d)
		case *ast.Enum:
			key := canonKey(d.Name)
			if _, dup := c.enums[key]; dup {
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate enum name %q", d.Name.String())
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
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate TableGroup name %q", d.Name.Name())
				continue
			}
			c.groups[d.Name.Name()] = d
		case *ast.StickyNote:
			if _, dup := c.sticky[d.Name.Name()]; dup {
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate sticky note name %q", d.Name.Name())
				continue
			}
			c.sticky[d.Name.Name()] = d
		case *ast.DiagramView:
			if _, dup := c.views[d.Name.Name()]; dup {
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate DiagramView name %q", d.Name.Name())
				continue
			}
			c.views[d.Name.Name()] = d
		case *ast.Project:
			if c.project != nil {
				c.errorf(d.Pos(), "project", "at most one Project element per file")
				continue
			}
			c.project = d
		}
	}
	for _, pi := range c.partials {
		c.info.Partials = append(c.info.Partials, pi)
	}

	for _, d := range typeDecls {
		if ti := c.typeDeclCheck(d); ti != nil {
			c.types[ti.Key] = ti
			c.info.Types = append(c.info.Types, ti)
		}
	}
	c.env = typeEnvOf(c.enums, c.types)
	for i, ti := range c.info.Tables {
		partials := c.partialsOf(ti.Decl)
		for _, p := range partials {
			if p != nil {
				c.partials[p.Name.Name()].Uses++
			}
		}
		var checked *TableInfo
		var diags []diag.Diagnostic
		if e := c.memo.lookup(ti.Decl, partials, c.info.HasImports, c.env.keys); e != nil {
			checked, diags = e.ti, e.diags
			c.memo.store(e)
			c.memo.Hits++
		} else {
			checked, diags = tableCheck(ti.Decl, ti.Key, ti.Alias, partials, c.info.HasImports, c.env)
			if c.memo != nil {
				c.memo.store(&memoTable{ti: checked, partials: partials, hasImports: c.info.HasImports, enums: c.env.keys, diags: diags})
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

func tableCheck(d *ast.Table, key, alias string, partials []*ast.TablePartial, hasImports bool, env *typeEnv) (*TableInfo, []diag.Diagnostic) {
	acc := &checker{info: &Info{HasImports: hasImports}, env: env}
	ti := &TableInfo{Decl: d, Key: key, Alias: alias}
	acc.columnsExpand(ti, partials)
	acc.tableBodyCheck(d.Pos(), d.Name.String(), d.Settings, d.Body, true)
	return ti, acc.diags
}

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
				return
			}
			if from == nil || !direct[name] {
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
					c.errorf(item.Pos(), "tablepartial", "unknown TablePartial %q", item.Name.Name())
				}
				continue
			}
			for _, pit := range p.Body {
				switch pit := pit.(type) {
				case *ast.Column:
					addCol(pit, p)
				case *ast.IndexesBlock:
					ti.Indexes = append(ti.Indexes, pit.Indexes...)
				case *ast.ValidatorsBlock:
					ti.Validators = append(ti.Validators, pit.Validators...)
				}
			}
		case *ast.IndexesBlock:
			ti.Indexes = append(ti.Indexes, item.Indexes...)
		case *ast.ValidatorsBlock:
			ti.Validators = append(ti.Validators, item.Validators...)
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
	c.typesResolve(ti)
	for _, item := range ti.Decl.Body {
		if r, ok := item.(*ast.Records); ok {
			c.recordsTypeCheck(ti, r)
		}
	}
	c.incrementCheck(ti)
	c.requiredCheck(ti)
}
