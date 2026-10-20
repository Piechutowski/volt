// CRUD generation (nao_queries.go): the v0 query surface of Not an ORM
// .
package golang

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
)

// GenerateQueries renders the queries file for one checked DBML file.
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
	enum     *check.EnumInfo
	imp      string
	param    string
	arg      string

	nullable   bool
	declared   bool
	required   bool
	increment  bool
	hasDefault bool
	unique     bool
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
	g := &generator{f: f, info: info, imports: map[string]bool{}}
	if err := g.enumTypesCollect(); err != nil {
		return nil, err
	}

	p := &plan{imports: g.imports}
	typeNames := map[string]string{"Queries": "the generated Queries type"}
	for _, e := range info.Enums {
		typeNames[g.enumTypes[e.Key]] = "enum " + e.Decl.Name.String()
	}
	sqlNames := map[string]string{}

	type built struct {
		tm  *tableModel
		imp map[string]bool
		err error
	}
	enums := enumTypesOf(info, g.enumTypes)
	tables := make([]built, len(info.Tables))
	par.For(len(info.Tables), func(i int) {
		if memo != nil {
			if e := memo.prev[info.Tables[i]]; e != nil && slices.Equal(e.enums, enums.pairs) {
				tables[i] = built{e.tm, e.imports, nil}
				return
			}
		}
		tm, imports, err := tableBuild(info.Tables[i], enums)
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
					memo.next[ti] = &memoModel{tm: b.tm, imports: b.imp, enums: enums.pairs}
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
			return nil, fmt.Errorf("table %s and %s both map to Go type %s", ti.Decl.Name.String(), prev, b.tm.model)
		}
		typeNames[b.tm.model] = "table " + ti.Decl.Name.String()
		if prev, dup := sqlNames[b.tm.sqlName]; dup {
			return nil, fmt.Errorf("tables %s and %s both flatten to SQLite name %q", prev, ti.Decl.Name.String(), b.tm.sqlName)
		}
		sqlNames[b.tm.sqlName] = ti.Decl.Name.String()
		for imp := range b.imp {
			g.imports[imp] = true
		}
		p.tables = append(p.tables, b.tm)
	}
	return p, nil
}

type enumTypes struct {
	byKey map[string]string
	pairs []enumType
}

type enumType struct{ key, typ string }

func enumTypesOf(info *check.Info, byKey map[string]string) *enumTypes {
	pairs := make([]enumType, 0, len(info.Enums))
	for _, e := range info.Enums {
		pairs = append(pairs, enumType{e.Key, byKey[e.Key]})
	}
	slices.SortFunc(pairs, func(a, b enumType) int { return strings.Compare(a.key, b.key) })
	return &enumTypes{byKey: byKey, pairs: pairs}
}

func tableBuild(ti *check.TableInfo, enums *enumTypes) (*tableModel, map[string]bool, error) {
	model, err := modelName(ti.Decl)
	if err != nil {
		return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.String(), err)
	}
	tm := &tableModel{ti: ti, model: model, sqlName: sqlTableName(ti.Decl.Name)}
	imports := map[string]bool{}
	pkFromIndex := compositePKColumns(ti)
	goFields := map[string]string{}
	params := map[string]string{}
	for _, cd := range ti.Columns {
		fp, err := fieldBuild(enums.byKey, imports, cd, pkFromIndex, goFields, params)
		if err != nil {
			return nil, nil, fmt.Errorf("table %s: %w", ti.Decl.Name.String(), err)
		}
		tm.fields = append(tm.fields, fp)
	}

	for _, f := range tm.fields {
		if hasSetting(f.col, "pk") || hasSetting(f.col, "primary key") || hasLegacyFlag(f.col, "pk") {
			tm.pk = append(tm.pk, f)
		}
	}
	if len(tm.pk) == 0 {
		for _, ix := range ti.Indexes {
			if ix.Settings.Get("pk") == nil {
				continue
			}
			for _, atom := range ix.Key {
				if id, ok := atom.(*ast.Ident); ok {
					if f := tm.field(id.Name()); f != nil {
						tm.pk = append(tm.pk, f)
					}
				}
			}
		}
	}

	for _, ix := range ti.Indexes {
		if ix.Settings.Get("unique") == nil || len(ix.Key) != 1 {
			continue
		}
		if id, ok := ix.Key[0].(*ast.Ident); ok {
			if f := tm.field(id.Name()); f != nil {
				f.unique = true
			}
		}
	}
	tm.crud = crudMethodsBuild(tm)
	tm.sigs = fieldSigsBuild(tm)
	tm.names = modelNamesBuild(tm)
	return tm, imports, nil
}

