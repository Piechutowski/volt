// spec: §records-sample-data, §columns, a records list names a built-in int key that is also a foreign key, in the table or at the top level, since its value comes from the referenced row and not from the database
// want: the rowid alias and a foreign key
table users {
  id int [pk]
}

table user_profiles {
  user_id int    [pk]
  bio     string

  records (bio) {
    "first"
  }
}

table user_settings {
  user_id int    [pk, ref: users.id]
  theme   string
}

records user_settings(theme) {
  "dark"
}

ref: user_profiles.user_id > users.id
