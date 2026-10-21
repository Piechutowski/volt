package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/rt"
)

func ddlDB(t *testing.T, ddl string) *sql.DB {
	t.Helper()
	db := memoryDB(t)
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("DDL rejected: %v\n%s", err, ddl)
	}
	return db
}

const postsDDL = `
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  views INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_posts_views ON posts (views) WHERE views > 0;
CREATE TABLE comments (
  id INTEGER PRIMARY KEY,
  post_id INTEGER NOT NULL,
  body TEXT,
  FOREIGN KEY (post_id) REFERENCES posts (id)
);
`

const rebuildPosts = "rebuild table posts {\n  id int [pk, increment]\n  slug string [unique]\n  title string\n  views int [default: 0]\n\n  indexes {\n    views [where: `views > 0`]\n  }\n}"

type diffCase struct {
	name         string
	fold, target string
	want         []string // the Volt text of every drift, in order
	reason       string   // a substring of the first drift's reason
	also         string   // a substring of some drift's reason
	unclosed     string   // why TestDriftCloses leaves the case out
}

var diffCases = []diffCase{
	{
		name: "identical up to column order and the ledger",
		fold: postsDDL + "CREATE TABLE volt_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL);",
		target: `
CREATE TABLE comments (
  id INTEGER PRIMARY KEY,
  body TEXT,
  post_id INTEGER NOT NULL,
  FOREIGN KEY (post_id) REFERENCES posts (id)
);
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  views INTEGER NOT NULL DEFAULT 0,
  title TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE
);
CREATE INDEX idx_posts_views ON posts (views) WHERE views > 0;
`,
	},
	{
		name:   "a table only in the schema, NOT NULL columns only in the schema",
		fold:   "CREATE TABLE posts (id INTEGER PRIMARY KEY);",
		target: postsDDL,
		want: []string{
			"create table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id]\n  body string [null]\n}",
			rebuildPosts,
		},
		reason:   "table comments is in the schema but not in the fold",
		also:     "column posts.slug is in the schema but not in the fold and is NOT NULL without a default, which add cannot fill: rebuild the table with a from: or a default: for it, or add it with a default:",
		unclosed: "rule 9 refuses the rebuild while posts.slug and posts.title, NOT NULL and new, have no from: or default:; the user supplies one",
	},
	{
		name:   "a unique constraint only in the schema on a column the fold has",
		fold:   strings.Replace(postsDDL, "slug TEXT NOT NULL UNIQUE", "slug TEXT NOT NULL", 1),
		target: postsDDL,
		want:   []string{"create index posts slug [unique]"},
		reason: "index (slug) on posts is in the schema but not in the fold",
	},
	{
		name:   "a table only in the fold",
		fold:   postsDDL + "CREATE TABLE scratch (id INTEGER PRIMARY KEY);",
		target: postsDDL,
		want:   []string{"drop table scratch"},
		reason: "table scratch is in the fold but not in the schema",
	},
	{
		name:   "a column only in the schema",
		fold:   postsDDL,
		target: strings.Replace(postsDDL, "  body TEXT,\n", "  body TEXT,\n  flagged INTEGER NOT NULL DEFAULT 0,\n", 1),
		want:   []string{"alter table comments {\n  add flagged int [default: 0]\n}"},
		reason: "column comments.flagged is in the schema but not in the fold",
	},
	{
		name:   "a column only in the fold",
		fold:   strings.Replace(postsDDL, "  body TEXT,\n", "  body TEXT,\n  draft TEXT,\n", 1),
		target: postsDDL,
		want:   []string{"alter table comments {\n  drop draft\n}"},
		reason: "column comments.draft is in the fold but not in the schema",
	},
	{
		name:   "an indexed column only in the fold loses its index first",
		fold:   strings.Replace(postsDDL, "  body TEXT,\n", "  body TEXT,\n  draft TEXT,\n", 1) + "CREATE INDEX idx_comments_draft ON comments (draft);",
		target: postsDDL,
		want:   []string{"drop index comments draft", "alter table comments {\n  drop draft\n}"},
		reason: "index idx_comments_draft on comments is in the fold but not in the schema",
	},
	{
		name:     "a NOT NULL column without a default only in the schema is a rebuild",
		fold:     strings.Replace(postsDDL, "  title TEXT NOT NULL,\n", "", 1),
		target:   postsDDL,
		want:     []string{rebuildPosts},
		reason:   "column posts.title is in the schema but not in the fold and is NOT NULL without a default, which add cannot fill",
		unclosed: "rule 9 refuses the rebuild while posts.title, NOT NULL and new, has no from: or default:; the user supplies one",
	},
	{
		name:   "a column with an expression default only in the schema is a rebuild",
		fold:   postsDDL,
		target: strings.Replace(postsDDL, "  body TEXT,\n", "  body TEXT,\n  seen TIMESTAMP DEFAULT (CURRENT_TIMESTAMP),\n", 1),
		want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id]\n  body string [null]\n  seen time.Time [null, default: `CURRENT_TIMESTAMP`]\n}"},
		reason: "column comments.seen is in the schema but not in the fold and has an expression default, which add cannot take",
	},
	{
		name:   "a nullable foreign key only in the schema is an add",
		fold:   strings.Replace(strings.Replace(postsDDL, "  post_id INTEGER NOT NULL,\n", "", 1), "  body TEXT,\n  FOREIGN KEY (post_id) REFERENCES posts (id)\n", "  body TEXT\n", 1),
		target: strings.Replace(postsDDL, "post_id INTEGER NOT NULL", "post_id INTEGER", 1),
		want:   []string{"alter table comments {\n  add post_id int [null, ref: > posts.id]\n}"},
		reason: "column comments.post_id is in the schema but not in the fold",
	},
	{
		name:     "a NOT NULL foreign key only in the schema is a rebuild",
		fold:     strings.Replace(strings.Replace(postsDDL, "  post_id INTEGER NOT NULL,\n", "", 1), "  body TEXT,\n  FOREIGN KEY (post_id) REFERENCES posts (id)\n", "  body TEXT\n", 1),
		target:   postsDDL,
		want:     []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id]\n  body string [null]\n}"},
		reason:   "foreign key comments (post_id) references posts (id) is in the schema but not in the fold",
		also:     "column comments.post_id is in the schema but not in the fold and is NOT NULL without a default, which add cannot fill",
		unclosed: "rule 9 refuses the rebuild while comments.post_id, NOT NULL and new, has no from: or default:; the user supplies one",
	},
	{
		name:   "a foreign key with a default only in the schema is a rebuild",
		fold:   strings.Replace(strings.Replace(postsDDL, "  post_id INTEGER NOT NULL,\n", "", 1), "  body TEXT,\n  FOREIGN KEY (post_id) REFERENCES posts (id)\n", "  body TEXT\n", 1),
		target: strings.Replace(postsDDL, "post_id INTEGER NOT NULL", "post_id INTEGER DEFAULT 0", 1),
		want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [null, default: 0, ref: > posts.id]\n  body string [null]\n}"},
		reason: "foreign key comments (post_id) references posts (id) is in the schema but not in the fold",
		also:   "column comments.post_id is in the schema but not in the fold and is a foreign key with a default, which add cannot make",
	},
	{
		name:   "a type change is a rebuild",
		fold:   strings.Replace(postsDDL, "views INTEGER NOT NULL DEFAULT 0", "views TEXT NOT NULL DEFAULT '0'", 1),
		target: postsDDL,
		want:   []string{rebuildPosts},
		reason: "column posts.views is INTEGER in the schema, TEXT in the fold; column posts.views defaults to 0 in the schema, '0' in the fold",
	},
	{
		name:   "a nullability change is a rebuild",
		fold:   strings.Replace(postsDDL, "body TEXT,", "body TEXT NOT NULL,", 1),
		target: postsDDL,
		want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id]\n  body string [null]\n}"},
		reason: "column comments.body is nullable in the schema, NOT NULL in the fold",
	},
	{
		name:   "a unique constraint and a unique index are one structure",
		fold:   strings.Replace(postsDDL, "slug TEXT NOT NULL UNIQUE", "slug TEXT NOT NULL", 1) + "CREATE UNIQUE INDEX idx_posts_slug ON posts (slug);",
		target: postsDDL,
	},
	{
		name:   "an index only in the schema",
		fold:   strings.Replace(postsDDL, "CREATE INDEX idx_posts_views ON posts (views) WHERE views > 0;\n", "", 1),
		target: postsDDL,
		want:   []string{"create index posts views [where: `views > 0`]"},
		reason: "index idx_posts_views on posts is in the schema but not in the fold",
	},
	{
		name:   "an index only in the fold",
		fold:   postsDDL + "CREATE INDEX idx_comments_post_id_body ON comments (post_id, body);",
		target: postsDDL,
		want:   []string{"drop index comments (post_id, body)"},
		reason: "index idx_comments_post_id_body on comments is in the fold but not in the schema",
	},
	{
		name:   "a changed index is a drop and a create",
		fold:   strings.Replace(postsDDL, "WHERE views > 0", "WHERE views > 1", 1),
		target: postsDDL,
		want:   []string{"drop index posts views [where: `views > 1`]", "create index posts views [where: `views > 0`]"},
		reason: "index idx_posts_views on posts is in the fold but not in the schema",
	},
	{
		name:   "an expression index with its name",
		fold:   postsDDL,
		target: postsDDL + "CREATE UNIQUE INDEX idx_posts_title_ci ON posts ((lower(title)));",
		want:   []string{"create index posts (`lower(title)`) [unique, name: 'idx_posts_title_ci']"},
		reason: "index idx_posts_title_ci on posts is in the schema but not in the fold",
	},
	{
		name:   "a foreign key change is a rebuild",
		fold:   strings.Replace(postsDDL, "REFERENCES posts (id)", "REFERENCES posts (id) ON DELETE CASCADE", 1),
		target: postsDDL,
		want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id]\n  body string [null]\n}"},
		reason: "foreign key comments (post_id) references posts (id) is in the schema but not in the fold; foreign key comments (post_id) references posts (id) [delete: cascade] is in the fold but not in the schema",
	},
	{
		name:   "referential actions read back as delete: and update:",
		fold:   postsDDL,
		target: strings.Replace(postsDDL, "REFERENCES posts (id)", "REFERENCES posts (id) ON DELETE CASCADE ON UPDATE SET NULL", 1),
		want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [ref: > posts.id, delete: cascade, update: set null]\n  body string [null]\n}"},
		reason: "foreign key comments (post_id) references posts (id) [delete: cascade, update: set null] is in the schema but not in the fold",
	},
	{
		name:   "a unique constraint only in the fold is a rebuild",
		fold:   strings.Replace(postsDDL, "title TEXT NOT NULL", "title TEXT NOT NULL UNIQUE", 1),
		target: postsDDL,
		want:   []string{rebuildPosts},
		reason: "unique constraint on posts (title) is in the fold but not in the schema",
	},
	{
		name: "a composite key and an expression default read back",
		fold: "CREATE TABLE a (id INTEGER PRIMARY KEY);",
		target: `
CREATE TABLE user_tags (
  user_id INTEGER NOT NULL,
  tag TEXT NOT NULL,
  since TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP),
  label TEXT DEFAULT 'it''s',
  PRIMARY KEY (user_id, tag)
);
CREATE UNIQUE INDEX idx_user_tags_tag_user_id ON user_tags (tag, user_id);
`,
		want: []string{
			"drop table a",
			"create table user_tags {\n  user_id int\n  tag string\n  since time.Time [default: `CURRENT_TIMESTAMP`]\n  label string [null, default: 'it\\'s']\n\n  indexes {\n    (user_id, tag) [pk]\n    (tag, user_id) [unique]\n  }\n}",
		},
		reason: "table a is in the fold but not in the schema",
	},
	{
		name: "a composite foreign key is written with exec",
		fold: "CREATE TABLE periods (merchant INTEGER NOT NULL, country TEXT NOT NULL, PRIMARY KEY (merchant, country));",
		target: `
CREATE TABLE periods (merchant INTEGER NOT NULL, country TEXT NOT NULL, PRIMARY KEY (merchant, country));
CREATE TABLE invoices (
  id INTEGER PRIMARY KEY,
  merchant INTEGER,
  country TEXT,
  FOREIGN KEY (merchant, country) REFERENCES periods (merchant, country) ON DELETE CASCADE
);
`,
		want: []string{
			"create table invoices {\n  id int [pk, increment]\n  merchant int [null]\n  country string [null]\n\n  indexes {\n    // foreign key invoices (merchant, country) references periods (merchant, country) [delete: cascade] has no shape item: write the table's CREATE TABLE with exec\n  }\n}",
		},
		reason:   "table invoices is in the schema but not in the fold; foreign key invoices (merchant, country) references periods (merchant, country) [delete: cascade] is over several columns, which no shape body holds: write the CREATE TABLE of invoices with exec",
		unclosed: "a shape body holds no composite foreign key; the user writes the CREATE TABLE with exec",
	},
}

