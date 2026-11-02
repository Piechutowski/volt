// spec: §selects-over-groups — join names a relation of the for table
// want: has no relation "owner" to join
package db

table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
}

table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  source_id int   [null, ref: metrics.id]
  ts        int
  value     float64
}

select rows (id, owner.name) for points join owner
