// spec: §migrations, a column with ref: must carry null when added, since the rows already there reference nothing
// want: must carry null when added
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [ref: > users.id]
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
    add user_id int [default: 0, ref: > users.id]
  }
}
