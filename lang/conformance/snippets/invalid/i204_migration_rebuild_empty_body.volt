// spec/migrations, the body of a table_rebuild declares at least one column
// want: table_rebuild "users": the body must declare at least one column
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id int [pk]
  }
}

migration 0002__empty [irreversible] {
  table_rebuild users {
  }
}
