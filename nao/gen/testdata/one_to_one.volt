// one-to-one: profile row per user, FK is also the unique key
Table users {
  id int [pk]
  email string [unique]
}
Table user_profiles {
  user_id int [pk, note: "Also the FK: one profile per user"]
  bio string [null]
  avatar_url string [null]
}
Ref: user_profiles.user_id > users.id
