// spec: §V10.2 — predicate references must be acyclic
package db

Table ms_revenue {
  id integer [pk]
  year integer [not null]
}

Pred a { b and year = :y }
Pred b { a }

Select rows for ms_revenue where a
