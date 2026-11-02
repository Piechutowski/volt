// spec: §migrations, tag: describes Go and is not a migration setting
// want: tag: describes the Go struct field
package db

table users {
  id  int [pk]
  bio string [null]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
}

migration 0002__bio {
  alter table users {
    add bio string [null, tag: "json:\"bio\""]
  }
}
