// spec: §predicates, a not over an exists term all of whose terms are taken out is taken out with it
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select untagged for posts where not exists tags (tag = :tag?)
