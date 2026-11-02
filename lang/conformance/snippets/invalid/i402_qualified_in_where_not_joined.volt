// spec: §selects-over-groups — a qualified column in where names a joined relation
// want: names a column of a relation, and the select joins none
package db

table metrics {
  id   int    [pk]
  name string
  unit string [null]
}

table points {
  id        int   [pk]
  metric_id int   [ref: metrics.id]
  source_id int   [null, ref: metrics.id]
  ts        int
  value     float64
}

select rows (id) for points where metric.name = :name
