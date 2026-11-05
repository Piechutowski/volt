// spec: §predicates, exists rel (pred) tests a forward or a reverse relation of the target, its columns bare, in a pred and in a where, beside a join, optional parameters vanishing it whole
package db

table members {
  id   int    [pk]
  name string
}

table posts {
  id        int    [pk]
  author_id int    [ref: members.id]
  title     string
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

table comments {
  id      int    [pk]
  post_id int    [ref: posts.id]
  body    string
}

pred tagged { exists tags (tag = :tag?) }

select by_author for posts where exists author (name = :name) [order: (id asc)]
select listed for posts where tagged [order: (id asc)]
select counted (id, count(comments.id) as n) for posts join comments where exists tags (tag in :tags or tag like "go%") [group: (id), order: (id asc)]
select untagged for posts where not exists tags (tag = "draft")
select discussed for members where exists posts (title != "") and name != ""
