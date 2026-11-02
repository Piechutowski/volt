// spec: §selects-over-groups — a reverse name two relationships share is ambiguous until reverse: names one
// want: is ambiguous
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

select rows (id, points.ts) for metrics join points
