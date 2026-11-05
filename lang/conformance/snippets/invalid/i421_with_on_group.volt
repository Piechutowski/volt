// spec: §selects-over-groups — with needs a table target
// want: with needs a table
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

group all {
  metrics
  points
}

select rows for all with points
