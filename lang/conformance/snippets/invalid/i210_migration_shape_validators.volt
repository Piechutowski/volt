// spec: §migrations, a validators block is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

Table users {
  id    int    [pk]
  email string
}

Migration 0001__init {
  create table users {
    id    int    [pk]
    email string

    Validators {
      EmailShape(email)
    }
  }
}
