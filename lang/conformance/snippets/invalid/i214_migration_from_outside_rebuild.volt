// spec: §migrations, from: is a setting of a rebuild table column only
// want: from: names the source of a rebuilt column
package db

table users {
  id   int [pk]
  hits int [default: 0]
}

migration 0001__init {
  create table users {
    id   int [pk]
    hits int [default: 0, from: `0`]
  }
}
