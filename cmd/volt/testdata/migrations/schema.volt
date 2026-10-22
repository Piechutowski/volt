// The migration CLI fixture: a blog whose two migrations fold to this
// schema. The tests copy the directory before writing into it.
package blog

Table posts {
  id    int    [pk, increment]
  title string
  body  string [null]
  views int    [default: 0]

  Indexes {
    views
  }
}

Table comments {
  id      int    [pk, increment]
  post_id int    [ref: posts.id]
  body    string
}
