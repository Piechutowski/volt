// spec: §selects-over-groups — two projected columns cannot share a Go field
// want: both map to the Go field
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
  metric_name string
}

select rows (metric_name, metric.name) for points join metric
