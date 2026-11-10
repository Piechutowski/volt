package rt

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type vanishNode struct {
	kind  string
	col   string
	param string
	kids  []vanishNode
}

func (n vanishNode) String() string {
	switch n.kind {
	case "cmp":
		if n.param == "" {
			return n.col + " = 1"
		}
		return n.col + " = :" + n.param + "?"
	case "null":
		return n.col + " is null"
	case "not":
		return "not (" + n.kids[0].String() + ")"
	case "exists":
		return "exists k (" + n.kids[0].String() + ")"
	}
	parts := make([]string, len(n.kids))
	for i, k := range n.kids {
		parts[i] = "(" + k.String() + ")"
	}
	return strings.Join(parts, " "+n.kind+" ")
}

func (n vanishNode) gen() GenPred {
	switch n.kind {
	case "cmp":
		if n.param == "" {
			p, _ := PredCmp(n.col, "=", "lit")
			return p
		}
		p, _ := PredCmp(n.col, "=", n.param)
		return PredOptional([]any{n.param}, p)
	case "null":
		return PredNull(n.col)
	case "not":
		return Not(n.kids[0].gen())
	case "exists":
		return PredExists("k", "k", [][2]string{{"k.t_id", "t.id"}}, n.kids[0].gen())
	}
	kids := make([]GenPred, len(n.kids))
	for i, k := range n.kids {
		kids[i] = k.gen()
	}
	if n.kind == "or" {
		return Or(kids...)
	}
	return And(kids...)
}

type vanishRow map[string]any

const (
	tvFalse = iota
	tvTrue
	tvNull
)

func (n vanishNode) out(env map[string]any) bool {
	switch n.kind {
	case "cmp":
		return n.param != "" && env[n.param] == nil
	case "null":
		return false
	case "not", "exists":
		return n.kids[0].out(env)
	}
	for _, k := range n.kids {
		if !k.out(env) {
			return false
		}
	}
	return true
}

func (n vanishNode) eval(row vanishRow, kids []vanishRow, env map[string]any) int {
	switch n.kind {
	case "cmp":
		v := env["lit"]
		if n.param != "" {
			v = env[n.param]
		}
		c := row[n.col]
		if c == nil || v == nil {
			return tvNull
		}
		if c == v {
			return tvTrue
		}
		return tvFalse
	case "null":
		if row[n.col] == nil {
			return tvTrue
		}
		return tvFalse
	case "not":
		return [...]int{tvTrue, tvFalse, tvNull}[n.kids[0].eval(row, kids, env)]
	case "exists":
		for _, k := range kids {
			if k["k.t_id"] == row["t.id"] && n.kids[0].eval(k, nil, env) == tvTrue {
				return tvTrue
			}
		}
		return tvFalse
	}
	and := n.kind == "and"
	got := tvTrue
	if !and {
		got = tvFalse
	}
	for _, k := range n.kids {
		if k.out(env) {
			continue
		}
		v := k.eval(row, kids, env)
		switch {
		case and && v == tvFalse, !and && v == tvTrue:
			return v
		case v == tvNull:
			got = tvNull
		}
	}
	return got
}

