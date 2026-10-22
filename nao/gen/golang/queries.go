// CRUD generation (nao_queries.go): the static query surface of Not an
// ORM, every statement rendered by the runtime (docs/spec.md
// §generated-query-surface, §predicates rule 6).
package golang

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/nao/rt"
)

// GenerateQueries renders the queries file for one checked schema file.
func GenerateQueries(f *ast.File, info *check.Info, opts Options) ([]byte, error) {
	return PlanBuild(f, info).Queries(opts)
}

// Queries renders the queries file for the planned package.
func (pl *Plan) Queries(opts Options) ([]byte, error) {
	if opts.Package == "" {
		return nil, fmt.Errorf("no package name")
	}
	if pl.err != nil {
		return nil, pl.err
	}
	e := &queryEmitter{plan: pl.p, opts: opts}
	e.run()
	if e.err != nil {
		return nil, e.err
	}
	return align.Finish(e.out.String()), nil
}

type plan struct {
	tables  []*tableModel
	imports map[string]bool
}

type tableModel struct {
	ti      *check.TableInfo
	model   string
	sqlName string
	fields  []*fieldPlan
	pk      []*fieldPlan

	crud  []CRUDMethod
	sigs  []FieldSig
	names []nameOrigin
}

type fieldPlan struct {
	col      *ast.Column
	colName  string
	wire     string
	goField  string
	tag      string
	goType   string
	baseType string
	storage  string
	imp      string
	arg      string

	nullable   bool
	declared   bool
	kinded     bool
	required   bool
	increment  bool
	hasDefault bool
	unique     bool
}

func (f *fieldPlan) plainBytes() bool { return f.goType == "[]byte" }

func (f *fieldPlan) bindOf(expr string) string {
	if f.plainBytes() {
		return "rt.Bytes(" + expr + ")"
	}
	return expr
}

func (f *fieldPlan) scanTarget() string {
	switch {
	case f.plainBytes():
		return "(*rt.Bytes)(&v." + f.goField + ")"
	case f.kinded && !f.nullable:
		return "(*" + f.storage + ")(&v." + f.goField + ")"
	}
	return "&v." + f.goField
}

func (t *tableModel) field(colName string) *fieldPlan {
	for _, f := range t.fields {
		if f.colName == colName {
			return f
		}
	}
	return nil
}

func (t *tableModel) nonPK() []*fieldPlan {
	var out []*fieldPlan
	for _, f := range t.fields {
		if !t.isPK(f) {
			out = append(out, f)
		}
	}
	return out
}

func (t *tableModel) isPK(f *fieldPlan) bool {
	for _, p := range t.pk {
		if p == f {
			return true
		}
	}
	return false
}

func (t *tableModel) settable() []*fieldPlan {
	var out []*fieldPlan
	for _, f := range t.fields {
		if f.increment {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (t *tableModel) handles(fields []*fieldPlan) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = t.model + f.goField
	}
	return out
}

func (t *tableModel) index(f *fieldPlan) int {
	for i, g := range t.fields {
		if g == f {
			return i
		}
	}
	return -1
}

func (p *plan) fieldCount() int {
	n := 0
	for _, t := range p.tables {
		n += len(t.fields)
	}
	return n
}

func planBuild(f *ast.File, info *check.Info, memo *PlanMemo) (*plan, error) {
	imports := map[string]bool{}
	p := &plan{imports: imports}
	typeNames := map[string]string{"Queries": "the generated Queries type"}
	sqlNames := map[string]string{}

	type built struct {
		tm  *tableModel
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
		if prev, dup := typeNames[b.tm.model]; dup {
			return nil, fmt.Errorf("table %s and %s both map to Go type %s", ti.Decl.Name.Name(), prev, b.tm.model)
		}
		typeNames[b.tm.model] = "table " + ti.Decl.Name.Name()
		if prev, dup := sqlNames[b.tm.sqlName]; dup {
			return nil, fmt.Errorf("tables %s and %s both flatten to SQLite name %q", prev, ti.Decl.Name.Name(), b.tm.sqlName)
		}
		sqlNames[b.tm.sqlName] = ti.Decl.Name.Name()
		for imp := range b.imp {
			imports[imp] = true
		}
		p.tables = append(p.tables, b.tm)
	}
	return p, nil
}

func tableBuild(ti *check.TableInfo) (*tableModel, map[string]bool, error) {
	model, err := modelName(ti.Decl)
	if err != nil {
		return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
	}
	tm := &tableModel{ti: ti, model: model, sqlName: ti.Key}
	imports := map[string]bool{}
	goFields := map[string]string{}
	for _, cd := range ti.Columns {
		fp, err := fieldBuild(imports, cd, goFields)
		if err != nil {
			return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.Name(), err)
		}
		tm.fields = append(tm.fields, fp)
	}

	for _, cd := range ti.PK() {
		if f := tm.field(cd.Col.Name.Name()); f != nil {
			tm.pk = append(tm.pk, f)
		}
	}
	for _, f := range tm.fields {
		f.unique = ti.ColumnsUnique([]string{f.colName})
	}
	tm.crud = crudMethodsBuild(tm)
	tm.sigs = fieldSigsBuild(tm)
	tm.names = modelNamesBuild(tm)
	return tm, imports, nil
}

