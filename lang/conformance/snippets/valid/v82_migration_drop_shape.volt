// spec: §migrations, a dropped column's shape is the inverse add and satisfies the add rules, so a flat body dropping it is reversible
package db

Table users {
  id int [pk, increment]
}

Migration 0001__init {
  create table users {
    id    int [pk, increment]
    hits  int [not null, default: 0]
    owner int [ref: > users.id]
  }
}

Migration 0002__users_trim {
  alter table users {
    drop hits int [not null, default: 0]
    drop owner int [default: null, ref: > users.id]
  }
}
