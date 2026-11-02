// spec: §selects-over-groups — same column name, different types: the predicate cannot be checked for all
package db

table ms_revenue {
  id int [pk]
  org string
}

table ks_seats {
  id int [pk]
  org int
}

group mixed {
  ms_revenue
  ks_seats
}

select rows for mixed where org = :org
