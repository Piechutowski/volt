// spec: §selects-over-groups — the children's field and a parent field cannot collide
// want: collide
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit   string [null]
  points int
}

Table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  ts        int
}

Select rows for metrics with points
