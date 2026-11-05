// spec: §selects-over-groups, §relationships-ref — relation names are read as SQLite reads an alias, without regard to case
// want: without regard to case
package db

table metrics {
  id   int    [pk]
  name string
  unit string [null]
  hour int
}

table points {
  id        int   [pk]
  metric_id int
  ts        int
  value     float64
}

ref: points.metric_id > metrics.id [as: Points]

select rows (id, Points.name) for points join Points
