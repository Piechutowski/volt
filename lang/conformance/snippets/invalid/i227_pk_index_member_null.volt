// spec: §columns, §indexes, a member of a [pk] index never carries null
// want: is a member of the [pk] index and carries null
Table post_tags {
  post_id int
  tag     string [null]

  indexes {
    (post_id, tag) [pk]
  }
}
