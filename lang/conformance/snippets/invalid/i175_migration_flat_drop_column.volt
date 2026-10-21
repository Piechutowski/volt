// spec: §migrations, a flat body with a drop column without a type has no inverse
// want: carries no type to add the column back with
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id  int [pk]
    bio string [null]
  }
}

Migration 0002__no_bio {
  alter table users {
    drop bio
  }
}
