// spec/migrations, every migration name of a package has one digit-run width, so that byte order and numeric order agree
// want: differ in digit width
package db

table users {
  id  int [pk]
  bio string [null]
}

migration 0001__init {
  table_create users {
    id int [pk]
  }
}

migration 00002__bio {
  table_alter users {
    add bio string [null]
  }
}
