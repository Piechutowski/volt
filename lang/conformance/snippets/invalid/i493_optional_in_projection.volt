// spec: §predicates, an optional parameter stands in a predicate term only, never in a projection
// want: an optional parameter stands in a predicate term only
package db

table points {
  id int [pk]
  ts int
}

select bucketed (id, ts / :width? as bucket) for points
