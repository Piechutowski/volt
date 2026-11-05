// spec: §predicates, inside exists a bare name is a column of the relation's table, never the outer table's
// want: table "post_tags", which exists tags reaches, has no column "title"
package db

table posts {
  id    int    [pk]
  title string
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select labeled for posts where exists tags (title = "go")
