// spec: §groups, tables and groups share one namespace, so a group may not take a table's name
// want: takes the name of a table
package db

table users {
  id int [pk]
}

table posts {
  id int [pk]
}

group users {
  posts
}
