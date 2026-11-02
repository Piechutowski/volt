// spec: §selects-over-groups — join needs a table target, a group has no relations
// want: join needs a table
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

group all {
  metrics
  points
}

select rows (id) for all join metric
