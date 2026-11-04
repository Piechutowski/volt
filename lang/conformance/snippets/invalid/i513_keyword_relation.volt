// spec: §keywords, §predicates, a predicate keyword names no relation inside a predicate, after exists or before a dot alike
// want: "like" is a keyword inside a predicate and names no relation there
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: like]

select tagged for posts where exists like (tag = "go")
select joined (id, like.tag) for posts join like where like.tag = "go"
