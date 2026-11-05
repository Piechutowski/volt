// spec: §columns, no rowid alias under a composite key, so no database-assigned key, which alone takes autoincrement
// want: composite primary key
table user_tags {
  user_id int    [pk, autoincrement]
  tag     string [pk]
}
