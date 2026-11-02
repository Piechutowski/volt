package db

migration 0001__init {
  create table users {
    id int [pk, increment]
  }
}
