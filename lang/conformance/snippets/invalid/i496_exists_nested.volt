// spec: §predicates, exists does not nest: the depth is one
// want: exists does not nest, the depth is one
package db

table members {
  id   int    [pk]
  name string
}

table posts {
  id        int [pk]
  author_id int [ref: members.id]
}

table comments {
  id      int [pk]
  post_id int [ref: posts.id]
}

select busy for members where exists posts (exists comments (id > 0))
