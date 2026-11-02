// spec: §selects-over-groups — a qualified column exists in the relation's table
// want: has no column "colour"
package db

table metrics {
  id   int    [pk]
  name string
  unit string [null]
}

table points {
  id        int   [pk]
  metric_id int   [ref: metrics.id]
  source_id int   [null, ref: metrics.id]
  ts        int
  value     float64
}

select rows (id, metric.colour) for points join metric
