// spec: §indexes, index names compare as SQLite compares them, without the case of ASCII letters: two names that differ only in case are one name
// want: table "posts" has two indexes named "IDX_Posts" and "idx_posts" (one name to SQLite)
table posts {
  id    int    [pk]
  title string
  body  string

  indexes {
    (title, body) [name: "IDX_Posts"]
    (body, title) [name: "idx_posts"]
  }
}
