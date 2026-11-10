// spec: §selects-over-groups, the select name CreateMany is the bulk insert's and is rejected outright
// want: collides with the generated CRUD surface
package db

table users {
  id int [pk]
  email string
}

select createMany for users
