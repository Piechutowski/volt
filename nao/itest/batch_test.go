package itest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func usersNew(n, from int) []UserChangeset {
	cs := make([]UserChangeset, n)
	for i := range cs {
		cs[i] = UserNew().EmailSet(fmt.Sprintf("u%d@example.com", from+i)).NameSet(fmt.Sprintf("User %d", from+i))
	}
	return cs
}

func usersStored(t *testing.T, db run.DB) int64 {
	t.Helper()
	n, err := run.Run(context.Background(), db, UserCount())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestCreateManyInsertsEveryRowInOrder proves the bulk insert (spec
// §changesets, §generated-query-surface): 25,000 rows insert in
// statements of as many rows as SQLite's 32,766 parameters hold, the
// Batch yields how many it inserted, and the rows keep the order given.
func TestCreateManyInsertsEveryRowInOrder(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	cs := usersNew(25000, 0)
	b := UserCreateMany(cs)
	if len(b.Steps) != 2 {
		t.Fatalf("25000 rows of 2 marked columns took %d statements; want 2 of at most 16383 rows", len(b.Steps))
	}
	n, err := run.Run(ctx, db, b)
	if err != nil || n != 25000 {
		t.Fatalf("CreateMany = %d, %v; want 25000", n, err)
	}
	us, err := run.Run(ctx, db, UserList())
	if err != nil || len(us) != 25000 {
		t.Fatalf("UserList = %d rows, %v", len(us), err)
	}
	for i, u := range us {
		if want := fmt.Sprintf("u%d@example.com", i); u.Email != want {
			t.Fatalf("row %d holds %q; want %q, the order given", i, u.Email, want)
		}
	}
	if n, err := run.Run(ctx, db, UserCreateMany(nil)); err != nil || n != 0 {
		t.Fatalf("CreateMany of no rows = %d, %v; want 0", n, err)
	}
}

// TestCreateManyReportsEveryInvalidRow proves the refusal of a batch
// (spec §changesets): every row that does not validate is reported by
// its index, with its field errors and the row in its details, and no
// row is inserted.
func TestCreateManyReportsEveryInvalidRow(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	cs := usersNew(5, 0)
	cs[1] = cs[1].EmailSet("not an address")
	cs[3] = cs[3].EmailSet("nor this")
	_, err := run.Run(ctx, db, UserCreateMany(cs))
	var rows *rt.InvalidRows
	if !errors.As(err, &rows) {
		t.Fatalf("CreateMany with two invalid rows = %v; want *rt.InvalidRows", err)
	}
	if len(rows.Rows) != 2 || rows.Rows[0].Index != 1 || rows.Rows[1].Index != 3 {
		t.Fatalf("invalid rows %+v; want indexes 1 and 3", rows.Rows)
	}
	if !strings.Contains(err.Error(), "rows 1, 3 invalid") {
		t.Errorf("message %q lacks the indexes", err)
	}
	d := rows.Details()
	if len(d) != 2 || d[0].Row == nil || *d[0].Row != 1 || d[1].Row == nil || *d[1].Row != 3 || d[0].Check != "EmailValid" {
		t.Errorf("details %+v; want one EmailValid per invalid row, with its row", d)
	}
	if n := usersStored(t, db); n != 0 {
		t.Fatalf("%d users after a refused batch; want none", n)
	}
}

// TestCreateManyRollsBackARefusedRow proves that a batch is all or
// nothing (spec §changesets): a row the database refuses, here a
// duplicate email, rolls the whole batch back and maps onto a unique
// field error with no row, since SQLite names none; inside a caller's
// transaction the batch joins it and the transaction rolls back too.
func TestCreateManyRollsBackARefusedRow(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	cs := append(usersNew(20000, 0), usersNew(1, 5)...)
	_, err := run.Run(ctx, db, UserCreateMany(cs))
	var inv *rt.Invalid
	if !errors.As(err, &inv) || len(inv.Fields) != 1 || inv.Fields[0].Check != "unique" {
		t.Fatalf("a batch with a duplicate email = %v; want a unique field error", err)
	}
	if n := usersStored(t, db); n != 0 {
		t.Fatalf("%d users after a refused batch; want none", n)
	}
	_, err = run.Run(ctx, db, rt.Tx(UserCreateMany(usersNew(3, 100)), UserCreateMany(usersNew(1, 100))))
	if !errors.As(err, &inv) {
		t.Fatalf("a transaction whose second batch is refused = %v; want the refusal", err)
	}
	if n := usersStored(t, db); n != 0 {
		t.Fatalf("%d users after a refused transaction of batches; want none", n)
	}
}

// TestCreateManyGroupsRowsByTheirMarks proves the grouping of a batch
// (spec §changesets): consecutive rows that mark the same columns share
// a statement, a row that marks another set starts a new one, and every
// row is stored with what it marked.
func TestCreateManyGroupsRowsByTheirMarks(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	cs := usersNew(6, 0)
	cs[2] = cs[2].BioSet(rt.Some("two"))
	cs[3] = cs[3].BioSet(rt.Some("three"))
	b := UserCreateMany(cs)
	if len(b.Steps) != 3 {
		t.Fatalf("rows marking (2, 2, 3, 3, 2, 2) columns took %d statements; want 3", len(b.Steps))
	}
	if n, err := run.Run(ctx, db, b); err != nil || n != 6 {
		t.Fatalf("CreateMany = %d, %v", n, err)
	}
	us, err := run.Run(ctx, db, UserList())
	if err != nil || len(us) != 6 {
		t.Fatalf("UserList = %v, %v", us, err)
	}
	for i, u := range us {
		want := rt.Null[string]{}
		if i == 2 {
			want = rt.Some("two")
		}
		if i == 3 {
			want = rt.Some("three")
		}
		if u.Bio != want || u.CreatedAt.IsZero() {
			t.Errorf("row %d: bio %v, created_at %v; want bio %v and the column's default", i, u.Bio, u.CreatedAt, want)
		}
	}
}

// TestSQLiteParameterLimit pins the limit a batch's statements are cut
// to (spec §changesets): go-sqlite3's SQLite binds 32,766 parameters in
// one statement and refuses 32,767, so a change of the driver's limit
// fails here before it fails an import.
func TestSQLiteParameterLimit(t *testing.T) {
	db := newDB(t)
	if _, err := db.Pool().Exec("CREATE TABLE limits (a INTEGER)"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		n  int
		ok bool
	}{{32766, true}, {32767, false}} {
		q := "INSERT INTO limits (a) VALUES " + strings.TrimSuffix(strings.Repeat("(?), ", c.n), ", ")
		stmt, err := db.Pool().Prepare(q)
		if err == nil {
			stmt.Close()
		}
		if (err == nil) != c.ok {
			t.Errorf("a statement of %d parameters: %v; want ok %v", c.n, err, c.ok)
		}
	}
}
