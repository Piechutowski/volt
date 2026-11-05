// spec: §selects-over-groups — as: and reverse: take a name
// want: "as" takes a name
package db

table metrics {
  id   int    [pk]
  name string
  unit string [null]
}

table points {
  id        int   [pk]
  metric_id int   [ref: metrics.id]
  source_id int   [null]
  ts        int
  value     float64
}

ref: points.source_id > metrics.id [as: "source"]
