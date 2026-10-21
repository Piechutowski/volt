// spec: §selects-over-groups — same column name, different types: the predicate cannot be checked for all
package db

Table ms_revenue {
  id int [pk]
  org string [not null]
}

Table ks_seats {
  id int [pk]
  org int [not null]
}

Group mixed {
  ms_revenue
  ks_seats
}

Select rows for mixed where org = :org
