// spec: §columns increment extension, no rowid alias under a composite key: increment is valid only on a single int pk column
// want: composite primary key
Table user_tags {
  user_id int    [pk, increment]
  tag     string [pk]
}
