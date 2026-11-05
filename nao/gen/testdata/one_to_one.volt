// one-to-one: profile row per user, FK is also the unique key

table users {
  id int [pk]
  email string [unique]
}
table user_profiles {
  // Also the FK: one profile per user
  user_id int [pk]
  bio string [null]
  avatar_url string [null]
}
ref: user_profiles.user_id > users.id
