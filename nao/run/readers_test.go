package run

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

func postsDB(t *testing.T) DB {
	t.Helper()
	db := migrateDB(t)
	if _, err := db.Pool().Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	return db
}

var postCount = rt.Lookup[int]{SQL: "SELECT count(*) FROM posts", Scan: rt.ScanOne[int]}

// TestReadBesideAnOpenWrite proves the two channels (spec
// §generated-query-surface, the runner): while the writer holds a write
// transaction it has not committed, a read, a transaction of reads and
// a parallel of reads answer at once from the committed snapshot on the
// readers, and see the write once it commits.
func TestReadBesideAnOpenWrite(t *testing.T) {
	db := postsDB(t)
	conn, err := db.Pool().Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "INSERT INTO posts (title) VALUES ('draft')"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if n, err := Run(ctx, db, postCount); err != nil || n != 0 {
		t.Fatalf("a read beside an open write = %d, %v; want 0 at once", n, err)
	}
	if out, err := Run(ctx, db, rt.Tx(postCount, postCount)); err != nil || out[0] != 0 || out[1] != 0 {
		t.Fatalf("a Tx of reads beside an open write = %v, %v; want the snapshot at once", out, err)
	}
	if out, err := Run(ctx, db, rt.All(postCount, postCount, postCount)); err != nil || len(out) != 3 {
		t.Fatalf("an All of reads beside an open write = %v, %v", out, err)
	}
	if _, err := conn.ExecContext(context.Background(), "COMMIT"); err != nil {
		t.Fatal(err)
	}
	if n, err := Run(context.Background(), db, postCount); err != nil || n != 1 {
		t.Fatalf("a read after the commit = %d, %v; want 1", n, err)
	}
}

// TestReadersFollowGOMAXPROCS proves the readers' size (spec
// §generated-query-surface, the runner): the pool holds as many
// connections, open and idle, as GOMAXPROCS, and follows it when it
// changes.
func TestReadersFollowGOMAXPROCS(t *testing.T) {
	defer runtime.SetDefaultGOMAXPROCS()
	db := postsDB(t)
	for _, n := range []int{2, 5, 3} {
		runtime.GOMAXPROCS(n)
		if _, err := Run(context.Background(), db, postCount); err != nil {
			t.Fatal(err)
		}
		if got := db.readers.pool.Stats().MaxOpenConnections; got != n {
			t.Fatalf("GOMAXPROCS %d: the readers allow %d connections", n, got)
		}
	}
}

// TestReadersRunReadsInParallel proves that reads outside a transaction
// share the readers (spec §generated-query-surface, the runner): an All
// of slow reads opens more than one reader and keeps them idle after.
func TestReadersRunReadsInParallel(t *testing.T) {
	defer runtime.SetDefaultGOMAXPROCS()
	runtime.GOMAXPROCS(4)
	db := postsDB(t)
	slow := rt.Lookup[int]{SQL: "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 1000000) SELECT count(*) FROM c", Scan: rt.ScanOne[int]}
	if _, err := Run(context.Background(), db, rt.All(slow, slow, slow, slow)); err != nil {
		t.Fatal(err)
	}
	if s := db.readers.pool.Stats(); s.OpenConnections < 2 || s.Idle != s.OpenConnections {
		t.Fatalf("after four slow reads at once the readers hold %d connections, %d idle; want more than one, all idle", s.OpenConnections, s.Idle)
	}
	if s := db.Pool().Stats(); s.MaxOpenConnections != 1 {
		t.Fatalf("the writer allows %d connections; want 1", s.MaxOpenConnections)
	}
}

// TestReaderRefusesWrites proves that a reader never writes (spec
// §generated-query-surface, the runner): a statement that writes fails
// under query_only, and a transaction that can write is refused before
// it begins.
func TestReaderRefusesWrites(t *testing.T) {
	ctx := context.Background()
	db := postsDB(t)
	r := db.reader()
	if _, err := r.Exec(ctx, "INSERT INTO posts (title) VALUES ('x')", nil); err == nil {
		t.Error("a reader ran an insert")
	}
	if err := r.Transaction(ctx, true, func(rt.Executor) error { return nil }); !errors.Is(err, errReaderWrite) {
		t.Errorf("a writing transaction on a reader = %v; want the refusal", err)
	}
	if n, err := Run(ctx, db, postCount); err != nil || n != 0 {
		t.Errorf("count = %d, %v; want 0", n, err)
	}
}
