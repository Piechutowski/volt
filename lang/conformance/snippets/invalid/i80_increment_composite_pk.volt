// spec: §columns increment extension — no rowid alias under a composite key
// want: composite primary key
Table user_tags {
  user_id int [increment]
  tag string

  indexes {
    (user_id, tag) [pk]
  }
}
