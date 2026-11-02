// spec: §migrations, a primary key cannot be added to an existing table
// want: pk cannot be added to an existing table
package db

table users {
  id    int    [pk]
  email string
}

migration 0001__init {
  create table users {
    email string
  }
}

migration 0002__id {
  alter table users {
    add id int [pk]
  }
}
