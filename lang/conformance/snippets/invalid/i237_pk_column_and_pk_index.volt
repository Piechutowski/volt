// spec: §columns, §indexes, a table has one primary key: a pk column beside a [pk] index is an error
// want: declares a second primary key
Table post_tags {
  post_id int [pk]
  tag     string

  Indexes {
    (post_id, tag) [pk]
  }
}
