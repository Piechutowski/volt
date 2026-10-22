// spec: §keywords, §migrations, the words of a migration body are lowercase keywords
// want: write create, not CREATE
package db

Table users {
  id int [pk, increment]
}

Migration 0001__init {
  CREATE TABLE users {
    id int [pk, increment]
  }
}
