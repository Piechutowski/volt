// spec: §migrations, a column added with ref: has no default but null
// want: a column with ref: takes no default but null
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [null, ref: > users.id]
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
    add user_id int [null, default: 1, ref: > users.id]
  }
}
