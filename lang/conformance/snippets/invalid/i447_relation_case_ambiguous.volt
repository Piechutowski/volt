// spec: §selects-over-groups, §relationships-ref — relation names are read as SQLite reads an alias, without regard to case
// want: without regard to case
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
  hour int
}

Table points {
  id        int   [pk, increment]
  metric_id int
  ts        int
  value     float64
}

Ref: points.metric_id > metrics.id [as: Points]

Select rows (id, Points.name) for points join Points
