// The migration CLI fixture: a blog whose two migrations fold to this
// schema. The tests copy the directory before writing into it.
package blog

Table posts {
  id    int    [pk, increment]
  title string [not null]
  body  string
  views int    [not null, default: 0]

  indexes {
    views
  }
}

Table comments {
  id      int    [pk, increment]
  post_id int    [not null, ref: > posts.id]
  body    string [not null]
}
