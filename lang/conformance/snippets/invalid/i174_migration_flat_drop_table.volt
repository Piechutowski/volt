// spec/migrations, a flat body with a table_drop without a body has no inverse
// want: table_drop audit has no body
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_drop audit
}
