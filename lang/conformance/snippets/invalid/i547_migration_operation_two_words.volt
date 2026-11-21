// spec/keywords, spec/migrations, an operation is one identifier: create table in two words is
// a syntax error naming table_create, reported once, and the operation still reads
// want: write table_create, not create table
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
}
