// spec: §indexes, §columns, pk is a column setting only: an index carrying pk is an error naming the column form, a composite key among them
// want: pk is a column setting, not an index setting: write pk on each key column
table post_tags {
  post_id int
  tag     string

  indexes {
    (post_id, tag) [pk]
  }
}
