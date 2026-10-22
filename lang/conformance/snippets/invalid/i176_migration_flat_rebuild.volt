// spec: §migrations, a flat body with a rebuild table has no inverse
// want: rebuild table users forgets the old shape
package db

Table users {
  id   int [pk]
  hits int [default: 0]
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    hits string [default: "0"]
  }
}

Migration 0002__int_hits {
  rebuild table users {
    id   int [pk]
    hits int [default: 0, from: `CAST(hits AS INTEGER)`]
  }
}
