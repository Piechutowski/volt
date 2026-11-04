// spec: §predicates, exists names a relation of the select's table; any other name is an error listing its relations
// want: table "posts" has no relation "labels" for exists to reach; its relations: tags
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select labeled for posts where exists labels (tag = "go")
