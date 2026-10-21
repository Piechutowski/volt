// spec: §predicates — predicate references must be acyclic
package db

Table ms_revenue {
  id int [pk]
  year int [not null]
}

Pred a { b and year = :y }
Pred b { a }

Select rows for ms_revenue where a
