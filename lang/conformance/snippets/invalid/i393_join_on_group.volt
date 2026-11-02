// spec: §selects-over-groups — join needs a table target, a group has no relations
// want: join needs a table
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

Group all {
  metrics
  points
}

Select rows (id) for all join metric
