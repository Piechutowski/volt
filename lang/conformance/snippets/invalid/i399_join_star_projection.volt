// spec: §selects-over-groups — a star projection cannot reach a joined relation
// want: lists the columns it projects
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

Select rows (* \ value) for points join metric
