// spec: §columns, an inline ref endpoint is table.column; a parenthesized column is outside the grammar
// want: an inline ref endpoint is table.column, without parentheses
table users {
  id int [pk]
}

table posts {
  id  int [pk]
  uid int [ref: users.(id)]
}
