// Inline one-to-one: the declaring column is the foreign key
// (spec §relationships-ref rule 3) and takes UNIQUE unless an index
// of the table makes it unique already (§mapping-to-sqlite-ddl);
// the referenced table carries nothing of it.
Table users {
  id    int    [pk, increment]
  email string [not null, unique]
}

Table profiles {
  id      int    [pk, increment]
  user_id int    [not null, ref: - users.id]
  bio     string
}

Table avatars {
  id      int    [pk, increment]
  user_id int    [not null, ref: - users.id]
  url     string [not null]

  indexes {
    user_id [unique]
  }
}
