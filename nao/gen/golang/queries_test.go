package golang

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	sqlitegen "github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/rt"
)

type stmtCase struct {
	SQL string `json:"sql"`
	N   int    `json:"n"`
}

func planStatements(t *testing.T, p *plan) map[string]stmtCase {
	t.Helper()
	type m = struct{}
	stmts := map[string]stmtCase{}
	static := func(name, sql string, args []string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stmts[name] = stmtCase{SQL: sql, N: len(args)}
	}
	cols := func(fields []*fieldPlan) []string {
		out := make([]string, len(fields))
		for i, f := range fields {
			out[i] = f.colName
		}
		return out
	}
	for _, tm := range p.tables {
		if len(tm.fields) == 0 {
			continue
		}
		table, returning := rt.Ident(tm.sqlName), tm.columnList()
		if len(tm.pk) > 0 {
			sql, args, err := tm.getSQL()
			static(tm.model+"Get", sql, args, err)
			if len(tm.pk) == 1 {
				many, margs, err := rt.SelectRender(table, returning, []rt.Opt[m]{rt.Column[m, any]{Name: tm.pk[0].colName}.In(nil)})
				if err != nil {
					t.Fatalf("%sGetMany: %v", tm.model, err)
				}
				stmts[tm.model+"GetMany"] = stmtCase{SQL: many, N: len(margs)}
			}
			if set := cols(tm.nonPK()); len(set) > 0 {
				key := cols(tm.pk)
				stmts[tm.model+"UpdateOne"] = stmtCase{SQL: rt.UpdateRowRender(table, set[:1], key, returning), N: 1 + len(key)}
				stmts[tm.model+"UpdateAll"] = stmtCase{SQL: rt.UpdateRowRender(table, set, key, returning), N: len(set) + len(key)}
			}
			sql, args, err = tm.deleteSQL()
			static(tm.model+"Delete", sql, args, err)
		}
		for _, f := range tm.uniqueFields() {
			sql, args, err := tm.getBySQL(f)
			static(tm.model+"GetBy"+f.goField, sql, args, err)
		}
		sql, args, err := tm.listSQL()
		static(tm.model+"List", sql, args, err)
		stmts[tm.model+"CreateDefaults"] = stmtCase{SQL: rt.InsertRender(table, nil, returning), N: 0}
		if set := cols(tm.settable()); len(set) > 0 {
			stmts[tm.model+"CreateOne"] = stmtCase{SQL: rt.InsertRender(table, set[:1], returning), N: 1}
			stmts[tm.model+"CreateAll"] = stmtCase{SQL: rt.InsertRender(table, set, returning), N: len(set)}
		}
	}
	return stmts
}

func planDynStatements(t *testing.T, p *plan) map[string]stmtCase {
	t.Helper()
	type m = struct{}
	stmts := map[string]stmtCase{}
	add := func(name, sql string, args []any, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stmts[name] = stmtCase{SQL: sql, N: len(args)}
	}
	for _, tm := range p.tables {
		if len(tm.fields) == 0 {
			continue
		}
		table := rt.Ident(tm.sqlName)
		cols := tm.columnList()
		var preds []rt.Pred[m]
		var orders []rt.Order[m]
		var key []any
		for _, f := range tm.fields {
			c := rt.Column[m, any]{Name: f.colName}
			preds = append(preds, c.Eq(nil))
			orders = append(orders, c.Asc())
			key = append(key, nil)
		}
		first := rt.Column[m, any]{Name: tm.fields[0].colName}

		sql, args, err := rt.SelectRender(table, cols, []rt.Opt[m]{
			rt.And(preds...),
			rt.Or(first.IsNull(), rt.Not(first.Eq(nil)), first.In(nil, nil), first.EqCol(first), first.InList(nil)),
			rt.Distinct[m](),
			rt.OrderBy(orders...),
			rt.After[m](key...),
			rt.Limit[m](10),
			rt.Offset[m](5),
		})
		add(tm.model+"DynQuery", sql, args, err)

		sql, args, err = rt.CountRender(table, preds)
		add(tm.model+"DynCount", sql, args, err)
		sql, args, err = rt.ExistsRender(table, preds)
		add(tm.model+"DynExists", sql, args, err)
		sql, args, err = rt.DeleteRender(table, []rt.Pred[m]{first.Eq(nil)})
		add(tm.model+"DynDeleteWhere", sql, args, err)
		sql, args, err = rt.UpdateRender(table, []rt.Assign[m]{first.Set(nil)}, []rt.Pred[m]{first.Eq(nil)})
		add(tm.model+"DynUpdateWhere", sql, args, err)
	}
	return stmts
}

func TestQueriesPrepare(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	const driver = `
import sqlite3, sys, json, datetime, uuid
inp = json.load(sys.stdin)
con = sqlite3.connect(":memory:")
con.execute("PRAGMA foreign_keys = ON")
def _now(): return datetime.datetime.now().isoformat(sep=" ")
con.create_function("now", 0, _now, deterministic=True)
con.create_function("getdate", 0, _now, deterministic=True)
con.create_function("uuid_generate_v4", 0, lambda: str(uuid.uuid4()), deterministic=True)
con.executescript(inp["ddl"])
for name, case in sorted(inp["stmts"].items()):
    stmt, n = case["sql"], case["n"]
    try:
        con.execute("EXPLAIN " + stmt, [None] * n)
    except Exception as e:
        sys.exit(f"{name}: {e}\n  {stmt}")
`
	for _, schema := range corpusSchemas(t) {
		schema := schema
		t.Run(strings.TrimSuffix(strings.TrimPrefix(schema, "../testdata/"), ".volt"), func(t *testing.T) {
			f, info := parseChecked(t, schema)
			ddl, err := sqlitegen.Generate(f, info, sqlitegen.Options{Source: schema})
			if err != nil {
				t.Fatalf("gen/sqlite: %v", err)
			}
			p, err := planBuild(f, info, nil)
			if err != nil {
				t.Fatalf("planBuild: %v", err)
			}
			stmts := planStatements(t, p)
			for name, c := range planDynStatements(t, p) {
				stmts[name] = c
			}
			input, err := json.Marshal(map[string]any{
				"ddl":   string(ddl),
				"stmts": stmts,
			})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-c", driver)
			cmd.Stdin = strings.NewReader(string(input))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("statement does not prepare against the generated DDL: %v\n%s", err, out)
			}
		})
	}
}

func TestQueriesGenerationErrors(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{
			name:    "model collision after singularization",
			src:     "Table users { id int [pk] }\nTable \"user\" { id int [pk] }",
			wantErr: "both map to Go type",
		},
		{
			name:    "reserved model name",
			src:     "Table jobs [model: 'Queries'] { id int [pk] }",
			wantErr: "Queries",
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

func TestArgNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ID", "id"},
		{"UserID", "userID"},
		{"APIKey", "apiKey"},
		{"HTTPStatus", "httpStatus"},
		{"Name", "name"},
		{"X2faCode", "x2faCode"},
		{"Type", "type_"},
		{"Ctx", "ctx_"},
	}
	for _, tc := range cases {
		if got := argName(tc.in); got != tc.want {
			t.Errorf("argName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
