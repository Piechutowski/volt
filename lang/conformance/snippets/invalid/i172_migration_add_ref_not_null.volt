// spec: §migrations, a column with ref: must carry null when added, since the rows already there reference nothing
// want: must carry null when added
package db

table users {
  id int [pk]
}

table posts {
  id      int [pk]
  user_id int [ref: users.id]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id int [pk]
  }
}

migration 0002__post_owner {
  alter table posts {
    add user_id int [default: 0, ref: users.id]
  }
}
