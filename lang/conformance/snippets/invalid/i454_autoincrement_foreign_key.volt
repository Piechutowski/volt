// spec: §columns, autoincrement is an error on a built-in int key that is also a foreign key, by a ref element or an inline ref:, since its values come from the referenced row
// want: its values come from the referenced row
table users {
  id int [pk]
}

table user_profiles {
  user_id int    [pk, autoincrement]
  bio     string
}

table user_settings {
  user_id int    [pk, autoincrement, ref: users.id]
  theme   string
}

ref: user_profiles.user_id > users.id