func fieldBuild(enumTypes map[string]string, imports map[string]bool, cd *check.ColumnDef, pkFromIndex map[string]bool, goFields, params map[string]string) (*fieldPlan, error) {
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

	typ := typeOf(cd.Type, enumTypes)
	if typ.imp != "" {
		imports[typ.imp] = true
	}
	nullable := isNullable(col) && !pkFromIndex[colName]
	goType := typ.name
	if nullable && !typ.nilable {
		goType = "rt.Null[" + goType + "]"
	}

	param := sqlParamName(colName)
	if prev, dup := params[param]; dup {
		return nil, fmt.Errorf("columns %q and %q both map to SQL parameter name %q", prev, colName, param)
	}
	params[param] = colName

	tag := fieldTag(col)
	return &fieldPlan{
		col:        col,
		colName:    colName,
		wire:       wireName(colName, tag),
		goField:    goField,
		tag:        tag,
		goType:     goType,
		baseType:   typ.name,
		storage:    cd.Type.Storage,
		enum:       cd.Type.Enum,
		imp:        typ.imp,
		param:      param,
		arg:        argName(goField),
		nullable:   nullable,
		declared:   cd.Type.Decl != nil,
		required:   hasSetting(col, "required"),
		increment:  hasSetting(col, "increment"),
		hasDefault: col.Settings.Get("default") != nil,
		unique:     hasSetting(col, "unique"),
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

func sqlTableName(q *ast.QualName) string {
	if s := q.Schema(); s != "" && s != "public" {
		return s + "_" + q.Base()
	}
	return q.Base()
}

func sqlParamName(colName string) string {
	var b strings.Builder
	for _, r := range colName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "p" + s
	}
	return s
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

func sqlIdentQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func hasSetting(col *ast.Column, name string) bool {
	return col.Settings.Get(name) != nil
}

func hasLegacyFlag(col *ast.Column, name string) bool {
	for _, f := range col.LegacyFlags {
		if strings.EqualFold(f.Name(), name) {
			return true
		}
	}
	return false
}

func (t *tableModel) columnList() string {
	names := make([]string, len(t.fields))
	for i, f := range t.fields {
		names[i] = sqlIdentQuote(f.colName)
	}
	return strings.Join(names, ", ")
}

func (t *tableModel) pkWhere() string {
	conds := make([]string, len(t.pk))
	for i, f := range t.pk {
		conds[i] = sqlIdentQuote(f.colName) + " = :" + f.param
	}
	return strings.Join(conds, " AND ")
}

func (t *tableModel) getSQL() string {
	return "SELECT " + t.columnList() + " FROM " + sqlIdentQuote(t.sqlName) + " WHERE " + t.pkWhere()
}

func (t *tableModel) getManySQL() string {
	return "SELECT " + t.columnList() + " FROM " + sqlIdentQuote(t.sqlName) + " WHERE " + sqlIdentQuote(t.pk[0].colName) + " IN"
}

func (t *tableModel) getBySQL(f *fieldPlan) string {
	return "SELECT " + t.columnList() + " FROM " + sqlIdentQuote(t.sqlName) + " WHERE " + sqlIdentQuote(f.colName) + " = :" + f.param
}

func (t *tableModel) listSQL() string {
	return "SELECT " + t.columnList() + " FROM " + sqlIdentQuote(t.sqlName)
}

func (t *tableModel) deleteSQL() string {
	return "DELETE FROM " + sqlIdentQuote(t.sqlName) + " WHERE " + t.pkWhere()
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

	needStrings bool
	needImports map[string]bool
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
	if e.needStrings {
		imports["strings"] = true
	}
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
	tbl := t.ti.Decl.Name.String()

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
		b.WriteString("&v." + f.goField)
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

func identityArgs(t *tableModel) string {
	parts := make([]string, len(t.pk))
	for i, f := range t.pk {
		parts[i] = fmt.Sprintf("sql.Named(%q, %s)", f.param, f.arg)
	}
	return strings.Join(parts, ", ")
}

func (e *queryEmitter) getEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "const %sGetSQL = `%s`\n\n", lower, t.getSQL())
	fmt.Fprintf(b, "// %sGet fetches the %s row with the given primary key, or rt.ErrNotFound.\n", t.model, tbl)
	fmt.Fprintf(b, "func (q *Queries) %sGet(ctx context.Context, %s) (%s, error) {\n", t.model, identityParams(t), t.model)
	fmt.Fprintf(b, "\treturn %sScan(q.db.QueryRowContext(ctx, %sGetSQL, %s))\n}\n\n", lower, lower, identityArgs(t))
}

func (e *queryEmitter) getManyEmit(t *tableModel, lower, tbl string) {
	e.needStrings = true
	b := &e.body
	key := t.pk[0]
	fmt.Fprintf(b, "const %sGetManySQL = `%s`\n\n", lower, t.getManySQL())
	fmt.Fprintf(b, "// %sGetMany fetches the %s rows whose primary key is in %ss, in\n", t.model, tbl, key.arg)
	fmt.Fprintf(b, "// database order. Missing keys are not an error; they are just absent.\n")
	fmt.Fprintf(b, "func (q *Queries) %sGetMany(ctx context.Context, %ss []%s) ([]%s, error) {\n", t.model, key.arg, key.goType, t.model)
	fmt.Fprintf(b, "\tif len(%ss) == 0 {\n\t\treturn nil, nil\n\t}\n", key.arg)
	fmt.Fprintf(b, "\targs := make([]any, len(%ss))\n", key.arg)
	fmt.Fprintf(b, "\tfor i, k := range %ss {\n\t\targs[i] = k\n\t}\n", key.arg)
	fmt.Fprintf(b, "\tquery := %sGetManySQL + \" (?\" + strings.Repeat(\", ?\", len(%ss)-1) + \")\"\n", lower, key.arg)
	fmt.Fprintf(b, "\trows, err := q.db.QueryContext(ctx, query, args...)\n")
	e.rowsScan(t, lower)
}

func (e *queryEmitter) getByEmit(t *tableModel, f *fieldPlan, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "const %sGetBy%sSQL = `%s`\n\n", lower, f.goField, t.getBySQL(f))
	fmt.Fprintf(b, "// %sGetBy%s fetches the %s row with the given %s (a unique column),\n// or rt.ErrNotFound.\n", t.model, f.goField, tbl, f.colName)
	fmt.Fprintf(b, "func (q *Queries) %sGetBy%s(ctx context.Context, %s %s) (%s, error) {\n", t.model, f.goField, f.arg, f.goType, t.model)
	fmt.Fprintf(b, "\treturn %sScan(q.db.QueryRowContext(ctx, %sGetBy%sSQL, sql.Named(%q, %s)))\n}\n\n", lower, lower, f.goField, f.param, f.arg)
}

