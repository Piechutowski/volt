// spec: §selects-over-groups, the select name Column is the column handle alias's and is rejected outright
// want: collides with the generated CRUD surface
package db

Table users {
  id int [pk]
  email string
}

Select column for users
