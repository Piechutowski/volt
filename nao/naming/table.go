package naming

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/nao/rt"
)

type plan struct {
	tables  []*Table
	imports map[string]bool
}

// Table is one table's plan (spec §mapping-to-go): the checked table, its
// model name and SQLite name, every column's plan in declaration order and
// the primary key's in key order. A Table is immutable once planned.
type Table struct {
	Info    *check.TableInfo
	Model   string
	SQLName string
	Fields  []*Field
	PK      []*Field

	crud  []CRUDMethod
	sigs  []FieldSig
	names []nameOrigin
}

// Field is one column's plan (spec §mapping-to-go, §struct-tags): its Go
// field, struct tag, wire key, Go type with and without rt.Null, storage
// type, import key, the parameter a key method takes for it, and the
// settings generation reads. A Field is immutable once planned.
type Field struct {
	Col      *ast.Column
	ColName  string
	Wire     string
	GoField  string
	Tag      string
	GoType   string
	BaseType string
	Storage  string
	Imp      string
	Arg      string

	Nullable  bool
	Declared  bool
	Kinded    bool
	Required  bool
	Increment bool
	Unique    bool
}

// PlainBytes reports whether the field is a non-nullable []byte column
// (spec §columns).
func (f *Field) PlainBytes() bool { return f.GoType == "[]byte" }

// BindOf is the Go expression a statement binds for the field's value
// expr: through rt.Bytes for plain bytes, so nil binds empty bytes (spec
// §columns).
func (f *Field) BindOf(expr string) string {
	if f.PlainBytes() {
		return "rt.Bytes(" + expr + ")"
	}
	return expr
}

// Field is the plan of the named column, nil when the table has none
// (spec §mapping-to-go).
func (t *Table) Field(colName string) *Field {
	for _, f := range t.Fields {
		if f.ColName == colName {
			return f
		}
	}
	return nil
}

// NonPK lists the fields outside the primary key, in declaration order,
// the columns an Update can write (spec §generated-query-surface).
func (t *Table) NonPK() []*Field {
	var out []*Field
	for _, f := range t.Fields {
		if !t.isPK(f) {
			out = append(out, f)
		}
	}
	return out
}

func (t *Table) isPK(f *Field) bool {
	for _, p := range t.PK {
		if p == f {
			return true
		}
	}
	return false
}

