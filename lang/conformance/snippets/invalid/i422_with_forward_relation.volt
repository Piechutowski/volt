// spec: §selects-over-groups — with takes a reverse relation
// want: with takes a reverse relation
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

select rows for points with metric
