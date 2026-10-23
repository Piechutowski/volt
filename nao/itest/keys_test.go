package itest

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/migrate"
	"github.com/Piechutowski/volt/nao/rt"
)

// TestCompositeKeyInDeclarationOrder runs the generated CRUD of
// daily_hits, whose key is its pk columns in declaration order, day then
// site, with hits between them (spec §columns, §generated-query-surface
// rule 1): the PRIMARY KEY lists day then site, and Get, Update and
// Delete take them in that order.
func TestCompositeKeyInDeclarationOrder(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)

	positions := map[string]int{}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(daily_hits)")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		positions[name] = pk
	}
	rows.Close()
	if positions["day"] != 1 || positions["site"] != 2 || positions["hits"] != 0 {
		t.Fatalf("primary key positions %v, want day 1 and site 2 in declaration order", positions)
	}

	hit, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(20261022).SiteSet("blog.example").HitsSet(7))
	if err != nil {
		t.Fatalf("DailyHitCreate: %v", err)
	}
	got, err := q.DailyHitGet(ctx, 20261022, "blog.example")
	if err != nil || got != hit {
		t.Fatalf("DailyHitGet(day, site) = %+v, %v; want %+v", got, err, hit)
	}
	if _, err := q.DailyHitGet(ctx, 20261023, "blog.example"); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("DailyHitGet on another day = %v; want rt.ErrNotFound", err)
	}
	_, err = q.DailyHitCreate(ctx, DailyHitNew().DaySet(20261022).SiteSet("blog.example"))
	if inv := invalidOf(t, err, "unique", "day", "site"); inv.StatusCode() != 409 {
		t.Errorf("a duplicate composite key is a 409 naming both columns: %+v", inv)
	}
	if _, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(20261023).SiteSet("blog.example")); err != nil {
		t.Errorf("the same site on another day is another key: %v", err)
	}

	updated, err := q.DailyHitUpdate(ctx, hit.Changeset().HitsSet(8))
	if err != nil || updated.Hits != 8 || updated.Day != hit.Day || updated.Site != hit.Site {
		t.Fatalf("DailyHitUpdate = %+v, %v; want hits 8 on the same key", updated, err)
	}
	if again, err := q.DailyHitGet(ctx, hit.Day, hit.Site); err != nil || again.Hits != 8 {
		t.Errorf("DailyHitGet after the update = %+v, %v", again, err)
	}
	if err := q.DailyHitDelete(ctx, hit.Day, hit.Site); err != nil {
		t.Fatalf("DailyHitDelete(day, site): %v", err)
	}
	if _, err := q.DailyHitGet(ctx, hit.Day, hit.Site); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("DailyHitGet after the delete = %v; want rt.ErrNotFound", err)
	}
	if n, err := q.DailyHitCount(ctx); err != nil || n != 1 {
		t.Errorf("the delete removes its key alone: %d rows, %v", n, err)
	}
}

// uniqueConstraintsSQL counts the indexes of the database's UNIQUE
// constraints, the sqlite_autoindex of origin u, leaving out those of
// a PRIMARY KEY constraint.
const uniqueConstraintsSQL = `SELECT count(*) FROM sqlite_master AS m, pragma_index_list(m.name) AS il WHERE m.type = 'table' AND il.origin = 'u'`

// indexSQL is the CREATE statement SQLite keeps for an index, and
// whether the index exists.
func indexSQL(t *testing.T, db *sql.DB, name string) (string, bool) {
	t.Helper()
	var text sql.NullString
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return text.String, true
}

