// spec: §migrations, note: is a setting of no migration position
// want: note: documents the schema
package db

Table users {
  id   int [pk]
  name string [null]
}

Migration 0001__init [note: "the first"] {
  create table users {
    id   int    [pk]
    name string [null, note: "display name"]

    Indexes {
      (name, id) [note: "by name"]
    }
  }
}
