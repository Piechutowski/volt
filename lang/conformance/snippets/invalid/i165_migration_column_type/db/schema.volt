// spec: §migrations, a column type in a migration is one of the six built-ins, never a declared type
// want: is not a built-in
package db

type State string

table users {
  id    int   [pk]
  state State

  validators {
    StateKnown(state)
  }
}

migration 0001__init {
  create table users {
    id    int   [pk]
    state State
  }
}
