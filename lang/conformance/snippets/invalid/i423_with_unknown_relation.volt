// spec: §selects-over-groups — with names a relation of the table
// want: has no relation "owners" to nest
package db

table metrics {
  id   int    [pk]
  name string
  unit string [null]
}

table points {
  id        int   [pk]
  metric_id int   [ref: metrics.id]
  ts        int
}

select rows for metrics with owners
