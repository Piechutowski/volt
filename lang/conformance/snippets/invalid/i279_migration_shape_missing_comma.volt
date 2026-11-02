// spec: §line-structure-and-whitespace, §migrations, a syntax error drops the column from a create table body, which is then not reported empty
// want: expected ',' or ']' in settings list, found identifier "increment"
package db

table users {
  id int [pk, increment]
}

migration 0001__init {
  create table users {
    id int [pk increment]
  }
}