func fieldBuild(imports map[string]bool, cd *check.ColumnDef, goFields map[string]string) (*fieldPlan, error) {
	col := cd.Col
	colName := col.Name.Name()

	goField, err := goName(colName)
	if err != nil {
		return nil, fmt.Errorf("column %q: %w", colName, err)
	}
	if prev, dup := goFields[goField]; dup {
		return nil, fmt.Errorf("columns %q and %q both map to Go field %s", prev, colName, goField)
	}
	goFields[goField] = colName

	typ := typeOf(cd.Type)
	if typ.imp != "" {
		imports[typ.imp] = true
	}
	goType := typ.name
	if cd.Nullable {
		goType = "rt.Null[" + goType + "]"
	}

	tag := fieldTag(col)
	arg := argName(goField)
	if arg == "rt" {
		arg = "rt_"
	}
	return &fieldPlan{
		col:        col,
		colName:    colName,
		wire:       wireName(colName, tag),
		goField:    goField,
		tag:        tag,
		goType:     goType,
		baseType:   typ.name,
		storage:    cd.Type.Storage,
		imp:        typ.imp,
		arg:        arg,
		nullable:   cd.Nullable,
		declared:   cd.Type.Decl != nil,
		kinded:     cd.Type.Kinded(),
		required:   col.Settings.Get("required") != nil,
		increment:  col.Settings.Get("increment") != nil,
		hasDefault: col.Settings.Get("default") != nil,
	}, nil
}

// wireName is the key a column has in a JSON document: the name its json
// tag gives it, the column name when the tag names none (spec §struct-tags).
func wireName(colName, tag string) string {
	const key = `json:"`
	i := strings.Index(tag, key)
	if i < 0 {
		return colName
	}
	rest := tag[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return colName
	}
	name, _, _ := strings.Cut(rest[:j], ",")
	if name == "" || name == "-" {
		return colName
	}
	return name
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true,
	"for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true,
	"switch": true, "type": true, "var": true,
}

func argName(exported string) string {
	runes := []rune(exported)
	upper := 0
	for upper < len(runes) && runes[upper] >= 'A' && runes[upper] <= 'Z' {
		upper++
	}
	switch {
	case upper == 0:
		if len(runes) > 0 {
			runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
		}
	case upper == len(runes) || upper == 1:
		for i := 0; i < upper; i++ {
			runes[i] = runes[i] - 'A' + 'a'
		}
	default:
		for i := 0; i < upper-1; i++ {
			runes[i] = runes[i] - 'A' + 'a'
		}
	}
	s := string(runes)
	if goKeywords[s] || s == "ctx" || s == "arg" || s == "q" {
		s += "_"
	}
	return s
}

func columnList(fields []*fieldPlan) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = rt.Ident(f.colName)
	}
	return strings.Join(names, ", ")
}

func (t *tableModel) columnList() string { return columnList(t.fields) }

func (t *tableModel) tableSQL() string { return rt.Ident(t.sqlName) }

