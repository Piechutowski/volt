// spec/migrations, from: is a setting of a table_rebuild column only
// want: from: names the source of a rebuilt column
package db

table users {
  id   int [pk]
  hits int [default: 0]
}

migration 0001__init {
  table_create users {
    id   int [pk]
    hits int [default: 0, from: `0`]
  }
}
