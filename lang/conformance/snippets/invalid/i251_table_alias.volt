// spec: §table, a table has one name: there is no alias
// want: expected '{' in table
table users as U {
  id int [pk]
}
