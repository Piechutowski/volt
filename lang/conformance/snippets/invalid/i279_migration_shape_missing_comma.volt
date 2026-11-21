// spec/line-structure-and-whitespace, spec/migrations, a syntax error drops the column from a table_create body, which is then not reported empty
// want: expected ',' or ']' in settings list, found identifier "autoincrement"
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id int [pk autoincrement]
  }
}
