// A one-to-one is a unique foreign key (spec §relationships-ref): the
// declaring column is the foreign key, profiles.user_id carries unique
// beside its ref and avatars.user_id has a unique index; the ref adds
// nothing, and the referenced table carries nothing of it
// (§mapping-to-sqlite-ddl).
Table users {
  id    int    [pk, increment]
  email string [unique]
}

Table profiles {
  id      int    [pk, increment]
  user_id int    [unique, ref: users.id]
  bio     string [null]
}

Table avatars {
  id      int    [pk, increment]
  user_id int    [unique, ref: users.id]
  url     string
}
