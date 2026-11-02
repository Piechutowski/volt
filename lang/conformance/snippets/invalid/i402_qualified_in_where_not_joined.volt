// spec: §selects-over-groups — a qualified column in where names a joined relation
// want: names a column of a relation, and the select joins none
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

Select rows (id) for points where metric.name = :name
