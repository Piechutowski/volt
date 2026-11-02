// spec: §migrations, a rebuilt NOT NULL column needs a source: a from:, an old column of its name, or a default:
// want: its source is NULL
package db

table users {
  id    int    [pk]
  email string
}

migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]
  }
}

migration 0002__email [irreversible] {
  rebuild table users {
    id    int    [pk]
    email string
  }
}
