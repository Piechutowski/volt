// spec: §migrations, a flat body with a rebuild is accepted when the migration is marked irreversible
package db

table users {
  id   int [pk]
  hits int [default: 0]
}

migration 0001__create_users {
  create table users {
    id   int    [pk]
    hits string [default: "0"]
  }
}

migration 0002__int_hits [irreversible] {
  rebuild table users {
    id   int [pk]
    hits int [default: 0, from: `CAST(hits AS INTEGER)`]
  }
}