func (t *tableModel) keyPreds() []rt.GenPred {
	preds := make([]rt.GenPred, len(t.pk))
	for i, f := range t.pk {
		preds[i] = rt.Column[rt.Gen, any]{Name: f.colName}.Eq(f.bindOf(f.arg))
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

func (t *tableModel) getSQL() (string, []string, error) {
	return genRender(rt.SelectRender(t.tableSQL(), t.columnList(), predOpts(t.keyPreds())))
}

func (t *tableModel) getBySQL(f *fieldPlan) (string, []string, error) {
	by := rt.Column[rt.Gen, any]{Name: f.colName}.Eq(f.bindOf(f.arg))
	return genRender(rt.SelectRender(t.tableSQL(), t.columnList(), []rt.Opt[rt.Gen]{by}))
}

func (t *tableModel) listSQL() (string, []string, error) {
	return genRender(rt.SelectRender[rt.Gen](t.tableSQL(), t.columnList(), nil))
}

func (t *tableModel) deleteSQL() (string, []string, error) {
	return genRender(rt.DeleteRender(t.tableSQL(), t.keyPreds()))
}

func (t *tableModel) uniqueFields() []*fieldPlan {
	var out []*fieldPlan
	for _, f := range t.fields {
		if !f.unique {
			continue
		}
		if len(t.pk) == 1 && t.pk[0] == f {
			continue
		}
		out = append(out, f)
	}
	return out
}

type queryEmitter struct {
	plan *plan
	opts Options
	out  strings.Builder
	body strings.Builder
	err  error

	needImports map[string]bool
}

func (e *queryEmitter) fail(t *tableModel, what string, err error) {
	if e.err == nil {
		e.err = fmt.Errorf("table %s: %s: %w", t.ti.Decl.Name.Name(), what, err)
	}
}

func (e *queryEmitter) run() {
	e.needImports = map[string]bool{}
	e.body.Grow(e.plan.fieldCount()*200 + len(e.plan.tables)*2000)
	for _, t := range e.plan.tables {
		e.tableEmit(t)
	}
	e.header()
	e.prologue()
	e.out.Grow(e.body.Len())
	e.out.WriteString(e.body.String())
}

func (e *queryEmitter) header() {
	fmt.Fprintf(&e.out, "// Code generated by nao gen from %s; DO NOT EDIT.\n", e.opts.Source)
	e.out.WriteString("\n")
	fmt.Fprintf(&e.out, "package %s\n\n", e.opts.Package)

	imports := map[string]bool{"context": true, "database/sql": true, rtImport: true}
	for imp := range e.needImports {
		imports[imp] = true
	}
	e.out.WriteString("import (\n")
	for _, p := range importKeysSort(imports) {
		fmt.Fprintf(&e.out, "\t%s\n", importDecl(p))
	}
	e.out.WriteString(")\n\n")
}

func (e *queryEmitter) prologue() {
	e.out.WriteString(`// Queries bundles the generated CRUD over one database handle: a
// *sql.DB, a *sql.Tx, or anything else satisfying rt.DBTX.
type Queries struct {
	db    rt.DBTX
	cache *rt.StmtCache
}

// New returns Queries running on db.
func New(db rt.DBTX) *Queries { return &Queries{db: db} }

// WithCache returns a copy of q whose run-time rendered statements (the
// changeset writes Create and Update, and the dynamic Query, Count,
// Exists, DeleteWhere, UpdateWhere) run through cache: identical shapes
// render identical SQL, prepared once. Bind a cache to a *sql.DB, never
// a transaction.
func (q *Queries) WithCache(cache *rt.StmtCache) *Queries {
	return &Queries{db: q.db, cache: cache}
}

// WithTx returns a copy of q running on tx. The statement cache does not
// follow: its statements are prepared on the outer handle.
func (q *Queries) WithTx(tx *sql.Tx) *Queries { return &Queries{db: tx} }

// Tx runs fn inside a transaction: the Queries handed to fn joins it, a
// nil error commits, anything else rolls back. q must be backed by a
// handle that can begin a transaction (a *sql.DB).
func (q *Queries) Tx(ctx context.Context, fn func(*Queries) error) error {
	return rt.Tx(ctx, q.db, func(tx *sql.Tx) error { return fn(q.WithTx(tx)) })
}

`)
}

func (e *queryEmitter) tableEmit(t *tableModel) {
	if len(t.fields) == 0 {
		return
	}
	b := &e.body
	lower := argName(t.model)
	tbl := t.ti.Decl.Name.Name()

	if len(t.pk) > 0 {
		for _, f := range t.pk {
			e.needImports[f.imp] = true
		}
	}
	for _, f := range t.uniqueFields() {
		e.needImports[f.imp] = true
	}
	delete(e.needImports, "")

	fmt.Fprintf(b, "func %sScan(r rt.Row) (%s, error) {\n\tvar v %s\n\terr := r.Scan(", lower, t.model, t.model)
	for i, f := range t.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.scanTarget())
	}
	fmt.Fprintf(b, ")\n\treturn v, rt.Refused(%q, err)\n}\n\n", t.model)

	if len(t.pk) > 0 {
		e.getEmit(t, lower, tbl)
		if len(t.pk) == 1 {
			e.getManyEmit(t, lower, tbl)
		}
	}
	for _, f := range t.uniqueFields() {
		e.getByEmit(t, f, lower, tbl)
	}
	e.listEmit(t, lower, tbl)
	e.createEmit(t, lower, tbl)
	if len(t.pk) > 0 {
		if len(t.nonPK()) > 0 {
			e.updateEmit(t, lower, tbl)
		}
		e.deleteEmit(t, lower, tbl)
	}
}

