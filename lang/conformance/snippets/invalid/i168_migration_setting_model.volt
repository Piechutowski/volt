// spec: §migrations, model: describes Go and is not a migration setting
// want: model: names the Go model
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk, model: "Account"]
  }
}
