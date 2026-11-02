// spec: §selects-over-groups — the projection carries the parent key
// want: which the projection must carry
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

Select rows (name) for metrics with points
