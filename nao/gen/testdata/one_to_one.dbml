// one-to-one: profile row per user, FK is also the unique key
Table users {
  id int [pk]
  email string [not null, unique]
}
Table user_profiles {
  user_id int [pk, note: 'Also the FK: one profile per user']
  bio string
  avatar_url string
}
Ref: users.id - user_profiles.user_id
