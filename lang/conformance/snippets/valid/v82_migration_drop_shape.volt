// spec: §migrations, a dropped column's shape is the inverse add and satisfies the add rules, so a flat body dropping it is reversible
package db

table users {
  id int [pk, increment]
}

migration 0001__init {
  create table users {
    id    int [pk, increment]
    hits  int [default: 0]
    owner int [null, ref: users.id]
  }
}

migration 0002__users_trim {
  alter table users {
    drop hits int [default: 0]
    drop owner int [null, ref: users.id]
  }
}
