// spec: §selects-over-groups — the data package a query route binds to
package db

Table users {
  id    int [pk, increment]
  email string    [not null, unique]
  name  string    [not null, default: '']
}

Select picked for users where id in :ids [order: (id asc)]
Select named  for users where name = :name
