// spec: §columns, §indexes, a table has one primary key: a second [pk] index is an error
// want: declares a second primary key
Table post_tags {
  post_id int
  tag     string

  indexes {
    (post_id, tag) [pk]
    tag [pk]
  }
}
