// spec: §selects-over-groups — with nests the rows of a reverse relation under each parent, with or without a projection that carries the key
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

Select board  for metrics with points where unit = :unit [order: (name asc)]
Select brief  (id, name) for metrics with points
Select public (* \ unit) for metrics with points
