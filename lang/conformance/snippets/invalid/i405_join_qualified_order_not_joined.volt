// spec: §selects-over-groups — an order: column of a relation names a joined one
// want: is no joined relation
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

Select rows (id, metric.name) for points join metric [order: (source.name asc)]
