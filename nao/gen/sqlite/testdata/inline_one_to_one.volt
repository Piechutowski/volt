// Inline one-to-one: the declaring column is the foreign key
// (spec §relationships-ref rule 3) and takes UNIQUE unless an index
// of the table makes it unique already (§mapping-to-sqlite-ddl);
// the referenced table carries nothing of it.
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
  user_id int    [ref: users.id]
  url     string

  indexes {
    user_id [unique]
  }
}
