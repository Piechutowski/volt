// spec: §migrations, a flat body with a drop column without a type has no inverse
// want: carries no type to add the column back with
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id  int [pk]
    bio string [null]
  }
}

migration 0002__no_bio {
  alter table users {
    drop bio
  }
}
