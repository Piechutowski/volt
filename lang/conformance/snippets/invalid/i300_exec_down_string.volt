// spec: §migrations, §expression-literals, the down: of an exec is an expression literal, as the exec's SQL is
// want: "down" must be a backtick expression
package db

Table users {
  id int [pk, increment]
}

Migration 0001__init [irreversible] {
  create table users {
    id int [pk, increment]
  }
  exec `PRAGMA user_version = 1` [down: "PRAGMA user_version = 0"]
}
