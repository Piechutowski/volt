// spec: §migrations, inside an up block a dropped column's shape is a column and not an inverse add, so the add rules do not bind it
package db

table users {
  id int [pk, increment]
}

migration 0001__init {
  create table users {
    id   int [pk, increment]
    hits int
  }
}

migration 0002__users_trim {
  up {
    alter table users {
      drop hits int
    }
  }
  down {
    rebuild table users {
      id   int [pk, increment]
      hits int [default: 0]
    }
  }
}