func (e *queryEmitter) listEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "const %sListSQL = `%s`\n\n", lower, t.listSQL())
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
		fmt.Fprintf(b, "\targs = append(args, c.data.%s)\n", f.goField)
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
		fmt.Fprintf(b, "\tif c.set[%d] {\n\t\tcols, args = append(cols, %q), append(args, v.%s)\n\t}\n", t.index(f), f.colName, f.goField)
	}
}

func (e *queryEmitter) deleteEmit(t *tableModel, lower, tbl string) {
	b := &e.body
	fmt.Fprintf(b, "const %sDeleteSQL = `%s`\n\n", lower, t.deleteSQL())
	fmt.Fprintf(b, "// %sDelete removes the identified %s row; rt.ErrNotFound reports\n// that nothing matched.\n", t.model, tbl)
	fmt.Fprintf(b, "func (q *Queries) %sDelete(ctx context.Context, %s) error {\n", t.model, identityParams(t))
	fmt.Fprintf(b, "\tres, err := q.db.ExecContext(ctx, %sDeleteSQL, %s)\n", lower, identityArgs(t))
	fmt.Fprintf(b, "\tif err != nil {\n\t\treturn rt.Refused(%q, err)\n\t}\n", t.model)
	fmt.Fprintf(b, "\tn, err := res.RowsAffected()\n\tif err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(b, "\tif n == 0 {\n\t\treturn rt.ErrNotFound\n\t}\n\treturn nil\n}\n\n")
}
