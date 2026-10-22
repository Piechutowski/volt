// A Ref's delete: and update: become the foreign key's ON DELETE and
// ON UPDATE clauses (spec §relationships-ref): set_null is SET NULL
// and set_default is SET DEFAULT, two SQL words for one Volt word;
// cascade and restrict are spelled as they are (§mapping-to-sqlite-ddl).
Table users {
  id    int    [pk, increment]
  email string [unique]
}

Table posts {
  id        int [pk, increment]
  author_id int [null, default: 1]
  owner_id  int
}

Ref: posts.author_id > users.id [delete: set_null, update: set_default]
Ref: posts.owner_id > users.id [delete: cascade, update: restrict]