// Settable lists the fields a changeset sets: every one but the
// auto-increment key (spec §changesets).
func (t *Table) Settable() []*Field {
	var out []*Field
	for _, f := range t.Fields {
		if f.Increment {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (t *Table) handles(fields []*Field) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = t.Model + f.GoField
	}
	return out
}

// Index is the position of f among the table's fields, -1 when it is not
// one of them; a changeset marks a column at that position (spec
// §changesets).
func (t *Table) Index(f *Field) int {
	for i, g := range t.Fields {
		if g == f {
			return i
		}
	}
	return -1
}

// UniqueFields lists the single-column unique fields a GetBy method reads
// by, the sole primary key left out (spec §generated-query-surface).
func (t *Table) UniqueFields() []*Field {
	var out []*Field
	for _, f := range t.Fields {
		if !f.Unique {
			continue
		}
		if len(t.PK) == 1 && t.PK[0] == f {
			continue
		}
		out = append(out, f)
	}
	return out
}

func planBuild(f *ast.File, info *check.Info, memo *PlanMemo) (*plan, error) {
	imports := map[string]bool{}
	p := &plan{imports: imports}
	typeNames := map[string]string{"Queries": "the generated Queries type"}
	sqlNames := map[string]string{}

	type built struct {
		tm  *Table
		imp map[string]bool
		err error
	}
	tables := make([]built, len(info.Tables))
	par.For(len(info.Tables), func(i int) {
		if memo != nil {
			if e := memo.prev[info.Tables[i]]; e != nil {
				tables[i] = built{e.tm, e.imports, nil}
				return
			}
		}
		tm, imports, err := tableBuild(info.Tables[i])
		tables[i] = built{tm, imports, err}
	})
	if memo != nil {
		memo.next = make(map[*check.TableInfo]*memoModel, len(info.Tables))
		for i, ti := range info.Tables {
			if b := tables[i]; b.err == nil {
				if e := memo.prev[ti]; e != nil && e.tm == b.tm {
					memo.Hits++
					memo.next[ti] = e
				} else {
					memo.Misses++
					memo.next[ti] = &memoModel{tm: b.tm, imports: b.imp}
				}
			}
		}
	}
	for i, ti := range info.Tables {
		b := tables[i]
		if b.err != nil {
			return nil, b.err
		}
		if prev, dup := typeNames[b.tm.Model]; dup {
			return nil, fmt.Errorf("table %s and %s both map to Go type %s", ti.Decl.Name.Name(), prev, b.tm.Model)
		}
		typeNames[b.tm.Model] = "table " + ti.Decl.Name.Name()
		if prev, dup := sqlNames[b.tm.SQLName]; dup {
			return nil, fmt.Errorf("tables %s and %s both flatten to SQLite name %q", prev, ti.Decl.Name.Name(), b.tm.SQLName)
		}
		sqlNames[b.tm.SQLName] = ti.Decl.Name.Name()
		for imp := range b.imp {
			imports[imp] = true
		}
		p.tables = append(p.tables, b.tm)
	}
	return p, nil
}

func tableBuild(ti *check.TableInfo) (*Table, map[string]bool, error) {
	model, err := ModelName(ti.Decl)
	if err != nil {
		return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
	}
	tm := &Table{Info: ti, Model: model, SQLName: ti.Key}
	imports := map[string]bool{}
	goFields := map[string]string{}
	for _, cd := range ti.Columns {
		fp, err := fieldBuild(imports, cd, goFields)
		if err != nil {
			return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
		}
		tm.Fields = append(tm.Fields, fp)
	}

	for _, cd := range ti.PK() {
		if f := tm.Field(cd.Col.Name.Name()); f != nil {
			tm.PK = append(tm.PK, f)
		}
	}
	for _, f := range tm.Fields {
		f.Unique = ti.ColumnsUnique([]string{f.ColName})
	}
	tm.crud = crudMethodsBuild(tm)
	tm.sigs = fieldSigsBuild(tm)
	tm.names = modelNamesBuild(tm)
	return tm, imports, nil
}

func fieldBuild(imports map[string]bool, cd *check.ColumnDef, goFields map[string]string) (*Field, error) {
	col := cd.Col
	colName := col.Name.Name()

	goField, err := GoName(colName)
	if err != nil {
		return nil, fmt.Errorf("column %q: %w", colName, err)
	}
	if prev, dup := goFields[goField]; dup {
		return nil, fmt.Errorf("columns %q and %q both map to Go field %s", prev, colName, goField)
	}
	goFields[goField] = colName

	imp := ImportKey(cd.Type.Alias, cd.Type.Import)
	if imp != "" {
		imports[imp] = true
	}
	goType := cd.Type.Go
	if cd.Nullable {
		goType = "rt.Null[" + goType + "]"
	}

	tag := fieldTag(col)
	return &Field{
		Col:       col,
		ColName:   colName,
		Wire:      wireName(colName, tag),
		GoField:   goField,
		Tag:       tag,
		GoType:    goType,
		BaseType:  cd.Type.Go,
		Storage:   cd.Type.Storage,
		Imp:       imp,
		Arg:       paramName(goField),
		Nullable:  cd.Nullable,
		Declared:  cd.Type.Decl != nil,
		Kinded:    cd.Type.Kinded(),
		Required:  col.Settings.Get("required") != nil,
		Increment: col.Settings.Get("increment") != nil,
	}, nil
}

func columnList(fields []*Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = rt.Ident(f.ColName)
	}
	return strings.Join(names, ", ")
}

// ColumnList is the table's quoted column list in declaration order, what
// every generated statement selects and returns (spec
// §generated-query-surface).
func (t *Table) ColumnList() string { return columnList(t.Fields) }

