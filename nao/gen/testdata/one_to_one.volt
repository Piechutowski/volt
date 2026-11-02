// one-to-one: profile row per user, FK is also the unique key
table users {
  id int [pk]
  email string [unique]
}
table user_profiles {
  user_id int [pk, note: "Also the FK: one profile per user"]
  bio string [null]
  avatar_url string [null]
}
ref: user_profiles.user_id > users.id
