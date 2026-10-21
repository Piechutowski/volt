// Package check implements semantic analysis of one parsed schema file — the
// role go/types plays for Go. It validates every constraint of the spec
// that goes beyond grammar (setting whitelists, duplicate names, reference
// resolution, records arity) and builds an Info symbol table.
//
// Diagnostic codes cite the spec section they enforce, e.g. "spec/relationships-ref".
package check

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// Info is the semantic model of one file, produced by File.
type Info struct {
	Tables   []*TableInfo
	Enums    []*EnumInfo
	Types    []*TypeInfo
	Partials []*PartialInfo
	Refs     []*RefInfo
	// Migrations in name order, the order they apply in (spec §migrations).
	Migrations []*MigrationInfo

	byName map[string]*TableInfo
}

// Table is the table of that name, or nil (spec §namespaces-and-uniqueness).
func (i *Info) Table(name string) *TableInfo { return i.byName[name] }

// TableInfo describes one table with partial injection applied.
type TableInfo struct {
	Decl *ast.Table
	Key  string // the table's name, its key in every table map

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
	Col      *ast.Column
	Partial  *ast.TablePartial // nil for direct definitions
	Type     ColumnType
	Nullable bool
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

// RefInfo is one resolved (or unresolved) relationship, from a Ref
// element or an inline ref (spec §relationships-ref). FK is the
// foreign-key side, the left endpoint of a Ref element or the declaring
// column of an inline ref, and Target the columns it references: the
// one place the foreign-key side is decided.
type RefInfo struct {
	Node   ast.Node // *ast.Ref, or the *ast.Setting of an inline ref
	Inline bool
	FK     EndpointInfo
	Target EndpointInfo
}

type EndpointInfo struct {
	Table   *TableInfo // nil if unresolved
	Columns []string
	Pos     token.Position
}

// File checks one parsed file and returns its semantic model together with
// all diagnostics.
func File(f *ast.File) (*Info, []diag.Diagnostic) {
	return FileMemo(f, nil)
}

// FileMemo is File with a memo of the file's earlier checks: a table
// whose declaration and injected partials are the nodes they were is
// answered from it. A nil memo checks everything.
func FileMemo(f *ast.File, memo *Memo) (*Info, []diag.Diagnostic) {
	c := &checker{
		info: &Info{byName: map[string]*TableInfo{}},
		memo: memo,
	}
	if memo != nil {
		memo.Hits, memo.Misses = 0, 0
	}
	c.collect(f)
	c.migrationsCheck(f)
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
			key := d.Name.Name()
			if _, dup := c.info.byName[key]; dup {
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate table name %q", key)
				continue
			}
			ti := &TableInfo{Decl: d, Key: key}
			c.info.byName[key] = ti
			c.info.Tables = append(c.info.Tables, ti)
		case *ast.TypeDecl:
			typeDecls = append(typeDecls, d)
		case *ast.Enum:
			key := d.Name.Name()
			if _, dup := c.enums[key]; dup {
				c.errorf(d.Pos(), "namespaces-and-uniqueness", "duplicate enum name %q", key)
				continue
			}
			ei := &EnumInfo{Decl: d, Key: key}
			c.enums[key] = ei
			c.info.Enums = append(c.info.Enums, ei)
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

	failed := map[string]bool{}
	for _, d := range typeDecls {
		if ti := c.typeDeclCheck(d); ti != nil {
			c.types[ti.Key] = ti
			c.info.Types = append(c.info.Types, ti)
		} else {
			failed[d.Name.String()] = true
		}
	}
	c.env = typeEnvOf(c.enums, c.types, failed)
	reported := map[diag.Diagnostic]bool{}
	for i, ti := range c.info.Tables {
		partials := c.partialsOf(ti.Decl)
		for _, p := range partials {
			if p != nil {
				c.partials[p.Name.Name()].Uses++
			}
		}
		var checked *TableInfo
		var diags []diag.Diagnostic
		if e := c.memo.lookup(ti.Decl, partials, c.env.keys); e != nil {
			checked, diags = e.ti, e.diags
			c.memo.store(e)
			c.memo.Hits++
		} else {
			checked, diags = tableCheck(ti.Decl, ti.Key, partials, c.env)
			if c.memo != nil {
				c.memo.store(&memoTable{ti: checked, partials: partials, enums: c.env.keys, diags: diags})
				c.memo.Misses++
			}
		}
		if checked != ti {
			c.info.Tables[i] = checked
			c.info.byName[checked.Key] = checked
		}
		c.diags = append(c.diags, partialColumnDedupe(checked, diags, reported)...)
	}
}

func partialColumnDedupe(ti *TableInfo, diags []diag.Diagnostic, reported map[diag.Diagnostic]bool) []diag.Diagnostic {
	var spans []*ast.Column
	for _, cd := range ti.Columns {
		if cd.Partial != nil {
			spans = append(spans, cd.Col)
		}
	}
	if len(spans) == 0 {
		return diags
	}
	within := func(p token.Position) bool {
		for _, col := range spans {
			if p.File() == col.Pos().File() && col.Pos().Offset() <= p.Offset() && p.Offset() <= col.End().Offset() {
				return true
			}
		}
		return false
	}
	out := make([]diag.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if within(d.Pos) {
			if reported[d] {
				continue
			}
			reported[d] = true
		}
		out = append(out, d)
	}
	return out
}

func tableCheck(d *ast.Table, key string, partials []*ast.TablePartial, env *typeEnv) (*TableInfo, []diag.Diagnostic) {
	acc := &checker{info: &Info{}, env: env}
	ti := &TableInfo{Decl: d, Key: key}
	acc.columnsExpand(ti, partials)
	acc.tableBodyCheck(d.Pos(), key, d.Settings, d.Body, true)
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
	var injections []indexInjection
	for _, item := range ti.Decl.Body {
		switch item := item.(type) {
		case *ast.Column:
			addCol(item, nil)
		case *ast.PartialRef:
			p := partials[injected]
			injected++
			if p == nil {
				c.errorf(item.Pos(), "tablepartial", "unknown TablePartial %q", item.Name.Name())
				continue
			}
			for _, pit := range p.Body {
				switch pit := pit.(type) {
				case *ast.Column:
					addCol(pit, p)
				case *ast.IndexesBlock:
					for _, ix := range pit.Indexes {
						injections = append(injections, indexInjection{ix, item, p})
					}
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
	c.partialIndexCheck(ti, injections)
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
	c.pkCountCheck(ti.Key, ti.Columns, ti.Indexes)
	for _, item := range ti.Decl.Body {
		if r, ok := item.(*ast.Records); ok {
			c.recordsTypeCheck(ti, r)
			c.recordsCoverageCheck(ti, r)
		}
	}
	c.incrementCheck(ti)
	c.requiredCheck(ti)
}

type indexInjection struct {
	ix      *ast.Index
	at      *ast.PartialRef
	partial *ast.TablePartial
}

// partialIndexCheck enforces that an index injected by a partial has a
// key no other index of the table has, declared in the body or injected
// by another partial: indexes are not overridden (spec §tablepartial).
func (c *checker) partialIndexCheck(ti *TableInfo, injections []indexInjection) {
	injectedAt := map[*ast.Index]int{}
	for i, inj := range injections {
		injectedAt[inj.ix] = i
	}
	for i, inj := range injections {
		key := indexAtoms(inj.ix)
		for _, other := range ti.Indexes {
			if other == inj.ix || indexAtoms(other) != key {
				continue
			}
			if j, injected := injectedAt[other]; injected && j > i {
				continue
			}
			c.errorf(inj.at.Pos(), "tablepartial", "TablePartial %q injects index (%s) into table %q, which has an index on that key already; an index is declared once, in the table or in one partial (§tablepartial)", inj.partial.Name.Name(), key, ti.Key)
			break
		}
	}
}

func indexAtoms(ix *ast.Index) string {
	parts := make([]string, 0, len(ix.Key))
	for _, atom := range ix.Key {
		switch a := atom.(type) {
		case *ast.Ident:
			parts = append(parts, a.Name())
		case *ast.FuncExpr:
			parts = append(parts, "`"+a.Text()+"`")
		}
	}
	return strings.Join(parts, ", ")
}
