package rt

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func migrateDB(t *testing.T) *sql.DB {
	t.Helper()
	return migrateDBAt(t, "sqlite3", filepath.Join(t.TempDir(), "m.db"))
}

func migrateDBAt(t *testing.T, driver, path string) *sql.DB {
	t.Helper()
	db, err := Open(driver, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var cancelHook struct {
	sync.Mutex
	fn func()
}

func init() {
	sql.Register("sqlite3_cancel", &sqlite3.SQLiteDriver{
		ConnectHook: func(c *sqlite3.SQLiteConn) error {
			return c.RegisterFunc("cancel_ctx", func() int64 {
				cancelHook.Lock()
				defer cancelHook.Unlock()
				if cancelHook.fn != nil {
					cancelHook.fn()
				}
				return 1
			}, false)
		},
	})
}

var blogMigrations = []Migration{
	{
		Name: "0001__posts",
		Up:   []string{"CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)"},
		Down: []string{"DROP TABLE posts"},
	},
	{
		Name: "0002__comments",
		Up: []string{
			"CREATE TABLE comments (id INTEGER PRIMARY KEY, post_id INTEGER NOT NULL, FOREIGN KEY (post_id) REFERENCES posts (id))",
			"CREATE INDEX idx_comments_post_id ON comments (post_id)",
		},
		Down: []string{"DROP INDEX idx_comments_post_id", "DROP TABLE comments"},
	},
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMigrateUpIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	applied, err := MigrateUp(ctx, db, blogMigrations)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(applied, ",") != "0001__posts,0002__comments" {
		t.Fatalf("applied %v", applied)
	}
	again, err := MigrateUp(ctx, db, blogMigrations)
	if err != nil || len(again) != 0 {
		t.Fatalf("second up: %v %v, want nothing applied", again, err)
	}
	status, err := MigrateStatus(ctx, db, blogMigrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 2 {
		t.Fatalf("%d status rows", len(status))
	}
	for _, s := range status {
		if !s.Applied || s.AppliedAt == "" || s.OutOfOrder || s.Note != "" {
			t.Errorf("%+v, want applied with a time", s)
		}
		if !strings.HasSuffix(s.AppliedAt, "Z") {
			t.Errorf("%s applied_at %q is not UTC RFC 3339", s.Name, s.AppliedAt)
		}
	}
	if !tableExists(t, db, "comments") || !tableExists(t, db, "volt_migrations") {
		t.Error("tables missing after up")
	}
}

func TestMigrateDownRevertsTheLast(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	if _, err := MigrateUp(ctx, db, blogMigrations); err != nil {
		t.Fatal(err)
	}
	name, err := MigrateDown(ctx, db, blogMigrations)
	if err != nil || name != "0002__comments" {
		t.Fatalf("down: %q %v", name, err)
	}
	if tableExists(t, db, "comments") || !tableExists(t, db, "posts") {
		t.Error("down did not revert exactly the last migration")
	}
	status, err := MigrateStatus(ctx, db, blogMigrations)
	if err != nil {
		t.Fatal(err)
	}
	if status[1].Applied || status[1].OutOfOrder {
		t.Errorf("after down: %+v, want pending in order", status[1])
	}
	irreversible := []Migration{blogMigrations[0], {Name: "0002__comments", Up: blogMigrations[1].Up}}
	if _, err := MigrateUp(ctx, db, irreversible); err != nil {
		t.Fatal(err)
	}
	_, err = MigrateDown(ctx, db, irreversible)
	if err == nil || !strings.Contains(err.Error(), "0002__comments") || !strings.Contains(err.Error(), "irreversible") {
		t.Fatalf("down over an irreversible migration: %v, want a refusal naming it", err)
	}
	if !tableExists(t, db, "comments") {
		t.Error("the refusal dropped the table")
	}
}

func TestMigrateStatusMarksOutOfOrder(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	later := Migration{Name: "0003__tags", Up: []string{"CREATE TABLE tags (id INTEGER PRIMARY KEY)"}, Down: []string{"DROP TABLE tags"}}
	if _, err := MigrateUp(ctx, db, []Migration{blogMigrations[0], later}); err != nil {
		t.Fatal(err)
	}
	all := []Migration{blogMigrations[0], blogMigrations[1], later}
	status, err := MigrateStatus(ctx, db, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 3 || status[1].Name != "0002__comments" || status[1].Applied || !status[1].OutOfOrder {
		t.Fatalf("status %+v, want 0002 pending out of order", status)
	}
	if status[2].OutOfOrder || !status[2].Applied {
		t.Errorf("0003: %+v", status[2])
	}
	applied, err := MigrateUp(ctx, db, all)
	if err != nil || strings.Join(applied, ",") != "0002__comments" {
		t.Fatalf("up applies the out-of-order one: %v %v", applied, err)
	}
	status, err = MigrateStatus(ctx, db, blogMigrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 3 || status[2].Name != "0003__tags" || !status[2].Applied || status[2].Note != "not in the package" {
		t.Fatalf("status without 0003 in the package: %+v, want its ledger row noted", status)
	}
}

func TestMigrateBracketRollsBackDanglingForeignKeys(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	if _, err := MigrateUp(ctx, db, blogMigrations); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO posts (id, title) VALUES (1, 'a'); INSERT INTO comments (id, post_id) VALUES (1, 1)"); err != nil {
		t.Fatal(err)
	}
	drop := Migration{
		Name:           "0003__drop_posts",
		Up:             []string{"DROP TABLE posts", "CREATE TABLE posts_log (id INTEGER PRIMARY KEY)"},
		ForeignKeysOff: true,
	}
	_, err := MigrateUp(ctx, db, append(blogMigrations, drop))
	if err == nil || !strings.Contains(err.Error(), "0003__drop_posts") || !strings.Contains(err.Error(), "comments") {
		t.Fatalf("bracket: %v, want the check failure naming the migration and the table", err)
	}
	if !tableExists(t, db, "posts") || tableExists(t, db, "posts_log") {
		t.Error("the failed migration was not rolled back")
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("PRAGMA foreign_keys = %d after the bracket, want 1", fk)
	}
	status, err := MigrateStatus(ctx, db, append(blogMigrations, drop))
	if err != nil {
		t.Fatal(err)
	}
	if status[2].Applied {
		t.Error("the ledger recorded a rolled-back migration")
	}
	rebuild := Migration{
		Name: "0003__rebuild_posts",
		Up: []string{
			"CREATE TABLE posts__new (id INTEGER PRIMARY KEY, title TEXT NOT NULL DEFAULT '')",
			"INSERT INTO posts__new (id, title) SELECT id, title FROM posts",
			"DROP TABLE posts",
			"ALTER TABLE posts__new RENAME TO posts",
		},
		ForeignKeysOff: true,
	}
	if _, err := MigrateUp(ctx, db, append(blogMigrations, rebuild)); err != nil {
		t.Fatalf("a rebuild under the bracket: %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM comments c JOIN posts p ON p.id = c.post_id").Scan(&n); err != nil || n != 1 {
		t.Errorf("rows after the rebuild: %d %v", n, err)
	}
}

func TestMigrateNoTransactionRunsWhatATransactionCannot(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	vacuum := Migration{Name: "0001__vacuum", Up: []string{"VACUUM"}}
	_, err := MigrateUp(ctx, db, []Migration{vacuum})
	if err == nil || !strings.Contains(err.Error(), "statement 1") || !strings.Contains(err.Error(), "VACUUM") {
		t.Fatalf("VACUUM in a transaction: %v, want the statement error", err)
	}
	vacuum.NoTransaction = true
	vacuum.Up = append(vacuum.Up, "PRAGMA journal_mode = DELETE")
	applied, err := MigrateUp(ctx, db, []Migration{vacuum})
	if err != nil || len(applied) != 1 {
		t.Fatalf("no transaction: %v %v", applied, err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Errorf("journal_mode %q %v, want delete", mode, err)
	}
	status, err := MigrateStatus(ctx, db, []Migration{vacuum})
	if err != nil || !status[0].Applied {
		t.Errorf("status after a no-transaction migration: %+v %v", status, err)
	}
}

func TestMigrateUpConcurrentRunnersApplyOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "shared.db")
	var all []Migration
	for _, table := range []string{"posts", "comments", "tags"} {
		all = append(all, Migration{
			Name: fmt.Sprintf("%04d__%s", len(all)+1, table),
			Up:   []string{fmt.Sprintf("CREATE TABLE %s AS WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 200000) SELECT count(*) AS n FROM c", table)},
			Down: []string{"DROP TABLE " + table},
		})
	}
	const runners = 2
	dbs := make([]*sql.DB, runners)
	applied := make([][]string, runners)
	errs := make([]error, runners)
	for i := range dbs {
		dbs[i] = migrateDBAt(t, "sqlite3", path)
	}
	var start, done sync.WaitGroup
	start.Add(1)
	for i := range dbs {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			applied[i], errs[i] = MigrateUp(ctx, dbs[i], all)
		}()
	}
	start.Done()
	done.Wait()
	total := 0
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("runner %d: %v", i, errs[i])
		}
		total += len(applied[i])
	}
	if total != len(all) {
		t.Fatalf("%d applications of %d migrations: %v", total, len(all), applied)
	}
	var rows int
	if err := dbs[0].QueryRow("SELECT count(*) FROM volt_migrations").Scan(&rows); err != nil || rows != len(all) {
		t.Errorf("%d ledger rows %v, want %d", rows, err, len(all))
	}
	for _, table := range []string{"posts", "comments", "tags"} {
		if !tableExists(t, dbs[1], table) {
			t.Errorf("table %s missing after two runners", table)
		}
	}
}

func TestMigrateNoTransactionDisablesTheBracket(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	seen := Migration{
		Name:           "0001__fk_seen",
		Up:             []string{"CREATE TABLE fk_seen AS SELECT foreign_keys AS fk FROM pragma_foreign_keys"},
		NoTransaction:  true,
		ForeignKeysOff: true,
	}
	if _, err := MigrateUp(ctx, db, []Migration{seen}); err != nil {
		t.Fatal(err)
	}
	var fk int
	if err := db.QueryRow("SELECT fk FROM fk_seen").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys during a no-transaction migration = %d %v, want 1: the bracket is off", fk, err)
	}
}

