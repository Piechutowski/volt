// spec: §selects-over-groups — the children's field and a parent field cannot collide
// want: collide
package db

table metrics {
  id   int    [pk]
  name string
  unit   string [null]
  points int
}

table points {
  id        int   [pk]
  metric_id int   [ref: metrics.id]
  ts        int
}

select rows for metrics with points
