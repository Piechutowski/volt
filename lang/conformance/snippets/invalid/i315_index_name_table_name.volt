// spec: §indexes, §namespaces-and-uniqueness, indexes and tables share SQLite's namespace: an index takes no table's name, derived or written, compared without the case of ASCII letters
// want: the index setting of column "title" of table "posts" is named "idx_posts_title", the name of table "idx_posts_title"
table posts {
  id    int    [pk]
  title string [index]
  body  string

  indexes {
    `lower(body)` [name: "Pages"]
  }
}

table idx_posts_title {
  id int [pk]
}

table pages {
  id int [pk]
}
