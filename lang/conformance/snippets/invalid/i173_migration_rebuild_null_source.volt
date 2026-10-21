// spec: §migrations, a rebuilt NOT NULL column needs a source: a from:, an old column of its name, or a default:
// want: its source is NULL
package db

Table users {
  id    int    [pk]
  email string
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]
  }
}

Migration 0002__email [irreversible] {
  rebuild table users {
    id    int    [pk]
    email string
  }
}
