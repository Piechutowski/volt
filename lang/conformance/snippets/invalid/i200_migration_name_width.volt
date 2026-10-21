// spec: §migrations, every migration name of a package has one digit-run width, so that byte order and numeric order agree
// want: differ in digit width
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

Migration 00002__bio {
  alter table users {
    add bio string [null]
  }
}