// TestUniqueColumnIsAUniqueIndex proves that a unique column is a CREATE
// UNIQUE INDEX named idx_<table>_<column>, never an inline UNIQUE and
// its sqlite_autoindex (spec §indexes, §mapping-to-sqlite-ddl), and that
// the database enforces it through that index: a duplicate is the
// 409-style refusal naming the column.
func TestUniqueColumnIsAUniqueIndex(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)
	text, ok := indexSQL(t, db, "idx_users_email")
	if !ok || !strings.HasPrefix(text, "CREATE UNIQUE INDEX idx_users_email ON users (email)") {
		t.Fatalf("idx_users_email = %q, %v; want the unique column's CREATE UNIQUE INDEX", text, ok)
	}
	var autoindexes int
	if err := db.QueryRow(uniqueConstraintsSQL).Scan(&autoindexes); err != nil {
		t.Fatal(err)
	}
	if autoindexes != 0 {
		t.Errorf("%d indexes of a UNIQUE constraint (sqlite_autoindex): the DDL carries an inline UNIQUE", autoindexes)
	}

	seedUser(t, q, "ann@example.com")
	_, err := q.UserCreate(ctx, UserNew().EmailSet("ann@example.com").NameSet("Ann again"))
	if inv := invalidOf(t, err, "unique", "email"); inv.StatusCode() != 409 || inv.Cause == nil {
		t.Errorf("a duplicate of a unique column is a 409 naming the column, with the database's error as its cause: %+v", inv)
	}

	if _, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(1).SiteSet("a").TokenSet(rt.Some("t-1"))); err != nil {
		t.Fatal(err)
	}
	for _, site := range []string{"b", "c"} {
		if _, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(1).SiteSet(site)); err != nil {
			t.Errorf("NULL tokens do not collide under a unique index: %v", err)
		}
	}
	_, err = q.DailyHitCreate(ctx, DailyHitNew().DaySet(2).SiteSet("a").TokenSet(rt.Some("t-1")))
	if inv := invalidOf(t, err, "unique", "token"); inv.StatusCode() != 409 {
		t.Errorf("a duplicate token is a 409 naming the column: %+v", inv)
	}
}

// TestIndexColumn proves that a column carrying index is a plain CREATE
// INDEX named idx_<table>_<column> (spec §indexes): it admits
// duplicates, and SQLite plans a lookup by the column through it.
func TestIndexColumn(t *testing.T) {
	ctx := context.Background()
	db, q := newDB(t)
	text, ok := indexSQL(t, db, "idx_daily_hits_label")
	if !ok || !strings.HasPrefix(text, "CREATE INDEX idx_daily_hits_label ON daily_hits (label)") {
		t.Fatalf("idx_daily_hits_label = %q, %v; want the index column's CREATE INDEX", text, ok)
	}
	for _, site := range []string{"a", "b"} {
		if _, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(1).SiteSet(site).LabelSet("launch")); err != nil {
			t.Fatalf("an index column admits duplicates: %v", err)
		}
	}
	var plan strings.Builder
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN SELECT * FROM daily_hits WHERE label = ?", "launch")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	rows.Close()
	if !strings.Contains(plan.String(), "idx_daily_hits_label") {
		t.Errorf("a lookup by label does not use its index:\n%s", plan.String())
	}
	got, err := q.DailyHitQuery(ctx, DailyHitLabel.Eq("launch"))
	if err != nil || len(got) != 2 {
		t.Errorf("DailyHitQuery by label = %+v, %v; want both rows", got, err)
	}
}

