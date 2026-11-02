// spec: §selects-over-groups — with nests under rows, not aggregates
// want: an aggregate yields none to nest under
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

Select rows (count() as n) for metrics with points
