// spec: §table, a table is named by its name only: there are no schemas
// want: expected '{' in table
table core.users {
  id int [pk]
}
