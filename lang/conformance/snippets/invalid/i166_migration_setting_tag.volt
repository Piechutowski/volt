// spec: §migrations, tag: describes Go and is not a migration setting
// want: tag: describes the Go struct field
package db

Table users {
  id  int [pk]
  bio string [null]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0002__bio {
  alter table users {
    add bio string [null, tag: "json:\"bio\""]
  }
}
