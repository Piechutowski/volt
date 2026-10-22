// spec: §migrations, §settings-lists, a setting name is one identifier: no transaction in two words is a syntax error, no_transaction is the setting
// want: expected ',' or ']' in settings list, found identifier "transaction"
package db

Table users {
  id int [pk]
}

Migration 0001__init [no transaction] {
  create table users {
    id int [pk]
  }
}
