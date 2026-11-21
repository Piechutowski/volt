// spec/migrations, spec/columns, unique and index are not both set on a migration column, in a table_create body as in a schema table
// want: "unique" and "index" cannot both be set
package db

table users {
  id    int    [pk]
  email string [unique]
}

migration 0001__init {
  table_create users {
    id    int    [pk]
    email string [unique, index]
  }
}
