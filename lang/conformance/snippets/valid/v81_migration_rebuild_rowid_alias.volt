// spec/migrations, a database-assigned key introduced by a rebuild takes NULL as its source and the database assigns it
package db

table events {
  id   int    [pk]
  name string
}

migration 0001__init {
  table_create events {
    name string
  }
}

migration 0002__events_id [irreversible] {
  table_rebuild events {
    id   int    [pk]
    name string
  }
}
