// spec: §migrations, inside an up block a dropped column's shape is a column and not an inverse add, so the add rules do not bind it
package db

Table users {
  id int [pk, increment]
}

Migration 0001__init {
  create table users {
    id   int [pk, increment]
    hits int [not null]
  }
}

Migration 0002__users_trim {
  up {
    alter table users {
      drop hits int [not null]
    }
  }
  down {
    rebuild table users {
      id   int [pk, increment]
      hits int [not null, default: 0]
    }
  }
}
