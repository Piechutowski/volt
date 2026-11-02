// spec: §migrations, an exec takes down: and no other setting
// want: unknown setting "note"
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  exec `PRAGMA user_version = 1` [down: `PRAGMA user_version = 0`, note: "bump"]
}
