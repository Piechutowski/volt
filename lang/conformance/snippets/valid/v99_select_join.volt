// spec: §selects-over-groups, §relationships-ref — a select joins a relation of its table, inner for a required key, left for a nullable one or a reverse relation, its columns reached as relation.column
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

Table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  source_id int   [null]
  ts        int
  value     float64
}

Ref: points.source_id > metrics.id [as: source, reverse: sourced]

Select recent (id, ts, value, metric.name, metric.unit) for points join metric where ts >= :since [order: (ts desc)]
Select traced (id, metric.name, source.name) for points join metric join source where source.unit is null
Select series (id, name, points.ts, points.value) for metrics join points where points.value > 0 [order: (id asc, points.ts asc)]
Select echoes (id, sourced.ts) for metrics join sourced
