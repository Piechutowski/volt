// spec: §selects-over-groups — with takes a reverse relation
// want: with takes a reverse relation
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

Select rows for points with metric
