// spec: §selects-over-groups, the select name Changeset is the changeset's own and is rejected outright
// want: collides with the generated CRUD surface
package db

Table users {
  id int [pk]
  email string
}

Select changeset for users
