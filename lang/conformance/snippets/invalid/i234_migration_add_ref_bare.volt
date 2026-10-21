// spec: §migrations, an added column with ref: must carry null, and that is the one error a bare foreign key gets
// want: must carry null when added
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
    add user_id int [ref: > users.id]
  }
}
