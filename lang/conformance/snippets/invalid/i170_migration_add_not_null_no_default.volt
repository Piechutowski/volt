// spec: §migrations, a not null column added to a table carries a default for the rows already there
// want: a not null column needs a default:
package db

Table users {
  id   int    [pk]
  name string [not null]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0002__name {
  alter table users {
    add name string [not null]
  }
}
