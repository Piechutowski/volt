package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"
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

func TestDiff(t *testing.T) {
	cases := []struct {
		name         string
		fold, target string
		want         []string // the Volt text of every drift, in order
		reason       string   // a substring of the first drift's reason
	}{
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
			name:   "a table only in the schema, columns only in the schema",
			fold:   "CREATE TABLE posts (id INTEGER PRIMARY KEY);",
			target: postsDDL,
			want: []string{
				"create table comments {\n  id int [pk, increment]\n  post_id int [not null, ref: > posts.id]\n  body string\n}",
				"alter table posts {\n  add slug string [not null, unique]\n}",
				"alter table posts {\n  add title string [not null]\n}",
				"alter table posts {\n  add views int [not null, default: 0]\n}",
				"create index posts views [where: `views > 0`]",
			},
			reason: "table comments is in the schema but not in the fold",
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
			want:   []string{"alter table comments {\n  add flagged int [not null, default: 0]\n}"},
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
			name:   "a type change is a rebuild",
			fold:   strings.Replace(postsDDL, "views INTEGER NOT NULL DEFAULT 0", "views TEXT NOT NULL DEFAULT '0'", 1),
			target: postsDDL,
			want: []string{
				"rebuild table posts {\n  id int [pk, increment]\n  slug string [not null, unique]\n  title string [not null]\n  views int [not null, default: 0]\n\n  indexes {\n    views [where: `views > 0`]\n  }\n}",
			},
			reason: "column posts.views is INTEGER in the schema, TEXT in the fold; column posts.views defaults to 0 in the schema, '0' in the fold",
		},
		{
			name:   "a nullability change is a rebuild",
			fold:   strings.Replace(postsDDL, "body TEXT,", "body TEXT NOT NULL,", 1),
			target: postsDDL,
			want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [not null, ref: > posts.id]\n  body string\n}"},
			reason: "column comments.body is nullable in the schema, not null in the fold",
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
			want:   []string{"rebuild table comments {\n  id int [pk, increment]\n  post_id int [not null, ref: > posts.id]\n  body string\n}"},
			reason: "foreign key comments (post_id) references posts (id) is in the schema but not in the fold",
		},
		{
			name:   "a unique constraint only in the fold is a rebuild",
			fold:   strings.Replace(postsDDL, "title TEXT NOT NULL", "title TEXT NOT NULL UNIQUE", 1),
			target: postsDDL,
			want: []string{
				"rebuild table posts {\n  id int [pk, increment]\n  slug string [not null, unique]\n  title string [not null]\n  views int [not null, default: 0]\n\n  indexes {\n    views [where: `views > 0`]\n  }\n}",
			},
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
				"create table user_tags {\n  user_id int [not null]\n  tag string [not null]\n  since time.Time [not null, default: `CURRENT_TIMESTAMP`]\n  label string [default: 'it\\'s']\n\n  indexes {\n    (user_id, tag) [pk]\n    (tag, user_id) [unique]\n  }\n}",
			},
			reason: "table a is in the fold but not in the schema",
		},
	}
	ctx := context.Background()
	for _, tc := range cases {
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
			var got []string
			for _, d := range drifts {
				got = append(got, d.Volt)
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
			for _, d := range drifts {
				if d.Table == "" || d.Reason == "" {
					t.Errorf("drift without a table or a reason: %+v", d)
				}
			}
		})
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
