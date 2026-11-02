// spec: §migrations, a rowid alias introduced by a rebuild takes NULL as its source and the database assigns it
package db

table events {
  id   int    [pk]
  name string
}

migration 0001__init {
  create table events {
    name string
  }
}

migration 0002__events_id [irreversible] {
  rebuild table events {
    id   int    [pk]
    name string
  }
}
