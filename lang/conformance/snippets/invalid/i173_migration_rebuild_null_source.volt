// spec: §migrations, a rebuilt not null column needs a source: a from:, an old column of its name, or a default:
// want: its source is NULL
package db

Table users {
  id    int    [pk]
  email string [not null]
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string
  }
}

Migration 0002__email [irreversible] {
  rebuild table users {
    id    int    [pk]
    email string [not null]
  }
}
