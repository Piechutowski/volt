// spec: §migrations, the default of an added column is a literal, since ADD COLUMN takes no expression
// want: the default must be a literal
package db

Table users {
  id      int       [pk]
  created time.Time [not null]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0002__created {
  alter table users {
    add created time.Time [not null, default: `CURRENT_TIMESTAMP`]
  }
}