func TestDiff(t *testing.T) {
	ctx := context.Background()
	for _, tc := range diffCases {
		t.Run(tc.name, func(t *testing.T) {
			fold, err := Read(ctx, ddlDB(t, tc.fold))
			if err != nil {
				t.Fatal(err)
			}
			target, err := Read(ctx, ddlDB(t, tc.target))
			if err != nil {
				t.Fatal(err)
			}
			drifts := Diff(target, fold)
			var got, reasons []string
			for _, d := range drifts {
				got = append(got, d.Volt)
				reasons = append(reasons, d.Reason)
			}
			if strings.Join(got, "\n---\n") != strings.Join(tc.want, "\n---\n") {
				t.Errorf("drifts:\n%s\nwant:\n%s", strings.Join(got, "\n---\n"), strings.Join(tc.want, "\n---\n"))
			}
			if tc.reason != "" && (len(drifts) == 0 || !strings.Contains(drifts[0].Reason, tc.reason)) {
				var first string
				if len(drifts) > 0 {
					first = drifts[0].Reason
				}
				t.Errorf("first reason %q lacks %q", first, tc.reason)
			}
			if tc.also != "" && !strings.Contains(strings.Join(reasons, "\n"), tc.also) {
				t.Errorf("reasons %q lack %q", reasons, tc.also)
			}
			for _, d := range drifts {
				if d.Table == "" || d.Reason == "" {
					t.Errorf("drift without a table or a reason: %+v", d)
				}
			}
		})
	}
}

