// spec/migrations, a dropped column's shape is the inverse add and satisfies the add rules, so a flat body dropping it is reversible
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id    int [pk]
    hits  int [default: 0]
    owner int [null, ref: users.id]
  }
}

migration 0002__users_trim {
  table_alter users {
    drop hits int [default: 0]
    drop owner int [null, ref: users.id]
  }
}
