// spec: §V11.4 — same column name, different types: the predicate cannot be checked for all
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
}

Table ks_seats {
  id integer [pk]
  org integer [not null]
}

Group mixed {
  ms_revenue
  ks_seats
}

Select rows for mixed where org = :org
