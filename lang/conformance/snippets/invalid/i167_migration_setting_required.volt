// spec: §migrations, required describes Go and is not a migration setting
// want: required describes Go-side validation
package db

table users {
  id    int    [pk]
  email string [required]
}

migration 0001__init {
  create table users {
    id    int    [pk]
    email string [required]
  }
}
