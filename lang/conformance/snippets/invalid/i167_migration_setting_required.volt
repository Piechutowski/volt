// spec: §migrations, required describes Go and is not a migration setting, on the key of a shape as on any column, with no second diagnostic about the database-assigned key
// want: required describes Go-side validation
package db

table users {
  id    int    [pk]
  email string [required]
}

migration 0001__init {
  create table users {
    id    int    [pk, required]
    email string [required]
  }
}
