// spec: §selects-over-groups — a reverse name two relationships share is ambiguous until reverse: names one
// want: is ambiguous
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

Select rows (id, points.ts) for metrics join points
