// spec: §selects-over-groups — join names a relation of the for table
// want: has no relation "owner" to join
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

Select rows (id, owner.name) for points join owner
