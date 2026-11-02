// The migration CLI fixture: a blog whose two migrations fold to this
// schema. The tests copy the directory before writing into it.
package blog

table posts {
  id    int    [pk, increment]
  title string
  body  string [null]
  views int    [index, default: 0]
}

table comments {
  id      int    [pk, increment]
  post_id int    [ref: posts.id]
  body    string
}
