// spec: §selects-over-groups — with needs a table target
// want: with needs a table
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

Group all {
  metrics
  points
}

Select rows for all with points
