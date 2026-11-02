// spec: §migrations, note: is a setting of no migration position
// want: note: documents the schema
package db

table users {
  id   int [pk]
  name string [null]
}

migration 0001__init [note: "the first"] {
  create table users {
    id   int    [pk]
    name string [null, note: "display name"]

    indexes {
      (name, id) [note: "by name"]
    }
  }
}