// TestPredVanishAgainstAReference proves the one constant text of
// PredVanish (spec §predicates, the vanishing rule) on SQLite against an
// evaluator of the rule as the spec states it: a term reading an absent
// parameter is taken out, an and, an or, a not or an exists term all of
// whose terms are out is out itself, a null test never is, and a where
// that is out keeps every row, over trees two levels deep of every form,
// for every assignment of two optional parameters, NULL columns among
// the rows.
func TestPredVanishAgainstAReference(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	rows := []vanishRow{
		{"t.id": int64(1), "t.a": int64(1), "t.b": int64(1)},
		{"t.id": int64(2), "t.a": int64(2), "t.b": nil},
		{"t.id": int64(3), "t.a": nil, "t.b": int64(2)},
		{"t.id": int64(4), "t.a": nil, "t.b": nil},
		{"t.id": int64(5), "t.a": int64(1), "t.b": int64(2)},
	}
	kids := []vanishRow{
		{"k.t_id": int64(1), "k.x": int64(1)},
		{"k.t_id": int64(1), "k.x": nil},
		{"k.t_id": int64(2), "k.x": int64(2)},
		{"k.t_id": int64(3), "k.x": int64(1)},
		{"k.t_id": int64(3), "k.x": int64(2)},
		{"k.t_id": int64(5), "k.x": nil},
	}
	stmts := []string{"CREATE TABLE t (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER)", "CREATE TABLE k (t_id INTEGER, x INTEGER)"}
	for _, r := range rows {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO t VALUES (%v, %v, %v)", r["t.id"], sqlLit(r["t.a"]), sqlLit(r["t.b"])))
	}
	for _, k := range kids {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO k VALUES (%v, %v)", k["k.t_id"], sqlLit(k["k.x"])))
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	level := func(atoms []vanishNode, inner []vanishNode) []vanishNode {
		out := slices.Clone(atoms)
		for _, a := range atoms {
			out = append(out, vanishNode{kind: "not", kids: []vanishNode{a}})
			for _, b := range atoms {
				out = append(out, vanishNode{kind: "and", kids: []vanishNode{a, b}}, vanishNode{kind: "or", kids: []vanishNode{a, b}})
			}
		}
		for _, in := range inner {
			out = append(out, vanishNode{kind: "exists", kids: []vanishNode{in}})
		}
		return out
	}
	outer := []vanishNode{{kind: "cmp", col: "t.a", param: "p"}, {kind: "cmp", col: "t.b", param: "q"}, {kind: "cmp", col: "t.a"}, {kind: "null", col: "t.b"}}
	innerAtoms := []vanishNode{{kind: "cmp", col: "k.x", param: "p"}, {kind: "cmp", col: "k.x", param: "q"}, {kind: "cmp", col: "k.x"}, {kind: "null", col: "k.x"}}
	one := level(outer, level(innerAtoms, nil))
	trees := slices.Clone(one)
	for _, x := range one {
		trees = append(trees, vanishNode{kind: "not", kids: []vanishNode{x}})
		for _, a := range outer {
			trees = append(trees,
				vanishNode{kind: "and", kids: []vanishNode{x, a}}, vanishNode{kind: "or", kids: []vanishNode{x, a}},
				vanishNode{kind: "and", kids: []vanishNode{a, x}}, vanishNode{kind: "or", kids: []vanishNode{a, x}})
		}
	}
	values := []any{nil, int64(1), int64(2)}
	queries := 0
	for _, tree := range trees {
		where, args, err := WhereRender([]GenPred{PredVanish(tree.gen())})
		if err != nil {
			t.Fatalf("%s: %v", tree, err)
		}
		for _, p := range values {
			for _, q := range values {
				env := map[string]any{"p": p, "q": q, "lit": int64(1)}
				var want []int64
				for _, r := range rows {
					if tree.out(env) || tree.eval(r, kids, env) == tvTrue {
						want = append(want, r["t.id"].(int64))
					}
				}
				bound := make([]any, len(args))
				for i, a := range args {
					bound[i] = env[a.(string)]
				}
				got, err := vanishIDs(db, "SELECT id FROM t WHERE "+where+" ORDER BY id", bound)
				if err != nil {
					t.Fatalf("%s: %v\n%s", tree, err, where)
				}
				queries++
				if !slices.Equal(got, want) {
					t.Fatalf("%s with p=%v q=%v keeps %v; the rule keeps %v\n%s", tree, p, q, got, want, where)
				}
			}
		}
	}
	if queries < 10000 {
		t.Fatalf("only %d queries ran", queries)
	}
}

func sqlLit(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprint(v)
}

func vanishIDs(db *sql.DB, query string, args []any) ([]int64, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
