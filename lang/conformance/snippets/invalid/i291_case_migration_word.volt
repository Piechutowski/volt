// spec/keywords, spec/migrations, the words of a migration body are lowercase keywords
// want: write table_create, not TABLE_CREATE
package db

table users {
  id int [pk]
}

migration 0001__init {
  TABLE_CREATE users {
    id int [pk]
  }
}