// TestDriftCloses is the promise of the drift report (spec §the-fold
// rule 5): its operations, written into one migration after the
// migration that made the fold, pass the checker, lower, run and leave
// no drift behind, in the order reported. The fold's migration is its
// tables as shapeVolt renders them, so it also proves the rendering
// reads back as the structure it came from; the pair stands beside
// one schema table, since a migration is an element of a package that
// declares tables.
func TestDriftCloses(t *testing.T) {
	ctx := context.Background()
	for _, tc := range diffCases {
		if len(tc.want) == 0 || tc.unclosed != "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			fold, err := Read(ctx, ddlDB(t, tc.fold))
			if err != nil {
				t.Fatal(err)
			}
			target, err := Read(ctx, ddlDB(t, tc.target))
			if err != nil {
				t.Fatal(err)
			}
			var made, ops []string
			for i := range fold.Tables {
				made = append(made, operationIndent("create table "+fold.Tables[i].Name+" {\n"+shapeVolt(&fold.Tables[i])+"}"))
			}
			for _, d := range Diff(target, fold) {
				ops = append(ops, operationIndent(d.Volt))
			}
			closing := "Migration 0001__close [irreversible] {\n" + strings.Join(ops, "\n") + "\n}\n"
			src := "package db\n\nTable keep {\n  id int [pk]\n}\n\nMigration 0000__fold {\n" + strings.Join(made, "\n") + "\n}\n\n" + closing
			_, info := parseChecked(t, "close.volt", src)
			ms, err := Lower(info)
			if err != nil {
				t.Fatalf("%v\n%s", err, src)
			}
			db := memoryDB(t)
			if _, err := rt.MigrateUp(ctx, db, ms[:1]); err != nil {
				t.Fatalf("%v\n%s", err, src)
			}
			remade, err := Read(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range Diff(fold, remade) {
				t.Errorf("the fold rendered and remade differs: %s\n%s", d.Reason, d.Volt)
			}
			if _, err := rt.MigrateUp(ctx, db, ms); err != nil {
				t.Fatalf("%v\n%s", err, src)
			}
			after, err := Read(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range Diff(target, after) {
				t.Errorf("drift left after the report ran: %s\n%s", d.Reason, d.Volt)
			}
		})
	}
}