func identityParams(t *tableModel) string {
	parts := make([]string, len(t.pk))
	for i, f := range t.pk {
		parts[i] = f.arg + " " + f.goType
	}
	return strings.Join(parts, ", ")
}

func (e *queryEmitter) getEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	sql, args, err := t.getSQL()
	if err != nil {
		e.fail(t, "Get", err)
		return
	}
	fmt.Fprintf(b, "const %sGetSQL = `%s`\n\n", lower, sql)
	fmt.Fprintf(b, "// %sGet fetches the %s row with the given primary key, or rt.ErrNotFound.\n", t.model, tbl)
	fmt.Fprintf(b, "func (q *Queries) %sGet(ctx context.Context, %s) (%s, error) {\n", t.model, identityParams(t), t.model)
	fmt.Fprintf(b, "\treturn %sScan(q.db.QueryRowContext(ctx, %sGetSQL, %s))\n}\n\n", lower, lower, strings.Join(args, ", "))
}

func (e *queryEmitter) getManyEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	key := t.pk[0]
	fmt.Fprintf(b, "// %sGetMany fetches the %s rows whose primary key is in %ss, in\n", t.model, tbl, key.arg)
	fmt.Fprintf(b, "// database order. Missing keys are not an error; they are just absent.\n")
	fmt.Fprintf(b, "func (q *Queries) %sGetMany(ctx context.Context, %ss []%s) ([]%s, error) {\n", t.model, key.arg, key.goType, t.model)
	fmt.Fprintf(b, "\tif len(%ss) == 0 {\n\t\treturn nil, nil\n\t}\n", key.arg)
	fmt.Fprintf(b, "\tquery, args, err := rt.SelectRender(%sTableSQL, %sColumnsSQL, []rt.Opt[%s]{%s%s.In(%ss...)})\n", lower, lower, t.model, t.model, key.goField, key.arg)
	b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")
	b.WriteString("\trows, err := q.db.QueryContext(ctx, query, args...)\n")
	e.rowsScan(t, lower)
}

func (e *queryEmitter) getByEmit(t *tableModel, f *fieldPlan, lower, tbl string) {
	b := &e.body
	sql, args, err := t.getBySQL(f)
	if err != nil {
		e.fail(t, "GetBy"+f.goField, err)
		return
	}
	fmt.Fprintf(b, "const %sGetBy%sSQL = `%s`\n\n", lower, f.goField, sql)
	fmt.Fprintf(b, "// %sGetBy%s fetches the %s row with the given %s (a unique column),\n// or rt.ErrNotFound.\n", t.model, f.goField, tbl, f.colName)
	fmt.Fprintf(b, "func (q *Queries) %sGetBy%s(ctx context.Context, %s %s) (%s, error) {\n", t.model, f.goField, f.arg, f.goType, t.model)
	fmt.Fprintf(b, "\treturn %sScan(q.db.QueryRowContext(ctx, %sGetBy%sSQL, %s))\n}\n\n", lower, lower, f.goField, strings.Join(args, ", "))
}

func (e *queryEmitter) listEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	sql, _, err := t.listSQL()
	if err != nil {
		e.fail(t, "List", err)
		return
	}
	fmt.Fprintf(b, "const %sListSQL = `%s`\n\n", lower, sql)
	fmt.Fprintf(b, "// %sList returns every %s row.\n", t.model, tbl)
	fmt.Fprintf(b, "func (q *Queries) %sList(ctx context.Context) ([]%s, error) {\n", t.model, t.model)
	fmt.Fprintf(b, "\trows, err := q.db.QueryContext(ctx, %sListSQL)\n", lower)
	e.rowsScan(t, lower)
}

func (e *queryEmitter) rowsScan(t *tableModel, lower string) {
	rowsScanEmit(&e.body, t.model, lower)
}

