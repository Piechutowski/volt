// spec: §columns, no rowid alias under a composite key: autoincrement is valid only on a single int pk column
// want: composite primary key
table user_tags {
  user_id int    [pk, autoincrement]
  tag     string [pk]
}
