// spec: §selects-over-groups — the projection carries the parent key
// want: which the projection must carry
package db

table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  ts        int
}

select rows (name) for metrics with points