func rowsScanEmit(b *strings.Builder, model, lower string) {
	fmt.Fprintf(b, "\tif err != nil {\n\t\treturn nil, err\n\t}\n\tdefer rows.Close()\n")
	fmt.Fprintf(b, "\tout := []%s{}\n\tfor rows.Next() {\n", model)
	fmt.Fprintf(b, "\t\tv, err := %sScan(rows)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tout = append(out, v)\n\t}\n", lower)
	fmt.Fprintf(b, "\treturn out, rows.Err()\n}\n\n")
}

func (e *queryEmitter) createEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "// %sCreate validates c and inserts its marked columns, returning the\n", t.model)
	fmt.Fprintf(b, "// stored %s row; nothing marked inserts the defaults (spec §changesets).\n", tbl)
	fmt.Fprintf(b, "func (q *Queries) %sCreate(ctx context.Context, c %sChangeset) (%s, error) {\n", t.model, t.model, t.model)
	fmt.Fprintf(b, "\tc = c.Validate()\n\tif err := c.Err(); err != nil {\n\t\treturn %s{}, err\n\t}\n", t.model)
	markedArgsEmit(b, t)
	fmt.Fprintf(b, "\trow, err := rt.StmtQueryRow(ctx, q.db, q.cache, rt.InsertRender(%sTableSQL, cols, %sColumnsSQL), args...)\n", lower, lower)
	fmt.Fprintf(b, "\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\treturn %sScan(row)\n}\n\n", t.model, lower)
}

func (e *queryEmitter) updateEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "// %sUpdate validates c and writes its marked columns to the %s row it\n", t.model, tbl)
	fmt.Fprintf(b, "// was made from, returning the stored row; nothing marked returns the row\n")
	fmt.Fprintf(b, "// untouched (spec §changesets).\n")
	fmt.Fprintf(b, "func (q *Queries) %sUpdate(ctx context.Context, c %sChangeset) (%s, error) {\n", t.model, t.model, t.model)
	fmt.Fprintf(b, "\tif !c.row {\n\t\treturn %s{}, rt.ErrNoRow\n\t}\n", t.model)
	fmt.Fprintf(b, "\tc = c.Validate()\n\tif err := c.Err(); err != nil {\n\t\treturn %s{}, err\n\t}\n", t.model)
	markedArgsEmit(b, t)
	b.WriteString("\tif len(cols) == 0 {\n\t\treturn v, nil\n\t}\n")
	keys := make([]string, len(t.pk))
	for i, f := range t.pk {
		fmt.Fprintf(b, "\targs = append(args, %s)\n", f.bindOf("c.data."+f.goField))
		keys[i] = f.colName
	}
	fmt.Fprintf(b, "\trow, err := rt.StmtQueryRow(ctx, q.db, q.cache, rt.UpdateRowRender(%sTableSQL, cols, %#v, %sColumnsSQL), args...)\n", lower, keys, lower)
	fmt.Fprintf(b, "\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\treturn %sScan(row)\n}\n\n", t.model, lower)
}

func markedArgsEmit(b *strings.Builder, t *tableModel) {
	settable := t.settable()
	if len(settable) > 0 {
		b.WriteString("\tv := c.Row()\n")
	}
	b.WriteString("\tvar cols []string\n\tvar args []any\n")
	for _, f := range settable {
		fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tcols, args = append(cols, %q), append(args, %s)\n\t}\n", t.index(f), f.colName, f.bindOf("v."+f.goField))
	}
}

func (e *queryEmitter) deleteEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	sql, args, err := t.deleteSQL()
	if err != nil {
		e.fail(t, "Delete", err)
		return
	}
	fmt.Fprintf(b, "const %sDeleteSQL = `%s`\n\n", lower, sql)
	fmt.Fprintf(b, "// %sDelete removes the identified %s row; rt.ErrNotFound reports\n// that nothing matched.\n", t.model, tbl)
	fmt.Fprintf(b, "func (q *Queries) %sDelete(ctx context.Context, %s) error {\n", t.model, identityParams(t))
	fmt.Fprintf(b, "\tres, err := q.db.ExecContext(ctx, %sDeleteSQL, %s)\n", lower, strings.Join(args, ", "))
	fmt.Fprintf(b, "\tif err != nil {\n\t\treturn rt.Refused(%q, err)\n\t}\n", t.model)
	fmt.Fprintf(b, "\tn, err := res.RowsAffected()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(b, "\tif n == 0 {\n\t\treturn rt.ErrNotFound\n\t}\n\treturn nil\n}\n\n")
}
