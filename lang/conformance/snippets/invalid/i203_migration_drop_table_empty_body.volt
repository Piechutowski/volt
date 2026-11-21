// spec/migrations, the body of a table_drop declares at least one column
// want: table_drop "audit": the body must declare at least one column
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_drop audit {
  }
}
