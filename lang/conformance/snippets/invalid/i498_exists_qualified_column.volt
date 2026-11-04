// spec: §predicates, inside exists a column is the related table's, named bare; a qualified name is an error
// want: inside exists tags a column is one of table "post_tags"'s, named bare; tags.tag is qualified
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select labeled for posts where exists tags (tags.tag = "go")
