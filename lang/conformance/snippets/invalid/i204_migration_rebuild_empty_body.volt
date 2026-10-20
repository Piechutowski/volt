// spec: §migrations, the body of a rebuild table declares at least one column
// want: rebuild table "users": the body must declare at least one column
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0002__empty [irreversible] {
  rebuild table users {
  }
}