func TestMigrateBracketRestoresForeignKeysOnCancel(t *testing.T) {
	db := migrateDBAt(t, "sqlite3_cancel", filepath.Join(t.TempDir(), "c.db"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelHook.Lock()
	cancelHook.fn = cancel
	cancelHook.Unlock()
	defer func() {
		cancelHook.Lock()
		cancelHook.fn = nil
		cancelHook.Unlock()
	}()
	cancelled := Migration{
		Name:           "0001__cancelled",
		Up:             []string{"SELECT cancel_ctx()", "CREATE TABLE after_cancel (id INTEGER PRIMARY KEY)"},
		ForeignKeysOff: true,
	}
	if _, err := MigrateUp(ctx, db, []Migration{cancelled}); err == nil {
		t.Fatal("a context cancelled inside the migration applied it")
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("PRAGMA foreign_keys = %d %v after the cancelled bracket, want 1", fk, err)
	}
	if tableExists(t, db, "after_cancel") {
		t.Error("the cancelled migration was not rolled back")
	}
	status, err := MigrateStatus(context.Background(), db, []Migration{cancelled})
	if err != nil || status[0].Applied {
		t.Errorf("status after the cancelled migration: %+v %v, want pending", status, err)
	}
}

func TestMigrateStatusOnlyReads(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ro.db")
	db := migrateDBAt(t, "sqlite3", path)
	status, err := MigrateStatus(ctx, db, blogMigrations)
	if err != nil || len(status) != 2 || status[0].Applied || status[1].Applied {
		t.Fatalf("status over an empty database: %+v %v, want two pending rows", status, err)
	}
	if _, err := MigrateDown(ctx, db, blogMigrations); err == nil || !strings.Contains(err.Error(), "no migration is applied") {
		t.Fatalf("down over an empty database: %v, want the refusal", err)
	}
	if tableExists(t, db, "volt_migrations") {
		t.Fatal("status or down created the ledger")
	}
	if _, err := MigrateUp(ctx, db, blogMigrations); err != nil {
		t.Fatal(err)
	}
	ro, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	status, err = MigrateStatus(ctx, ro, blogMigrations)
	if err != nil || len(status) != 2 || !status[0].Applied || !status[1].Applied {
		t.Fatalf("status over a read-only connection: %+v %v, want two applied rows", status, err)
	}
}

func TestMigrateUpReportsTheFailingStatement(t *testing.T) {
	ctx := context.Background()
	db := migrateDB(t)
	bad := Migration{Name: "0001__bad", Up: []string{"CREATE TABLE a (id INTEGER)", "CREATE TABLE a (id INTEGER)"}}
	applied, err := MigrateUp(ctx, db, []Migration{bad})
	if err == nil || len(applied) != 0 {
		t.Fatalf("%v %v", applied, err)
	}
	for _, want := range []string{"migration 0001__bad", "statement 2", "CREATE TABLE a (id INTEGER)", "already exists"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if tableExists(t, db, "a") {
		t.Error("the failed migration left its first statement applied")
	}
}
