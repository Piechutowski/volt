// spec: §migrations, a column type in a migration is one of the six built-ins, never a declared type
// want: is not a built-in
package db

Type State string

Table users {
  id    int   [pk]
  state State

  Validators {
    StateKnown(state)
  }
}

Migration 0001__init {
  create table users {
    id    int   [pk]
    state State
  }
}
