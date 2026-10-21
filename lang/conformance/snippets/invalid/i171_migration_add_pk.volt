// spec: §migrations, a primary key cannot be added to an existing table
// want: pk cannot be added to an existing table
package db

Table users {
  id    int    [pk]
  email string
}

Migration 0001__init {
  create table users {
    email string
  }
}

Migration 0002__id {
  alter table users {
    add id int [pk]
  }
}
