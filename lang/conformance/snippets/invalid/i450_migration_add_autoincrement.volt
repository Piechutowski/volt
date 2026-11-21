// spec/migrations, autoincrement cannot be added to an existing table, since it is a table rewrite only table_rebuild makes
// want: autoincrement cannot be added to an existing table
package db

table users {
  id    int    [pk]
  email string
  seq   int    [null]
}

migration 0001__init {
  table_create users {
    id    int    [pk]
    email string
  }
}

migration 0002__seq {
  table_alter users {
    add seq int [null, autoincrement]
  }
}
