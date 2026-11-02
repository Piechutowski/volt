// spec: §selects-over-groups — an order: column of a relation names a joined one
// want: is no joined relation
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

select rows (id, metric.name) for points join metric [order: (source.name asc)]
