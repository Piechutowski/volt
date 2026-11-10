package dbtest

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStatementPrepares(t *testing.T) {
	db := Open(t, `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`)
	cases := []struct {
		name, sql string
		n         int
		ok        bool
	}{
		{"matching", `SELECT "id", "title" FROM "posts" WHERE "id" = ?`, 1, true},
		{"unknown column", `SELECT body FROM "posts"`, 0, false},
		{"unknown table", `SELECT 1 FROM "authors"`, 0, false},
		{"too few arguments", `SELECT 1 FROM "posts" WHERE "id" = ? AND "title" = ?`, 1, false},
		{"too many arguments", `SELECT 1 FROM "posts" WHERE "id" = ?`, 2, false},
	}
	for _, c := range cases {
		if err := StatementPrepares(db, c.sql, c.n); (err == nil) != c.ok {
			t.Errorf("%s: StatementPrepares = %v, want ok %v", c.name, err, c.ok)
		}
	}
	var n int
	if err := db.Pool().QueryRow(`SELECT count(*) FROM "posts"`).Scan(&n); err != nil || n != 0 {
		t.Errorf("explaining wrote rows or failed: n=%d err=%v", n, err)
	}
}

func TestOpenRefusesBadDDL(t *testing.T) {
	ft := &fatalT{TB: t}
	func() {
		defer func() { _ = recover() }()
		Open(ft, `CREATE TABLE posts (id INTEGER PRIMARY KEY,)`)
	}()
	if !ft.failed {
		t.Error("Open accepted DDL SQLite rejects")
	}
}

type fatalT struct {
	testing.TB
	failed bool
}

func (f *fatalT) Fatalf(string, ...any) {
	f.failed = true
	panic("fatal")
}
