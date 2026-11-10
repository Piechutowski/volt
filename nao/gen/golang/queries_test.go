package golang

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/dbtest"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	sqlitegen "github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/naming"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
	_ "github.com/mattn/go-sqlite3"
)

type stmtCase struct {
	SQL string
	N   int
}

func planStatements(t *testing.T, p *naming.Plan) map[string]stmtCase {
	t.Helper()
	type m = struct{}
	stmts := map[string]stmtCase{}
	static := func(name, sql string, args []string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stmts[name] = stmtCase{SQL: sql, N: len(args)}
	}
	cols := func(fields []*naming.Field) []string {
		out := make([]string, len(fields))
		for i, f := range fields {
			out[i] = f.ColName
		}
		return out
	}
	for _, tm := range p.Tables() {
		if len(tm.Fields) == 0 {
			continue
		}
		table, returning := rt.Ident(tm.SQLName), tm.ColumnList()
		if len(tm.PK) > 0 {
			sql, args, err := tm.GetSQL()
			static(tm.Model+"Get", sql, args, err)
			if len(tm.PK) == 1 {
				many, margs, err := rt.SelectRender(table, returning, []rt.Opt[m]{rt.Column[m, any]{Name: tm.PK[0].ColName}.In(nil)})
				if err != nil {
					t.Fatalf("%sGetMany: %v", tm.Model, err)
				}
				stmts[tm.Model+"GetMany"] = stmtCase{SQL: many, N: len(margs)}
			}
			if set := cols(tm.NonPK()); len(set) > 0 {
				key := cols(tm.PK)
				stmts[tm.Model+"UpdateOne"] = stmtCase{SQL: rt.UpdateRowRender(table, set[:1], key, returning), N: 1 + len(key)}
				stmts[tm.Model+"UpdateAll"] = stmtCase{SQL: rt.UpdateRowRender(table, set, key, returning), N: len(set) + len(key)}
			}
			sql, args, err = tm.DeleteSQL()
			static(tm.Model+"Delete", sql, args, err)
		}
		for _, f := range tm.UniqueFields() {
			sql, args, err := tm.GetBySQL(f)
			static(tm.Model+"GetBy"+f.GoField, sql, args, err)
		}
		sql, args, err := tm.ListSQL()
		static(tm.Model+"List", sql, args, err)
		stmts[tm.Model+"CreateDefaults"] = stmtCase{SQL: rt.InsertRender(table, nil, returning), N: 0}
		if set := cols(tm.Fields); len(set) > 0 {
			stmts[tm.Model+"CreateOne"] = stmtCase{SQL: rt.InsertRender(table, set[:1], returning), N: 1}
			stmts[tm.Model+"CreateAll"] = stmtCase{SQL: rt.InsertRender(table, set, returning), N: len(set)}
		}
	}
	return stmts
}

func planDynStatements(t *testing.T, p *naming.Plan) map[string]stmtCase {
	t.Helper()
	type m = struct{}
	stmts := map[string]stmtCase{}
	add := func(name, sql string, args []any, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stmts[name] = stmtCase{SQL: sql, N: len(args)}
	}
	for _, tm := range p.Tables() {
		if len(tm.Fields) == 0 {
			continue
		}
		table := rt.Ident(tm.SQLName)
		cols := tm.ColumnList()
		var preds []rt.Pred[m]
		var orders []rt.Order[m]
		var key []any
		for _, f := range tm.Fields {
			c := rt.Column[m, any]{Name: f.ColName}
			preds = append(preds, c.Eq(nil))
			orders = append(orders, c.Asc())
			key = append(key, nil)
		}
		first := rt.Column[m, any]{Name: tm.Fields[0].ColName}

		sql, args, err := rt.SelectRender(table, cols, []rt.Opt[m]{
			rt.And(preds...),
			rt.Or(first.IsNull(), rt.Not(first.Eq(nil)), first.In(nil, nil), first.EqCol(first), first.InList(nil)),
			rt.Distinct[m](),
			rt.OrderBy(orders...),
			rt.After[m](key...),
			rt.Limit[m](10),
			rt.Offset[m](5),
		})
		add(tm.Model+"DynQuery", sql, args, err)

		sql, args, err = rt.CountRender(table, preds)
		add(tm.Model+"DynCount", sql, args, err)
		sql, args, err = rt.ExistsRender(table, preds)
		add(tm.Model+"DynExists", sql, args, err)
		sql, args, err = rt.DeleteRender(table, []rt.Pred[m]{first.Eq(nil)})
		add(tm.Model+"DynDeleteWhere", sql, args, err)
		sql, args, err = rt.UpdateRender(table, []rt.Assign[m]{first.Set(nil)}, []rt.Pred[m]{first.Eq(nil)})
		add(tm.Model+"DynUpdateWhere", sql, args, err)
	}
	return stmts
}

func TestQueriesPrepare(t *testing.T) {
	for _, schema := range corpusSchemas(t) {
		t.Run(strings.TrimSuffix(strings.TrimPrefix(schema, "../testdata/"), ".volt"), func(t *testing.T) {
			f, info := parseChecked(t, schema)
			ddl, err := sqlitegen.Generate(f, info, sqlitegen.Options{Source: schema})
			if err != nil {
				t.Fatalf("gen/sqlite: %v", err)
			}
			p := naming.PlanBuild(f, info)
			if err := p.Err(); err != nil {
				t.Fatalf("PlanBuild: %v", err)
			}
			stmts := planStatements(t, p)
			for name, c := range planDynStatements(t, p) {
				stmts[name] = c
			}
			stmtsPrepare(t, dbtest.Open(t, string(ddl)), stmts)
		})
	}
}

// stmtsPrepare fails t for every statement of stmts, in name order, that
// does not prepare against db's schema with its number of arguments.
func stmtsPrepare(t *testing.T, db run.DB, stmts map[string]stmtCase) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(stmts)) {
		if err := dbtest.StatementPrepares(db, stmts[name].SQL, stmts[name].N); err != nil {
			t.Errorf("%s does not prepare against the generated DDL: %v", name, err)
		}
	}
}

func TestQueriesGenerationErrors(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{
			name:    "model collision after singularization",
			src:     "table users { id int [pk] }\ntable user { id int [pk] }",
			wantErr: "both map to Go type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, diags := parser.ParseFile("t", tc.src)
			info, semDiags := check.File(f)
			if diag.HasErrors(append(diags, semDiags...)) {
				t.Fatalf("input unexpectedly invalid: %v %v", diags, semDiags)
			}
			_, err := GenerateQueries(f, info, Options{Package: "models", Source: "t"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
