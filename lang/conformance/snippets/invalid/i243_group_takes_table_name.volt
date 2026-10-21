// spec: §groups, tables and groups share one namespace, so a group may not take a table's name
// want: takes the name of a table
package db

Table users {
  id int [pk]
}

Table posts {
  id int [pk]
}

Group users {
  posts
}
