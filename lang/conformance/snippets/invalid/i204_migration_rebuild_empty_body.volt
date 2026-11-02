// spec: §migrations, the body of a rebuild table declares at least one column
// want: rebuild table "users": the body must declare at least one column
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
}

migration 0002__empty [irreversible] {
  rebuild table users {
  }
}