func operationIndent(op string) string {
	return "  " + strings.ReplaceAll(op, "\n", "\n  ")
}

// TestForeignKeyActionsRoundTrip takes the rebuild body the drift
// renders for a foreign key with referential actions through TableDDL
// and back through Read: the same key with the same actions.
func TestForeignKeyActionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	const withActions = "REFERENCES posts (id) ON DELETE CASCADE ON UPDATE SET NULL"
	target, err := Read(ctx, ddlDB(t, strings.Replace(postsDDL, "REFERENCES posts (id)", withActions, 1)))
	if err != nil {
		t.Fatal(err)
	}
	fold, err := Read(ctx, ddlDB(t, postsDDL))
	if err != nil {
		t.Fatal(err)
	}
	drifts := Diff(target, fold)
	if len(drifts) != 1 || !strings.HasPrefix(drifts[0].Volt, "rebuild table comments {") {
		t.Fatalf("drifts %+v, want one rebuild of comments", drifts)
	}
	f, diags := parser.ParseFile("t.volt", "package db\n\nMigration 0001__actions {\n  "+strings.ReplaceAll(drifts[0].Volt, "\n", "\n  ")+"\n}\n")
	if len(diags) > 0 {
		t.Fatalf("the drift does not parse: %v\n%s", diags, drifts[0].Volt)
	}
	var body []ast.TableItem
	for _, d := range f.Decls {
		if m, ok := d.(*ast.Migration); ok {
			body = m.Up[0].(*ast.RebuildTable).Body
		}
	}
	ddl, err := sqlite.TableDDL("comments", body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "FOREIGN KEY (post_id) "+withActions) {
		t.Errorf("TableDDL of the drift lacks the actions:\n%s", ddl)
	}
	db := ddlDB(t, strings.Replace(postsDDL, "CREATE TABLE comments", "CREATE TABLE comments_old", 1)+ddl)
	back, err := Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := back.Table("comments").ForeignKeys, target.Table("comments").ForeignKeys; len(got) != 1 || !foreignKeyEqual(got[0], want[0]) {
		t.Errorf("foreign keys read back %+v, want %+v", got, want)
	}
}

