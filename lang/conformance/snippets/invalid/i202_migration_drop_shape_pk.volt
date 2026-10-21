// spec: §migrations, the shape of a dropped column is the inverse add: a primary key cannot be added back to an existing table
// want: whose shape is the inverse add: pk cannot be added to an existing table
package db

Table users {
  email string [pk]
}

Migration 0001__init {
  create table users {
    id    int    [pk]
    email string
  }
}

Migration 0002__no_id {
  alter table users {
    drop id int [pk]
  }
}
