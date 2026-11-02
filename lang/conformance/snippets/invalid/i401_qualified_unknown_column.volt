// spec: §selects-over-groups — a qualified column exists in the relation's table
// want: has no column "colour"
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

Table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  source_id int   [null, ref: metrics.id]
  ts        int
  value     float64
}

Select rows (id, metric.colour) for points join metric
