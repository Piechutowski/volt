// spec/migrations, a table_create body declares at least one column
// want: must declare at least one column
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
  }
}
