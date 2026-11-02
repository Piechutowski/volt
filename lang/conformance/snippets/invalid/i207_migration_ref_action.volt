// spec: §migrations, delete: and update: on a migration column take the referential actions of §relationships-ref
// want: invalid referential action "explode" for 'delete'
package db

table users {
  id int [pk]
}

table posts {
  id      int [pk]
  user_id int [null, ref: users.id]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id      int [pk]
    user_id int [null, ref: users.id, delete: explode]
  }
}