func TestReadLeavesOutSQLiteObjects(t *testing.T) {
	ctx := context.Background()
	db := ddlDB(t, "CREATE TABLE t (id INTEGER PRIMARY KEY AUTOINCREMENT, n TEXT UNIQUE);")
	s, err := Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tables) != 1 || s.Tables[0].Name != "t" {
		t.Fatalf("tables %+v, want t alone (no sqlite_sequence)", s.Tables)
	}
	ix := s.Tables[0].Indexes
	if len(ix) != 1 || ix[0].Name != "" || !ix[0].Unique || strings.Join(ix[0].Key, ",") != "n" {
		t.Errorf("indexes %+v, want one unnamed unique on n", ix)
	}
}

func TestIndexSQLParse(t *testing.T) {
	cases := []struct {
		sql   string
		key   []string
		where string
	}{
		{"CREATE INDEX i ON t (a, b)", []string{"a", "b"}, ""},
		{`CREATE UNIQUE INDEX "i" ON "t" ("a") WHERE a > 0`, []string{"a"}, "a > 0"},
		{"CREATE INDEX i ON t ((lower(title)), b)", []string{"lower(title)", "b"}, ""},
		{"create index i on t (a) where x = 'y, (z)'", []string{"a"}, "x = 'y, (z)'"},
		{"CREATE INDEX i ON t ((a  +\n b))", []string{"a + b"}, ""},
	}
	for _, tc := range cases {
		key, where := indexSQLParse(tc.sql)
		if strings.Join(key, "|") != strings.Join(tc.key, "|") || where != tc.where {
			t.Errorf("%s: key %q where %q, want %q %q", tc.sql, key, where, tc.key, tc.where)
		}
	}
}
