// spec: §selects-over-groups — as: and reverse: take a name
// want: "as" takes a name
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

Ref: points.source_id > metrics.id [as: "source"]
