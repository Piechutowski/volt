// spec: §selects-over-groups — the data package a query route binds to
package db

table users {
  id    int [pk]
  email string    [unique]
  name  string    [default: ""]
}

select picked for users where id in :ids [order: (id asc)]
select named  for users where name = :name
