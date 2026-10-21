// spec: §migrations, a rowid alias introduced by a rebuild takes NULL as its source and the database assigns it
package db

Table events {
  id   int    [pk, increment]
  name string [not null]
}

Migration 0001__init {
  create table events {
    name string [not null]
  }
}

Migration 0002__events_id [irreversible] {
  rebuild table events {
    id   int    [pk, increment]
    name string [not null]
  }
}
