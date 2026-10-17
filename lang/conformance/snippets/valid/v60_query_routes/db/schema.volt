// spec: §selects-over-groups — the data package a query route binds to
package db

Table users {
  id    integer [pk, increment]
  email text    [not null, unique]
  name  text    [not null, default: '']
}

Select picked for users where id in :ids [order: (id asc)]
Select named  for users where name = :name
