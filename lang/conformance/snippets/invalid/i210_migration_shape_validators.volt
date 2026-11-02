// spec: §migrations, a validators block is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

table users {
  id    int    [pk]
  email string
}

migration 0001__init {
  create table users {
    id    int    [pk]
    email string

    validators {
      EmailShape(email)
    }
  }
}
