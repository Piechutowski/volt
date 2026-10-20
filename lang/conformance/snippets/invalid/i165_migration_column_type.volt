// spec: §migrations, a column type in a migration is one of the six built-ins, never an enum or a declared type
// want: is not a built-in
package db

Enum status {
  active
  banned
}

Table users {
  id    int    [pk]
  state status [not null]
}

Migration 0001__init {
  create table users {
    id    int    [pk]
    state status [not null]
  }
}
