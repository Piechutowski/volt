// spec: §table, a table has one name: there is no alias
// want: expected '{' in Table
Table users as U {
  id int [pk]
}
