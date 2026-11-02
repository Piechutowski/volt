// spec: §selects-over-groups — with nests under rows, not aggregates
// want: an aggregate yields none to nest under
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

select rows (count() as n) for metrics with points
