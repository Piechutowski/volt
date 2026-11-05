// spec: §selects-over-groups — with nests the rows of a reverse relation under each parent, with or without a projection that carries the key
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

select board  for metrics with points where unit = :unit [order: (name asc)]
select brief  (id, name) for metrics with points
select public (* \ unit) for metrics with points
