// spec/migrations, delete: and update: name the action of the column's ref:
// want: names the action of the column's ref:, and the column declares none
package db

table posts {
  id      int [pk]
  user_id int [null]
}

migration 0001__init {
  table_create posts {
    id      int [pk]
    user_id int [null, delete: cascade]
  }
}