// TableSQL is the table's quoted SQLite name (spec §mapping-to-sqlite-ddl).
func (t *Table) TableSQL() string { return rt.Ident(t.SQLName) }

func (t *Table) keyPreds() []rt.GenPred {
	preds := make([]rt.GenPred, len(t.PK))
	for i, f := range t.PK {
		preds[i] = rt.Column[rt.Gen, any]{Name: f.ColName}.Eq(f.BindOf(f.Arg))
	}
	return preds
}

func predOpts(preds []rt.GenPred) []rt.Opt[rt.Gen] {
	opts := make([]rt.Opt[rt.Gen], len(preds))
	for i, p := range preds {
		opts[i] = p
	}
	return opts
}

func genRender(sql string, args []any, err error) (string, []string, error) {
	if err != nil {
		return "", nil, err
	}
	exprs := make([]string, len(args))
	for i, a := range args {
		expr, ok := a.(string)
		if !ok {
			return "", nil, fmt.Errorf("value slot %d holds %T, not Go source", i, a)
		}
		exprs[i] = expr
	}
	return sql, exprs, nil
}

// GetSQL renders the statement Get runs through the runtime (spec
// §predicates), with the Go expressions it binds in placeholder order.
func (t *Table) GetSQL() (string, []string, error) {
	return genRender(rt.SelectRender(t.TableSQL(), t.ColumnList(), predOpts(t.keyPreds())))
}

// GetBySQL renders the statement GetBy reads f by, with its binds (spec
// §generated-query-surface).
func (t *Table) GetBySQL(f *Field) (string, []string, error) {
	by := rt.Column[rt.Gen, any]{Name: f.ColName}.Eq(f.BindOf(f.Arg))
	return genRender(rt.SelectRender(t.TableSQL(), t.ColumnList(), []rt.Opt[rt.Gen]{by}))
}

// ListSQL renders the statement List runs (spec §generated-query-surface).
func (t *Table) ListSQL() (string, []string, error) {
	return genRender(rt.SelectRender[rt.Gen](t.TableSQL(), t.ColumnList(), nil))
}

// DeleteSQL renders the statement Delete runs, with its binds (spec
// §generated-query-surface).
func (t *Table) DeleteSQL() (string, []string, error) {
	return genRender(rt.DeleteRender(t.TableSQL(), t.keyPreds()))
}

// SelectColumns lists the fields one select instantiation reads, in its
// projection's order (spec §selects-over-groups).
func (t *Table) SelectColumns(fn SelectFn) ([]*Field, error) {
	switch {
	case len(fn.Cols) > 0:
		out := make([]*Field, 0, len(fn.Cols))
		for _, c := range fn.Cols {
			f := t.Field(c)
			if f == nil {
				return nil, fmt.Errorf("select %s: no column %q in table %s", fn.MethodSuffix, c, t.SQLName)
			}
			out = append(out, f)
		}
		return out, nil
	case len(fn.Excluded) > 0:
		ex := make(map[string]bool, len(fn.Excluded))
		for _, c := range fn.Excluded {
			if t.Field(c) == nil {
				return nil, fmt.Errorf("select %s: no column %q in table %s", fn.MethodSuffix, c, t.SQLName)
			}
			ex[c] = true
		}
		var out []*Field
		for _, f := range t.Fields {
			if !ex[f.ColName] {
				out = append(out, f)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("select %s: every column of %s excluded", fn.MethodSuffix, t.SQLName)
		}
		return out, nil
	}
	return t.Fields, nil
}

// SelectSQL renders the statement of one instantiation through the
// runtime (spec §predicates) and lists the Go expressions its method
// binds, in placeholder order.
func (t *Table) SelectSQL(fn SelectFn) (string, []string, error) {
	cols, err := t.SelectColumns(fn)
	if err != nil {
		return "", nil, err
	}
	opts := []rt.Opt[rt.Gen]{fn.Where}
	if len(fn.Order) > 0 {
		opts = append(opts, rt.OrderBy(fn.Order...))
	}
	return genRender(rt.SelectRender(rt.Ident(t.SQLName), columnList(cols), opts))
}
