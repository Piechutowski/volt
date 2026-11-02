// spec: §migrations, a column added without null is NOT NULL and carries a default for the rows already there
// want: a column without null is NOT NULL and needs a default:
package db

table users {
  id   int    [pk]
  name string
}

migration 0001__init {
  create table users {
    id int [pk]
  }
}

migration 0002__name {
  alter table users {
    add name string
  }
}
