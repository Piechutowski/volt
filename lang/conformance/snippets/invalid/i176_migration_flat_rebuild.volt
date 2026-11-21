// spec/migrations, a flat body with a table_rebuild has no inverse
// want: table_rebuild users forgets the old shape
package db

table users {
  id   int [pk]
  hits int [default: 0]
}

migration 0001__init {
  table_create users {
    id   int    [pk]
    hits string [default: "0"]
  }
}

migration 0002__int_hits {
  table_rebuild users {
    id   int [pk]
    hits int [default: 0, from: `CAST(hits AS INTEGER)`]
  }
}
