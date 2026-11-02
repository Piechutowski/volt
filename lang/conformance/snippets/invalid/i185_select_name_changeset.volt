// spec: §selects-over-groups, the select name Changeset is the changeset's own and is rejected outright
// want: collides with the generated CRUD surface
package db

table users {
  id int [pk]
  email string
}

select changeset for users
