// spec: §selects-over-groups — a qualified column names a joined relation
// want: is no joined relation
package db

table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  source_id int   [null, ref: metrics.id]
  ts        int
  value     float64
}

select rows (id, source.name) for points join metric
