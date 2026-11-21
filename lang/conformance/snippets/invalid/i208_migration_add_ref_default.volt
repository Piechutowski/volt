// spec/migrations, a column added with ref: has no default
// want: a column with ref: takes no default
package db

table users {
  id int [pk]
}

table posts {
  id      int [pk]
  user_id int [null, ref: users.id]
}

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_create posts {
    id int [pk]
  }
}

migration 0002__post_owner {
  table_alter posts {
    add user_id int [null, default: 1, ref: users.id]
  }
}
