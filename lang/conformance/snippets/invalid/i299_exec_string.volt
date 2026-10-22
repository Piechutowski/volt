// spec: §migrations, §expression-literals, exec takes its SQL in backticks, an expression literal, and so does its down:
// want: exec takes its SQL in backticks
package db

Table users {
  id int [pk, increment]
}

Migration 0001__init [irreversible] {
  create table users {
    id int [pk, increment]
  }
  exec "PRAGMA user_version = 1"
}
