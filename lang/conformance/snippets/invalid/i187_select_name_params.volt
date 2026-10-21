// spec: §selects-over-groups, the select name Params is the write body alias's and is rejected outright
// want: collides with the generated CRUD surface
package db

Table users {
  id int [pk]
  email string [not null]
}

Select params for users
