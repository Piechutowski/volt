// spec: §selects-over-groups — a relation is joined at most once
// want: is joined twice
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

select rows (id, metric.name) for points join metric join metric
