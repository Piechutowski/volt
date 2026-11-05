// spec: §columns, §relationships-ref, §records-sample-data, a built-in int key that is also a foreign key, by a ref element or an inline ref:, is the rowid alias in the DDL but no database-assigned key: its value comes from the referenced row, so required is allowed on it and records list it
table users {
  id    int    [pk]
  email string

  records (email) {
    "a@example.com"
  }
}

table user_profiles {
  user_id int    [pk, required]
  bio     string

  records (user_id, bio) {
    1, "first"
  }
}

table user_settings {
  user_id int    [pk, ref: users.id]
  theme   string
}

records user_settings(user_id, theme) {
  1, "dark"
}

ref: user_profiles.user_id > users.id
