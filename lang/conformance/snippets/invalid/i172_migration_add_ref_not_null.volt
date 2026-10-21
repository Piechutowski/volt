// spec: §migrations, a column with ref: is nullable when added, since the rows already there reference nothing
// want: must be nullable when added
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [not null, ref: > users.id]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id int [pk]
  }
}

Migration 0002__post_owner {
  alter table posts {
    add user_id int [not null, default: 0, ref: > users.id]
  }
}
