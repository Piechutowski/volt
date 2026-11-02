// spec: §selects-over-groups, §relationships-ref — a select joins a relation of its table, inner for a required key, left for a nullable one or a reverse relation, its columns reached as relation.column
package db

table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  source_id int   [null]
  ts        int
  value     float64
}

ref: points.source_id > metrics.id [as: source, reverse: sourced]

select recent (id, ts, value, metric.name, metric.unit) for points join metric where ts >= :since [order: (ts desc)]
select traced (id, metric.name, source.name) for points join metric join source where source.unit is null
select series (id, name, points.ts, points.value) for metrics join points where points.value > 0 [order: (id asc, points.ts asc)]
select echoes (id, sourced.ts) for metrics join sourced
