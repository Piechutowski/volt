// spec: §selects-over-groups — with takes a reverse relation
// want: with takes a reverse relation
package db

table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  ts        int
}

select rows for points with metric
