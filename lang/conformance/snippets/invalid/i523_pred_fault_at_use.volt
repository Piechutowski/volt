// spec: §predicates, a pred is typed at each use site, so a fault it has over one select is reported where that select reads it, naming the select
// want: select quiet reads pred tagged here, where at
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk, ref: posts.id]
  tag     string [pk]
}

table comments {
  id      int [pk]
  post_id int [ref: posts.id]
}

pred tagged { exists post_tags (tag = :tag?) }

select listed for posts where tagged [order: (id asc)]
select quiet for comments where tagged [order: (id asc)]
