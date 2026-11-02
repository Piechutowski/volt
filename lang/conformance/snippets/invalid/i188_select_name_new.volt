// spec: §selects-over-groups, the select name New is the changeset constructor's and is rejected outright
// want: collides with the generated CRUD surface
package db

table users {
  id int [pk]
  email string
}

select new for users
