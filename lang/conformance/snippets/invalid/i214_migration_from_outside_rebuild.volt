// spec: §migrations, from: is a setting of a rebuild table column only
// want: from: names the source of a rebuilt column
package db

Table users {
  id   int [pk]
  hits int [default: 0]
}

Migration 0001__init {
  create table users {
    id   int [pk]
    hits int [default: 0, from: `0`]
  }
}
