// spec: §predicates, exists needs a table target, since a group has no relations
// want: exists needs a table: "content" is a group, and a group has no relations
package db

table posts {
  id int [pk]
}

table pages {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

group content = posts + pages

select labeled for content where exists tags (tag = "go")