// TestGetByUniqueColumn runs <Model>GetBy<Column> over unique columns, a
// NOT NULL one and a nullable one (spec §generated-query-surface):
// the row, or rt.ErrNotFound.
func TestGetByUniqueColumn(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	seedUser(t, q, "ann@example.com")
	inv, err := q.InviteCreate(ctx, InviteNew().UserEmailSet("ann@example.com").CodeSet("WELCOME"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := q.InviteGetByCode(ctx, "WELCOME"); err != nil || got != inv {
		t.Errorf("InviteGetByCode = %+v, %v; want %+v", got, err, inv)
	}
	if _, err := q.InviteGetByCode(ctx, "NOPE"); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("InviteGetByCode on a miss = %v; want rt.ErrNotFound", err)
	}
	hit, err := q.DailyHitCreate(ctx, DailyHitNew().DaySet(1).SiteSet("a").TokenSet(rt.Some("t-1")))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := q.DailyHitGetByToken(ctx, rt.Some("t-1")); err != nil || got != hit {
		t.Errorf("DailyHitGetByToken = %+v, %v; want %+v", got, err, hit)
	}
}

// TestForeignKeyToAUniqueIndexedColumn proves that a foreign key may
// reference a column whose uniqueness is a unique index (spec
// §relationships-ref): SQLite takes invites.user_email > users.email,
// enforces it on insert, and refuses to delete the referenced row.
func TestForeignKeyToAUniqueIndexedColumn(t *testing.T) {
	ctx := context.Background()
	_, q := newDB(t)
	u := seedUser(t, q, "ann@example.com")
	if _, err := q.InviteCreate(ctx, InviteNew().UserEmailSet("ann@example.com").CodeSet("A")); err != nil {
		t.Fatalf("an invite referencing a user's email: %v", err)
	}
	_, err := q.InviteCreate(ctx, InviteNew().UserEmailSet("nobody@example.com").CodeSet("B"))
	var refused *rt.Invalid
	if !errors.As(err, &refused) || len(refused.Fields) != 1 || refused.Fields[0].Check != "foreign_key" {
		t.Fatalf("an invite referencing no user = %v; want the foreign key refusal", err)
	}
	if err := q.UserDelete(ctx, u.ID); !errors.As(err, &refused) || refused.Fields[0].Check != "foreign_key" {
		t.Errorf("deleting a referenced user = %v; want the foreign key refusal", err)
	}
}

// TestMigrationAddsAUniqueColumnWithoutARebuild applies the fixture's
// migrations up to 0005, writes rows, and applies 0006, which adds an
// index column and a unique column (spec §migrations): each add
// is its ADD COLUMN and its CREATE INDEX, no rebuild, where SQLite's
// own ADD COLUMN ... UNIQUE is refused; the rows stay, the unique index
// holds from then on, and once the later migrations apply the database
// folds to the schema.
func TestMigrationAddsAUniqueColumnWithoutARebuild(t *testing.T) {
	ctx := context.Background()
	db, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := -1
	for i, m := range Migrations {
		if m.Name == "0006__daily_hits_label_and_token" {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the fixture has no migration 0006__daily_hits_label_and_token; update this test")
	}
	last := Migrations[at]
	want := []string{
		"ALTER TABLE daily_hits ADD COLUMN label TEXT NOT NULL DEFAULT ''",
		"CREATE INDEX idx_daily_hits_label ON daily_hits (label)",
		"ALTER TABLE daily_hits ADD COLUMN token TEXT",
		"CREATE UNIQUE INDEX idx_daily_hits_token ON daily_hits (token)",
	}
	if strings.Join(last.Up, "\n") != strings.Join(want, "\n") || last.ForeignKeysOff {
		t.Fatalf("0006 lowers to %q (bracket %v); want the adds and their indexes, no rebuild:\n%q", last.Up, last.ForeignKeysOff, want)
	}

	if _, err := rt.MigrateUp(ctx, db, Migrations[:at]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO daily_hits (day, site, hits) VALUES (1, 'a', 3), (1, 'b', 4)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE daily_hits ADD COLUMN other TEXT UNIQUE"); err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("SQLite's ADD COLUMN ... UNIQUE = %v; want it refused, which is why add lowers to an index", err)
	}
	if applied, err := rt.MigrateUp(ctx, db, Migrations[:at+1]); err != nil || len(applied) != 1 || applied[0] != last.Name {
		t.Fatalf("MigrateUp of 0006 over rows = %v, %v", applied, err)
	}

	q := New(db)
	hits, err := q.DailyHitList(ctx)
	if err != nil || len(hits) != 2 || hits[0].Label != "" || hits[0].Token.Valid || hits[1].Hits != 4 {
		t.Fatalf("the rows after 0006 = %+v, %v; want both kept, label empty, token NULL", hits, err)
	}
	if _, err := q.DailyHitUpdate(ctx, hits[0].Changeset().TokenSet(rt.Some("t-1"))); err != nil {
		t.Fatal(err)
	}
	_, err = q.DailyHitUpdate(ctx, hits[1].Changeset().TokenSet(rt.Some("t-1")))
	if inv := invalidOf(t, err, "unique", "token"); inv.StatusCode() != 409 {
		t.Errorf("the added unique column refuses a duplicate: %+v", inv)
	}

	if _, err := rt.MigrateUp(ctx, db, Migrations); err != nil {
		t.Fatalf("MigrateUp of the migrations after 0006: %v", err)
	}
	fold, err := migrate.Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	target := schemaRead(t, schemaDDL(t))
	for _, d := range migrate.Diff(target, fold) {
		t.Errorf("drift after 0006 and the migrations after it: %s\n%s", d.Reason, d.Volt)
	}
}

// TestOldLoweringFoldsEqual proves that a database built by the old
// lowering of unique, an inline UNIQUE and its sqlite_autoindex, folds
// equal to the new lowering's CREATE UNIQUE INDEX on the same key (spec
// §the-fold): the fold compares indexes by key, uniqueness and
// predicate, never by name, so such a database needs no migration to
// stay equal; dropping that uniqueness later takes a rebuild there.
func TestOldLoweringFoldsEqual(t *testing.T) {
	ddl := schemaDDL(t)
	single := regexp.MustCompile(`(?m)^CREATE UNIQUE INDEX \w+ ON (\w+) \((\w+)\);\n`)
	old := ddl
	moved := 0
	for _, m := range single.FindAllStringSubmatch(ddl, -1) {
		table, column := m[1], m[2]
		old = strings.Replace(old, m[0], "", 1)
		start := strings.Index(old, "CREATE TABLE "+table+" (\n")
		end := start + strings.Index(old[start:], "\n);")
		if start < 0 || end < start {
			t.Fatalf("no CREATE TABLE %s in the generated DDL", table)
		}
		line := regexp.MustCompile(`(?m)^(  ` + column + ` [^,\n]*)(,?)$`)
		block := line.ReplaceAllString(old[start:end], "${1} UNIQUE${2}")
		if block == old[start:end] {
			t.Fatalf("no column %s in CREATE TABLE %s", column, table)
		}
		old = old[:start] + block + old[end:]
		moved++
	}
	if moved < 5 || strings.Contains(old, "CREATE UNIQUE INDEX") {
		t.Fatalf("moved %d unique indexes inline; want every single-column one:\n%s", moved, old)
	}

	oldDB := ddlOpen(t, old)
	var autoindexes int
	if err := oldDB.QueryRow(uniqueConstraintsSQL).Scan(&autoindexes); err != nil {
		t.Fatal(err)
	}
	if autoindexes != moved {
		t.Fatalf("the old lowering has %d indexes of a UNIQUE constraint (sqlite_autoindex), want %d", autoindexes, moved)
	}
	oldSchema, err := migrate.Read(context.Background(), oldDB)
	if err != nil {
		t.Fatal(err)
	}
	newSchema := schemaRead(t, ddl)
	for _, d := range migrate.Diff(newSchema, oldSchema) {
		t.Errorf("an old-lowering database drifts from the schema: %s\n%s", d.Reason, d.Volt)
	}
	for _, d := range migrate.Diff(oldSchema, newSchema) {
		t.Errorf("the new lowering drifts from an old-lowering schema: %s\n%s", d.Reason, d.Volt)
	}
}

func schemaDDL(t *testing.T) string {
	t.Helper()
	ddl, err := os.ReadFile("nao_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(ddl)
}

func ddlOpen(t *testing.T, ddl string) *sql.DB {
	t.Helper()
	db, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("DDL rejected: %v\n%s", err, ddl)
	}
	return db
}

func schemaRead(t *testing.T, ddl string) migrate.Schema {
	t.Helper()
	s, err := migrate.Read(context.Background(), ddlOpen(t, ddl))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
