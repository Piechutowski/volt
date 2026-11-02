// spec: §selects-over-groups, the select name Column is the column handle alias's and is rejected outright
// want: collides with the generated CRUD surface
package db

table users {
  id int [pk]
  email string
}

select column for users
