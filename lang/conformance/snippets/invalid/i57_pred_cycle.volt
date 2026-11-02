// spec: §predicates — predicate references must be acyclic
package db

table ms_revenue {
  id int [pk]
  year int
}

pred a { b and year = :y }
pred b { a }

select rows for ms_revenue where a
