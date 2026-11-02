// spec: §migrations, a flat body with an exec without down: has no inverse
// want: exec carries no down:
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  exec `INSERT INTO users (id) VALUES (1)`
}
