// spec: §selects-over-groups — with names a relation of the table
// want: has no relation "owners" to nest
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

Table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  ts        int
}

Select rows for metrics with owners
