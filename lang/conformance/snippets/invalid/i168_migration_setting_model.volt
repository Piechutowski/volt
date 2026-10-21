// spec: §migrations, model: describes Go and is not a migration setting
// want: model: names the Go model
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk, model: 'Account']
  }
}
